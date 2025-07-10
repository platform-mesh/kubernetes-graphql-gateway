package gateway_test

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// TestCreateGetAndDeletePod generates a schema then creates a Pod, gets it and deletes it.
func (suite *CommonTestSuite) TestCreateGetAndDeletePod() {
	workspaceName := "myWorkspace"

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// this url must be generated after new file added
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create the Pod and check results
	createResp, statusCode, err := suite.sendAuthenticatedRequest(url, createPodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Get the Pod
	getResp, statusCode, err := suite.sendAuthenticatedRequest(url, getPodQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	podData := getResp.Data.Core.Pod
	require.Equal(suite.T(), "test-pod", podData.Metadata.Name)
	require.Equal(suite.T(), "default", podData.Metadata.Namespace)
	require.Equal(suite.T(), "test-container", podData.Spec.Containers[0].Name)
	require.Equal(suite.T(), "nginx", podData.Spec.Containers[0].Image)

	// Delete the Pod
	deleteResp, statusCode, err := suite.sendAuthenticatedRequest(url, deletePodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)

	// Try to get the Pod after deletion
	getRespAfterDelete, statusCode, err := suite.sendAuthenticatedRequest(url, getPodQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NotNil(suite.T(), getRespAfterDelete.Errors, "Expected error when querying deleted Pod, but got none")
}
