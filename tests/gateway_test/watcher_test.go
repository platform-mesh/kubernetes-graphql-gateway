package gateway_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/stretchr/testify/require"
)

func (suite *CommonTestSuite) TestWorkspaceRemove() {
	workspaceName := "myWorkspace"
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// Create the Pod
	_, statusCode, err := sendRequest(url, createPodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")

	err = os.Remove(filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName))
	require.NoError(suite.T(), err)

	// Wait until the handler is removed
	time.Sleep(sleepTime)

	// Attempt to access the URL again
	_, statusCode, _ = sendRequest(url, createPodMutation())
	require.Equal(suite.T(), http.StatusNotFound, statusCode, "Expected StatusNotFound after handler is removed")
}

func (suite *CommonTestSuite) TestWorkspaceRename() {
	workspaceName := "myWorkspace"
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// Create the Pod
	_, statusCode, err := sendRequest(url, createPodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")

	newWorkspaceName := "myNewWorkspace"
	err = os.Rename(filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName), filepath.Join(suite.appCfg.OpenApiDefinitionsPath, newWorkspaceName))
	require.NoError(suite.T(), err)
	time.Sleep(sleepTime) // let's give some time to the manager to process the file and create a url

	// old url should not be accessible, status should be NotFound
	_, statusCode, _ = sendRequest(url, createPodMutation())
	require.Equal(suite.T(), http.StatusNotFound, statusCode, "Expected StatusNotFound after workspace rename")

	// now new url should be accessible
	newUrl := fmt.Sprintf("%s/%s/graphql", suite.server.URL, newWorkspaceName)
	_, statusCode, err = sendRequest(newUrl, createPodMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
}
