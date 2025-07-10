package gateway_test

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// TestCreateGetAndDeleteAccount tests the creation, retrieval, and deletion of an account resource.
func (suite *CommonTestSuite) TestCreateGetAndDeleteAccount() {
	workspaceName := "myWorkspace"
	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	// Create the account and verify the response
	createResp, statusCode, err := suite.sendAuthenticatedRequest(url, createAccountMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Retrieve the account and verify its details
	getResp, statusCode, err := suite.sendAuthenticatedRequest(url, getAccountQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	accountData := getResp.Data.CoreOpenmfpOrg.Account
	require.Equal(suite.T(), "test-account", accountData.Metadata.Name)
	require.Equal(suite.T(), "test-account-display-name", accountData.Spec.DisplayName)
	require.Equal(suite.T(), "account", accountData.Spec.Type)

	// Delete the account and verify the response
	deleteResp, statusCode, err := suite.sendAuthenticatedRequest(url, deleteAccountMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)

	// Attempt to retrieve the account after deletion and expect an error
	getRespAfterDelete, statusCode, err := suite.sendAuthenticatedRequest(url, getAccountQuery())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.NotNil(suite.T(), getRespAfterDelete.Errors, "Expected error when querying deleted account, but got none")
}
