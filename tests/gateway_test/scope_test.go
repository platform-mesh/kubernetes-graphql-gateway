package gateway_test

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

func (suite *CommonTestSuite) TestCrudClusterRole() {
	workspaceName := "myWorkspace"

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// this url must be generated after new file added
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create ClusterRole and check results
	createResp, statusCode, err := suite.sendAuthenticatedRequest(url, CreateClusterRoleMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Get ClusterRole
	getResp, statusCode, err := suite.sendAuthenticatedRequest(url, GetClusterRoleQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	data := getResp.Data.RbacAuthorizationK8sIO.ClusterRole
	require.Equal(suite.T(), "test-cluster-role", data.Metadata.Name)

	// Delete ClusterRole
	deleteResp, statusCode, err := suite.sendAuthenticatedRequest(url, DeleteClusterRoleMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)

	// Try to get the ClusterRole after deletion
	getRespAfterDelete, statusCode, err := suite.sendAuthenticatedRequest(url, GetClusterRoleQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NotNil(suite.T(), getRespAfterDelete.Errors, "Expected error when querying deleted ClusterRole, but got none")
}
