package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"log"
	kube "github.com/tembleking/sysdig_scheduler/kubernetes"
	"github.com/tembleking/sysdig_scheduler/sysdig"
	"bytes"
	"net/http"
	"flag"
	"os/user"
)

var schedulerName string 
var kubeApi kube.KubernetesCoreV1Api
var sysdigApi sysdig.SysdigApiClient
var metrics []map[string]interface{}
var sysdigMetric string

type event struct {
	Type string `json:"type"`
	Object struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
		Spec struct {
			SchedulerName string `json:"schedulerName"`
		} `json:"spec"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"object"`
}

func getRequestTime(hostname string) (requestTime float64, err error) {
	hostFilter := fmt.Sprintf(`host.hostName = '%s'`, hostname)
	start := -60
	end := 0
	sampling := 60

	metricDataResponse, err := sysdigApi.GetData(metrics, start, end, sampling, hostFilter, "host")
	if err != nil {
		return
	} else if metricDataResponse.StatusCode != 200 {
		err = fmt.Errorf("Metric data response: %s\n", metricDataResponse.Status)
		return
	}
	defer metricDataResponse.Body.Close()

	var metricData struct {
		Data []struct {
			D []float64 `json:"d"`
		} `json:"data"`
	}

	err = json.NewDecoder(metricDataResponse.Body).Decode(&metricData)
	if err != nil {
		return
	}

	if len(metricData.Data) > 0 && len(metricData.Data[0].D) > 0 {
		requestTime = metricData.Data[0].D[0]
	} else {
		err = fmt.Errorf("no data found with those parameters")
	}

	return
}

func getBestRequestTime(nodes []string) (bestNodeFound string, lessNodeTime float64, err error) {
	if len(nodes) == 0 {
		err = fmt.Errorf("node list must contain at least one element")
		return
	}

	type nodeStats struct {
		name string
		time float64
		err  error
	}

	// We will make all the request asynchronous for performance reasons
	wg := sync.WaitGroup{}
	nodeStatsChannel := make(chan nodeStats, len(nodes))
	nodeStatsErrorsChannel := make(chan nodeStats, len(nodes))

	// Launch all requests asynchronously
	for _, node := range nodes {
		wg.Add(1)

		go func(nodeName string) {
			defer wg.Done()

			requestTime, err := getRequestTime(nodeName)
			if err == nil { // No error found, we will send the struct
				nodeStatsChannel <- nodeStats{name: nodeName, time: requestTime}
			} else {
				nodeStatsErrorsChannel <- nodeStats{name: nodeName, err: err}
			}
		}(node)
	}

	wg.Wait()
	close(nodeStatsChannel)
	close(nodeStatsErrorsChannel)

	lessNodeTime = -1
	for node := range nodeStatsChannel {

		if lessNodeTime == -1 { // First iteration -> Time not set
			lessNodeTime = node.time
			bestNodeFound = node.name
		} else if node.time < lessNodeTime { // Found a best node time?
			lessNodeTime = node.time
			bestNodeFound = node.name
		}
	}
	errorHappenedString := `Error retrieving node "%s": "%s"`
	for node := range nodeStatsErrorsChannel {
		log.Printf(errorHappenedString+"\n", node.name, node.err.Error())
	}

	if bestNodeFound == "" {
		err = fmt.Errorf("not a single node could be found")
	}

	return
}

func nodesAvailable() (readyNodes []string) {
	nodes, err := kubeApi.ListNodes()
	if err != nil {
		log.Println(err)
	}
	for _, node := range nodes {
		for _, status := range node.Status.Conditions {
			if status.Status == "True" && status.Type == "Ready" {
				readyNodes = append(readyNodes, node.Metadata.Name)
			}
		}
	}
	return
}

