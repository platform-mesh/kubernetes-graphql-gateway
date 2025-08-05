package gateway_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openmfp/account-operator/api/v1alpha1"
	"github.com/openmfp/golang-commons/logger"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Initialize the logger for the test suite
// This is necessary to avoid the "[controller-runtime] log.SetLogger(...) was never called" error
// when running the tests
func TestMain(m *testing.M) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	os.Exit(m.Run())
}

type CommonTestSuite struct {
	suite.Suite
	testEnv       *envtest.Environment
	log           *logger.Logger
	restCfg       *rest.Config
	appCfg        appConfig.Config
	runtimeClient client.WithWatch
	graphqlSchema graphql.Schema
	manager       http.Handler
	server        *httptest.Server

	LocalDevelopment           bool
	AuthenticateSchemaRequests bool

	staticTokenFile    string
	staticToken        string
	originalKubeconfig string
	tempKubeconfigFile string
}

func TestCommonTestSuite(t *testing.T) {
	suite.Run(t, new(CommonTestSuite))
}

func (suite *CommonTestSuite) SetupSuite() {
	suite.LocalDevelopment = true
}

func (suite *CommonTestSuite) SetupTest() {
	// Store and clear KUBECONFIG to prevent interference with test environment
	suite.originalKubeconfig = os.Getenv("KUBECONFIG")
	os.Unsetenv("KUBECONFIG")

	runtimeScheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(appsv1.AddToScheme(runtimeScheme))
	utilruntime.Must(v1.AddToScheme(runtimeScheme))
	utilruntime.Must(corev1.AddToScheme(runtimeScheme))

	var err error

	// 1. Generate a static token and write it to a file
	suite.staticToken = "test-token-123"
	tokenFile, err := os.CreateTemp("", "static-token.csv")
	require.NoError(suite.T(), err)
	_, err = tokenFile.WriteString(fmt.Sprintf("%s,admin,admin,system:masters\n", suite.staticToken))
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), tokenFile.Close())
	suite.staticTokenFile = tokenFile.Name()

	// 2. Prepare envtest.Environment and configure the API server with the token file
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("testdata", "crd"),
		},
	}
	// Add the token-auth-file argument before starting the environment
	suite.testEnv.ControlPlane.GetAPIServer().Configure().Append("token-auth-file", suite.staticTokenFile)

	suite.restCfg, err = suite.testEnv.Start()
	require.NoError(suite.T(), err)

	// 3. Set BearerToken in restCfg
	suite.restCfg.BearerToken = suite.staticToken

	// 4. Create a temporary kubeconfig file from our test restCfg and set KUBECONFIG to it
	suite.tempKubeconfigFile, err = suite.createTempKubeconfig()
	require.NoError(suite.T(), err)
	os.Setenv("KUBECONFIG", suite.tempKubeconfigFile)

	suite.appCfg.OpenApiDefinitionsPath, err = os.MkdirTemp("", "watchedDir")
	require.NoError(suite.T(), err)

	suite.appCfg.LocalDevelopment = suite.LocalDevelopment
	suite.appCfg.Gateway.Cors.Enabled = true
	suite.appCfg.IntrospectionAuthentication = suite.AuthenticateSchemaRequests

	// Set URL configuration for the gateway tests
	suite.appCfg.Url.VirtualWorkspacePrefix = "virtual-workspace"
	suite.appCfg.Url.DefaultKcpWorkspace = "root"
	suite.appCfg.Url.GraphqlSuffix = "graphql"

	suite.log, err = logger.New(logger.DefaultConfig())
	require.NoError(suite.T(), err)

	suite.runtimeClient, err = kcp.NewClusterAwareClientWithWatch(suite.restCfg, client.Options{
		Scheme: runtimeScheme,
	})
	require.NoError(suite.T(), err)

	// Create resolver service with the logger pointer
	resolverService := resolver.New(suite.log, suite.runtimeClient)

	definitions, err := readDefinitionFromFile("./testdata/kubernetes")
	require.NoError(suite.T(), err)

	g, err := schema.New(suite.log, definitions, resolverService)
	require.NoError(suite.T(), err)

	suite.graphqlSchema = *g.GetSchema()

	suite.manager, err = manager.NewGateway(suite.T().Context(), suite.log, suite.appCfg)
	require.NoError(suite.T(), err)

	suite.server = httptest.NewServer(suite.manager)
}

