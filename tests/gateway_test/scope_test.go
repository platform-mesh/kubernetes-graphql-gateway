package gateway_test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"path/filepath"
)

func (suite *CommonTestSuite) TestCrudClusterRole() {
	workspaceName := "myWorkspace"

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// this url must be generated after new file added
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create ClusterRole and check results
	createResp, statusCode, err := sendRequest(url, CreateClusterRoleMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Get ClusterRole
	getResp, statusCode, err := sendRequest(url, GetClusterRoleQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	data := getResp.Data.RbacAuthorizationK8sIO.ClusterRole
	require.Equal(suite.T(), "test-cluster-role", data.Metadata.Name)

	// Delete ClusterRole
	deleteResp, statusCode, err := sendRequest(url, DeleteClusterRoleMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)

	// Try to get the ClusterRole after deletion
	getRespAfterDelete, statusCode, err := sendRequest(url, GetClusterRoleQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NotNil(suite.T(), getRespAfterDelete.Errors, "Expected error when querying deleted ClusterRole, but got none")
}
