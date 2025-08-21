package gateway_test

type RbacAuthorizationK8sIO struct {
	ClusterRole        *ClusterRole        `json:"ClusterRole,omitempty"`
	ClusterRoleBinding *ClusterRoleBinding `json:"ClusterRoleBinding,omitempty"`
}

type ClusterRole struct {
	Metadata metadata `json:"metadata"`
}

type ClusterRoleBinding struct {
	Metadata metadata `json:"metadata"`
	RoleRef  roleRef  `json:"roleRef"`
}

type roleRef struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	APIGroup string `json:"apiGroup"`
	Role     crMeta `json:"role"`
}

type crMeta struct {
	Metadata metadata `json:"metadata"`
}

func CreateClusterRoleMutation() string {
	return `mutation {
			  rbac_authorization_k8s_io {
				createClusterRole(
				  object: {
					metadata: {
					  name: "test-cluster-role"
					}
				  }
				) {
				  metadata {
					name
				  }
				}
			  }
			}`
}

func GetClusterRoleQuery() string {
	return `{
			  rbac_authorization_k8s_io {
				ClusterRole(name: "test-cluster-role") {
				  metadata {
					name
				  }
				}
			  }
			}`
}

func DeleteClusterRoleMutation() string {
	return `mutation {
	  rbac_authorization_k8s_io {
		deleteClusterRole(name: "test-cluster-role") 
	  }
	}`
}
