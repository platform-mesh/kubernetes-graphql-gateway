package gateway_test

type RbacAuthorizationK8sIo struct {
	ClusterRole *ClusterRole `json:"ClusterRole,omitempty"`
}

type ClusterRole struct {
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
