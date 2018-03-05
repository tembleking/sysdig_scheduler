package kubernetes

type KubeConfig struct {
	ApiVersion     string    `yaml:"apiVersion"`
	Clusters       []Cluster `yaml:"clusters"`
	Contexts       []Context `yaml:"contexts"`
	CurrentContext string    `yaml:"current-context"`
	Kind           string    `yaml:"kind"`
	Users          []User    `yaml:"users"`
}

type Cluster struct {
	Data ClusterData `yaml:"cluster"`
	Name string      `yaml:"name"`
}

type ClusterData struct {
	CertificateAuthorityDataStr string `yaml:"certificate-authority-data"`
	CertificateAuthorityData    []byte
	Server                      string `yaml:"server"`
}

type Context struct {
	Data ContextData `yaml:"context"`
	Name string      `yaml:"name"`
}

type ContextData struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type User struct {
	Name string   `yaml:"name"`
	Data UserData `yaml:"user"`
}

type UserData struct {
	ClientCertificateDataStr string `yaml:"client-certificate-data"`
	ClientCertificateData    []byte
	ClientKeyDataStr         string `yaml:"client-key-data"`
	ClientKeyData            []byte
}