package kubernetes

type KubeNode struct {
	Metadata KubeNodeMetadata `json:"metadata"`
	Spec     KubeNodeSpec     `json:"spec"`
	Status   KubeNodeStatus   `json:"status"`
}

type KubeNodeMetadata struct {
	Name              string `json:"name"`
	SelfLink          string `json:"selfLink"`
	Uid               string `json:"uid"`
	ResourceVersion   string `json:"resourceVersion"`
	CreationTimestamp string `json:"creationTimestamp"`
	// `json:"labels"`
	//`json:"annotations"`
}

type KubeNodeSpec struct {
	PodCIDR    string `json:"podCIDR"`
	ExternalID string `json:"externalID"`
}

type KubeNodeStatus struct {
	Conditions []KubeNodeStatusConditions `json:"conditions"`
}

type KubeNodeStatusConditions struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastHeartbeatTime  string `json:"lastHeartbeatTime"`
	LastTransitionTime string `json:"lastTransitionTime"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
}