func scheduler(podName, nodeName, namespace string) (response *http.Response, err error) {
	if namespace == "" {
		namespace = "default"
	}

	body := map[string]interface{}{
		"target": map[string]string{
			"kind":       "Node",
			"apiVersion": "v1",
			"name":       nodeName,
		},
		"metadata": map[string]string{
			"name": podName,
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return
	}

	return kubeApi.CreateNamespacedBinding(namespace, bytes.NewReader(data))
}

func usage() {
	fmt.Printf("Usage: %s [-s SCHEDULER_NAME] [-m SYSDIG_METRIC] [-t SYSDIG_TOKEN] [-k KUBERNETES_CONFIG_FILE]", os.Args[0])
	fmt.Println(`
If the env KUBECONFIG is not set, the -k option must be provided.
If the env SDC_TOKEN is not set, the -t option must be provided.
If the env SDC_METRIC is not set, the -m option must be provided.
If the env SDC_SCHEDULER is not set, the -s option must be provided.
`)
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	usr, _ := user.Current()

	sysdigTokenFlag := flag.String("t", "", "Sysdig Cloud Token")
	kubeConfigFileFlag := flag.String("k", "", "Kubernetes config file")
	sysdigMetricFlag := flag.String("m", "", "Sysdig metric to monitorize")
	schedulerNameFlag := flag.String("s", "", "Scheduler name")

	flag.Usage = usage
	flag.Parse()

	if sysdigTokenEnv, tokenSetByEnv := os.LookupEnv("SDC_TOKEN"); !tokenSetByEnv && *sysdigTokenFlag == "" {
		fmt.Println("Error: Sysdig Cloud token is not set.\n")
		usage()
	} else {
		if tokenSetByEnv {
			sysdigApi.SetToken(sysdigTokenEnv)
		}
		if *sysdigTokenFlag != "" { // If the flag is set, overrides the environment
			sysdigApi.SetToken(*sysdigTokenFlag)
		}
	}

	if _, kubeTokenSetByEnv := os.LookupEnv("KUBECONFIG"); !kubeTokenSetByEnv && *kubeConfigFileFlag == "" {
		os.Setenv("KUBECONFIG", usr.HomeDir+"/.kube/config")
	} else {
		if *kubeConfigFileFlag != "" {
			os.Setenv("KUBECONFIG", *kubeConfigFileFlag)
		}
	}
	kubeApi.LoadKubeConfig()

	if sysdigMetricEnv, sysdigMetricEnvIsSet := os.LookupEnv("SDC_METRIC"); !sysdigMetricEnvIsSet && *sysdigMetricFlag == "" {
		fmt.Println("The Sysdig metric must be defined\n")
		usage()
	} else {
		if sysdigMetricEnvIsSet {
			sysdigMetric = sysdigMetricEnv
		}
		if *sysdigMetricFlag != "" {
			sysdigMetric = *sysdigMetricFlag
		}
	}

	if schedulerNameEnv, schedulernameEnvIsSet := os.LookupEnv("SDC_SCHEDULER"); !schedulernameEnvIsSet && *schedulerNameFlag == "" {
		fmt.Println("Scheduler name must be set\n")
		usage()
	} else {
		if schedulernameEnvIsSet {
			schedulerName = schedulerNameEnv
		}
		if *schedulerNameFlag != "" {
			schedulerName = *schedulerNameFlag
		}
	}
}

func main() {
	event := event{}

	metrics = append(metrics, map[string]interface{}{
		"id": sysdigMetric,
		"aggregations": map[string]string{
			"time": "timeAvg", "group": "avg",
		},
	})

	ch, _ := kubeApi.Watch("GET", "api/v1/namespaces/default/pods", nil, nil)
	for data := range ch {
		err := json.Unmarshal(data, &event)
		if err != nil {
			log.Println("Error:", err)
			continue
		}

		if event.Object.Status.Phase == "Pending" && event.Object.Spec.SchedulerName == schedulerName && event.Type == "ADDED" {
			fmt.Println("Scheduling", event.Object.Metadata.Name)

			bestNodeFound, lessNodeTime, err := getBestRequestTime(nodesAvailable())
			if err != nil {
				log.Println("Error:", err)
			} else {
				fmt.Println("Best node found: ", bestNodeFound, lessNodeTime)
				response, err := scheduler(event.Object.Metadata.Name, bestNodeFound, "")
				if err != nil {
					log.Println("Error:", err)
				}
				response.Body.Close()
			}
		}
	}
}
