package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// Test_relation_clusterrolebinding_role_ref mirrors pod test style: creates schema file per workspace,
// creates a ClusterRole and ClusterRoleBinding via GraphQL, then queries roleRef.role to ensure relation resolution.
func (suite *CommonTestSuite) Test_relation_clusterrolebinding_role_ref() {
	workspaceName := "relationsWorkspace"

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create ClusterRole
	statusCode, body := suite.doRawGraphQL(url, createClusterRoleForRelationMutation())
	require.Equal(suite.T(), http.StatusOK, statusCode)
	require.Nil(suite.T(), body["errors"])

	// Create ClusterRoleBinding referencing the ClusterRole
	statusCode, body = suite.doRawGraphQL(url, createClusterRoleBindingForRelationMutation())
	require.Equal(suite.T(), http.StatusOK, statusCode)
	require.Nil(suite.T(), body["errors"])

	// Query ClusterRoleBinding and expand roleRef.role
	statusCode, body = suite.doRawGraphQL(url, getClusterRoleBindingWithRoleQuery())
	require.Equal(suite.T(), http.StatusOK, statusCode)
	require.Nil(suite.T(), body["errors"])

	// Extract nested role name from generic map
	data, _ := body["data"].(map[string]interface{})
	rbac, _ := data["rbac_authorization_k8s_io"].(map[string]interface{})
	crb, _ := rbac["ClusterRoleBinding"].(map[string]interface{})
	roleRef, _ := crb["roleRef"].(map[string]interface{})
	role, _ := roleRef["role"].(map[string]interface{})
	metadata, _ := role["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	require.Equal(suite.T(), "test-cluster-role-rel", name)
}

// local helper mirroring helpers_test.go but returning generic body
func (suite *CommonTestSuite) doRawGraphQL(url, query string) (int, map[string]interface{}) {
	reqBody := map[string]string{"query": query}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	// add auth token used by suite
	if suite.staticToken != "" {
		req.Header.Set("Authorization", "Bearer "+suite.staticToken)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	var body map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	require.NoError(suite.T(), dec.Decode(&body))
	return resp.StatusCode, body
}

// GraphQL payloads
func createClusterRoleForRelationMutation() string {
	return `mutation {
  rbac_authorization_k8s_io {
    createClusterRole(
      object: {
        metadata: { name: "test-cluster-role-rel" }
        rules: [{ apiGroups:[""], resources:["pods"], verbs:["get","list"] }]
      }
    ) { metadata { name } }
  }
}`
}

func createClusterRoleBindingForRelationMutation() string {
	return `mutation {
  rbac_authorization_k8s_io {
    createClusterRoleBinding(
      object: {
        metadata: { name: "test-crb-rel" }
        roleRef: {
          apiGroup: "rbac.authorization.k8s.io"
          kind: "ClusterRole"
          name: "test-cluster-role-rel"
        }
        subjects: []
      }
    ) { metadata { name } }
  }
}`
}

func getClusterRoleBindingWithRoleQuery() string {
	return `{
  rbac_authorization_k8s_io {
    ClusterRoleBinding(name: "test-crb-rel") {
      roleRef {
        name kind apiGroup
        role { metadata { name } }
      }
    }
  }
}`
}
