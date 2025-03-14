package gateway_test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"path/filepath"
)

// TestCreateGetAndDeletePod generates a schema then creates a Pod, gets it and deletes it.
func (suite *CommonTestSuite) TestCreateGetAndDeletePod() {
	workspaceName := "myWorkspace"

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// this url must be generated after new file added
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create the Pod and check results
	createResp, statusCode, err := sendRequest(url, createPodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Get the Pod
	getResp, statusCode, err := sendRequest(url, getPodQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	podData := getResp.Data.Core.Pod
	require.Equal(suite.T(), "test-pod", podData.Metadata.Name)
	require.Equal(suite.T(), "default", podData.Metadata.Namespace)
	require.Equal(suite.T(), "test-container", podData.Spec.Containers[0].Name)
	require.Equal(suite.T(), "nginx", podData.Spec.Containers[0].Image)

	// Delete the Pod
	deleteResp, statusCode, err := sendRequest(url, deletePodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)

	// Try to get the Pod after deletion
	getRespAfterDelete, statusCode, err := sendRequest(url, getPodQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NotNil(suite.T(), getRespAfterDelete.Errors, "Expected error when querying deleted Pod, but got none")
}
