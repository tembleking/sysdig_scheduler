package kubernetes

import (
	"net/http"
	"io"
	"os"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"os/user"
	"crypto/tls"
	"encoding/base64"
	"crypto/x509"
	"encoding/json"
	"net/url"
	"bufio"
	"fmt"
	"log"
)

type KubernetesCoreV1Api struct {
	config KubeConfig
}

func getKubeConfigFileDefaultLocation() string {
	kubeConf, isSet := os.LookupEnv("KUBECONFIG")
	if isSet && kubeConf != "" {
		return kubeConf
	} else {
		usr, err := user.Current()
		if err != nil {
			log.Panic(err)
		}
		return usr.HomeDir + "/.kube/config"
	}
}

func (api KubernetesCoreV1Api) CreateNamespacedBinding(namespace string, body io.Reader) (response *http.Response, err error) {
	return api.Request("POST", fmt.Sprintf("api/v1/namespaces/%s/bindings", namespace), nil, body)
}

func (api KubernetesCoreV1Api) Watch(httpMethod, apiMethod string, values url.Values, body io.Reader) (responseChannel chan []byte, err error) {
	if values == nil {
		values = url.Values{}
	}
	values.Add("watch", "true")
	responseChannel = make(chan []byte)
	go func() {
		response, err := api.Request(httpMethod, apiMethod, values, body)
		if err != nil {
			return
		}
		defer response.Body.Close()

		reader := bufio.NewReader(response.Body)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				log.Println(err)
				continue
			}
			responseChannel <- line
		}
		close(responseChannel)
	}()
	return
}

func (api KubernetesCoreV1Api) Request(httpMethod, apiMethod string, values url.Values, body io.Reader) (response *http.Response, err error) {
	apiUrl := api.getCurrentApiUrlEndpoint()

	certificate, caCertPool := api.getCurrentTLSInfo()
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      caCertPool,
	}

	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := http.Client{Transport: transport}
	request, err := http.NewRequest(httpMethod, apiUrl+"/"+apiMethod, body)
	if err != nil {
		log.Panic(err)
	}
	if values != nil {
		request.URL.RawQuery = values.Encode()
	}

	// Get the info in json
	request.Header.Add("Content-Type", "application/json")

	// Make the request
	response, err = client.Do(request)
	if err != nil {
		log.Panic(err)
	}
	return
}

func (api KubernetesCoreV1Api) ListNodes() (nodes []KubeNode, err error) {
	response, err := api.Request("GET", "api/v1/nodes", nil, nil)
	if err != nil {
		return
	}
	defer response.Body.Close()

	var nodeInfo struct {
		Items []KubeNode `json:"items"`
	}
	err = json.NewDecoder(response.Body).Decode(&nodeInfo)
	if err != nil {
		return
	}
	nodes = nodeInfo.Items
	return
}

func (api KubernetesCoreV1Api) getCurrentApiUrlEndpoint() string {
	for _, context := range api.config.Contexts {
		if context.Name == api.config.CurrentContext {
			for _, cluster := range api.config.Clusters {
				if cluster.Name == context.Data.Cluster {
					return cluster.Data.Server
				}
			}
		}
	}
	log.Panic("Current API Url endpoint couldn't be determined, checkout if the configuration is correct")
	return ""
}

func (api KubernetesCoreV1Api) getCurrentTLSInfo() (clientCert tls.Certificate, serverCaCert *x509.CertPool) {
	var currentContextUser string
	var currentContextCluster string
	var certData []byte
	var keyData []byte
	var caCertData []byte

	// Load current context information
	for _, context := range api.config.Contexts {
		if context.Name == api.config.CurrentContext {
			currentContextUser = context.Data.User
			currentContextCluster = context.Data.Cluster
		}
	}

	// Get cert and key data from the user
	for _, user := range api.config.Users {
		if user.Name == currentContextUser {
			certData, keyData = user.Data.ClientCertificateData, user.Data.ClientKeyData
		}
	}

	// Get CA Cert data from current cluster
	for _, cluster := range api.config.Clusters {
		if cluster.Name == currentContextCluster {
			caCertData = cluster.Data.CertificateAuthorityData
		}
	}

	certificate, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		panic(err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertData)

	return certificate, caCertPool
}

// Reads the configuration file and loads the config struct
func (api *KubernetesCoreV1Api) LoadKubeConfig() (err error) {
	yamlFile, err := ioutil.ReadFile(getKubeConfigFileDefaultLocation())
	if err != nil {
		panic("Could not load the Kubernetes configuration")
	}

	kubeConfig := KubeConfig{}
	err = yaml.Unmarshal(yamlFile, &kubeConfig)
	if err != nil {
		return err
	}

	// Decode the certificate
	for k, cluster := range kubeConfig.Clusters {
		certBytes, err := base64.StdEncoding.DecodeString(cluster.Data.CertificateAuthorityDataStr)
		if err != nil {
			panic(err)
		}
		cluster.Data.CertificateAuthorityData = certBytes
		kubeConfig.Clusters[k] = cluster
	}

	// Decode the certificate and the key
	for k, user := range kubeConfig.Users {
		cert, err := base64.StdEncoding.DecodeString(user.Data.ClientCertificateDataStr)
		if err != nil {
			panic(err)
		}
		user.Data.ClientCertificateData = cert
		key, err := base64.StdEncoding.DecodeString(user.Data.ClientKeyDataStr)
		if err != nil {
			panic(err)
		}
		user.Data.ClientKeyData = key
		kubeConfig.Users[k] = user
	}

	api.config = kubeConfig
	return err
}
