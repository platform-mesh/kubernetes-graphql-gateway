package gateway_test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"path/filepath"
	"strings"
)

func (suite *CommonTestSuite) TestTokenValidation() {
	suite.LocalDevelopment = false
	suite.SetupTest()
	defer func() {
		suite.LocalDevelopment = true
		suite.TearDownTest()
	}()

	workspaceName := "myWorkspace"

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	req, err := http.NewRequest("POST", url, nil)
	require.NoError(suite.T(), err)

	// Use the BearerToken from restCfg, which is valid for envtest
	req.Header.Set("Authorization", "Bearer "+suite.restCfg.BearerToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	require.NotEqual(suite.T(), http.StatusUnauthorized, resp.StatusCode, "Token should be valid for test cluster")
}

func (suite *CommonTestSuite) TestIntrospectionAuth() {
	suite.LocalDevelopment = false
	suite.AuthenticateSchemaRequests = true
	suite.SetupTest()
	defer func() {
		suite.LocalDevelopment = true
		suite.AuthenticateSchemaRequests = false
		suite.TearDownTest()
	}()

	workspaceName := "myWorkspace"

	require.NoError(suite.T(), writeToFile(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Introspection query
	introspectionBody := `{"query":"query { __schema { queryType { name } } }"}`

	req, err := http.NewRequest("POST", url, strings.NewReader(introspectionBody))
	require.NoError(suite.T(), err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.restCfg.BearerToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	require.NotEqual(suite.T(), http.StatusUnauthorized, resp.StatusCode, "Token should be valid for introspection query")
}