func (suite *CommonTestSuite) TearDownTest() {
	require.NoError(suite.T(), os.RemoveAll(suite.appCfg.OpenApiDefinitionsPath))
	require.NoError(suite.T(), suite.testEnv.Stop())
	suite.server.Close()

	// Clean up the token file
	if suite.staticTokenFile != "" {
		os.Remove(suite.staticTokenFile)
	}

	// Clean up the temporary kubeconfig file
	if suite.tempKubeconfigFile != "" {
		os.Remove(suite.tempKubeconfigFile)
	}

	// Restore original KUBECONFIG if it was set
	if suite.originalKubeconfig != "" {
		os.Setenv("KUBECONFIG", suite.originalKubeconfig)
	}
}

// createTempKubeconfig creates a temporary kubeconfig file from the test environment's rest.Config
func (suite *CommonTestSuite) createTempKubeconfig() (string, error) {
	// Create a temporary kubeconfig file
	tempKubeconfig, err := os.CreateTemp("", "test-kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	defer tempKubeconfig.Close()

	// Create a kubeconfig structure
	kubeconfig := &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"test-cluster": {
				Server:                suite.restCfg.Host,
				InsecureSkipTLSVerify: suite.restCfg.TLSClientConfig.Insecure,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"test-context": {
				Cluster:   "test-cluster",
				AuthInfo:  "test-user",
				Namespace: "default",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test-user": {
				Token: suite.restCfg.BearerToken,
			},
		},
		CurrentContext: "test-context",
	}

	// Add CA data if present
	if len(suite.restCfg.TLSClientConfig.CAData) > 0 {
		kubeconfig.Clusters["test-cluster"].CertificateAuthorityData = suite.restCfg.TLSClientConfig.CAData
		kubeconfig.Clusters["test-cluster"].InsecureSkipTLSVerify = false
	}

	// Write the kubeconfig to the temporary file
	err = clientcmd.WriteToFile(*kubeconfig, tempKubeconfig.Name())
	if err != nil {
		return "", err
	}

	return tempKubeconfig.Name(), nil
}

// writeToFileWithClusterMetadata writes an enhanced schema file with cluster metadata for cluster access mode
func (suite *CommonTestSuite) writeToFileWithClusterMetadata(from, to string) error {
	// Read the base schema file (definitions only)
	definitions, err := readDefinitionFromFile(from)
	if err != nil {
		return fmt.Errorf("failed to read base schema: %w", err)
	}

	// Create schema data with cluster metadata
	schemaData := map[string]interface{}{
		"definitions": definitions,
		"x-cluster-metadata": map[string]interface{}{
			"host": suite.restCfg.Host,
			"auth": map[string]interface{}{
				"type":  "token",
				"token": base64.StdEncoding.EncodeToString([]byte(suite.staticToken)),
			},
		},
	}

	// Add CA data if present
	if len(suite.restCfg.TLSClientConfig.CAData) > 0 {
		schemaData["x-cluster-metadata"].(map[string]interface{})["ca"] = map[string]interface{}{
			"data": base64.StdEncoding.EncodeToString(suite.restCfg.TLSClientConfig.CAData),
		}
	}

	// Write the enhanced schema file
	data, err := json.Marshal(schemaData)
	if err != nil {
		return fmt.Errorf("failed to marshal schema data: %w", err)
	}

	err = os.WriteFile(to, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	// let's give some time to the manager to process the file and create a url
	time.Sleep(sleepTime)

	return nil
}

// sendAuthenticatedRequest is a helper method to send authenticated GraphQL requests using the test token
func (suite *CommonTestSuite) sendAuthenticatedRequest(url, query string) (*GraphQLResponse, int, error) {
	return sendRequestWithAuth(url, query, suite.staticToken)
}

// sendAuthenticatedRequestWithVariables is a helper method to send authenticated GraphQL requests with variables using the test token
func (suite *CommonTestSuite) sendAuthenticatedRequestWithVariables(url, query string, variables map[string]interface{}) (*GraphQLResponse, int, error) {
	return sendRequestWithAuthAndVariables(url, query, suite.staticToken, variables)
}
