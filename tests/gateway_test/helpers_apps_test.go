package gateway_test

type apps struct {
	Deployment       *deployment `json:"Deployment,omitempty"`
	CreateDeployment *deployment `json:"createDeployment,omitempty"`
	DeleteDeployment *bool       `json:"deleteDeployment,omitempty"`
}

type deployment struct {
	Metadata deploymentMetadata `json:"metadata"`
	Spec     deploymentSpec     `json:"spec"`
}

type deploymentMetadata struct {
	Name        string      `json:"name"`
	Namespace   string      `json:"namespace"`
	Labels      interface{} `json:"labels,omitempty"`      // Can be map[string]interface{} for scalar approach
	Annotations interface{} `json:"annotations,omitempty"` // Can be map[string]interface{} for scalar approach
}

type deploymentSpec struct {
	Replicas int                `json:"replicas"`
	Selector deploymentSelector `json:"selector"`
	Template podTemplate        `json:"template"`
}

type deploymentSelector struct {
	MatchLabels interface{} `json:"matchLabels,omitempty"` // Can be map[string]interface{} for scalar approach
}

type podTemplate struct {
	Metadata podTemplateMetadata `json:"metadata"`
	Spec     podTemplateSpec     `json:"spec"`
}

type podTemplateMetadata struct {
	Labels interface{} `json:"labels,omitempty"` // Can be map[string]interface{} for scalar approach
}

type podTemplateSpec struct {
	NodeSelector interface{}           `json:"nodeSelector,omitempty"` // Can be map[string]interface{} for scalar approach
	Containers   []deploymentContainer `json:"containers"`
}

type deploymentContainer struct {
	Name  string           `json:"name"`
	Image string           `json:"image"`
	Ports []deploymentPort `json:"ports,omitempty"`
}

type deploymentPort struct {
	ContainerPort int `json:"containerPort"`
}
