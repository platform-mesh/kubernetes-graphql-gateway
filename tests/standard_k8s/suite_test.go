package standard_k8s_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/platform-mesh/golang-commons/logger/testlogger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/manager"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	testEnv       *envtest.Environment
	testCfg       *rest.Config
	testScheme    *runtime.Scheme
	testLog       logr.Logger
	commonsLogger *testlogger.TestLogger
)

type IntegrationTestSuite struct {
	suite.Suite
	k8sClient     client.Client
	ctx           context.Context
	cancel        context.CancelFunc
	schemaDir     string
	reconciler    reconciler.CustomReconciler
	gateway       *manager.Service
	gatewayServer *httptest.Server
}

type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

type GraphQLResponse struct {
	Data   interface{}    `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string        `json:"message"`
	Path    []interface{} `json:"path,omitempty"`
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func TestMain(m *testing.M) {
	commonsLogger = testlogger.New()
	testLog = commonsLogger.ComponentLogger("test-main").Logr()
	logf.SetLogger(commonsLogger.Logr())

	testLog.Info("Starting integration test suite")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	testLog.Info("Starting envtest control plane")
	testCfg, err = testEnv.Start()
	if err != nil {
		testLog.Error(err, "Failed to start test environment")
		os.Exit(1)
	}
	testLog.Info("Control plane started", "host", testCfg.Host)

	testScheme = runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(testScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(testScheme))
	utilruntime.Must(gatewayv1alpha1.AddToScheme(testScheme))

	code := m.Run()

	testLog.Info("Stopping envtest control plane")
	if err := testEnv.Stop(); err != nil {
		testLog.Error(err, "Failed to stop test environment")
	}

	os.Exit(code)
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(s.T().Context())

	var err error
	s.k8sClient, err = client.New(testCfg, client.Options{Scheme: testScheme})
	s.Require().NoError(err, "Failed to create Kubernetes client")

	s.schemaDir = s.T().TempDir()
	testLog.Info("Created temp schema directory", "schemaDir", s.schemaDir)

	ioHandler, err := workspacefile.NewIOHandler(s.schemaDir)
	s.Require().NoError(err, "Failed to create IO handler")

	schemaResolver := apischema.NewResolver(commonsLogger.ComponentLogger("schema-resolver"))

	mgrOpts := ctrl.Options{
		Scheme: testScheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
	}

	reconcilerOpts := reconciler.ReconcilerOpts{
		Scheme:                 testScheme,
		Client:                 s.k8sClient,
		Config:                 testCfg,
		ManagerOpts:            mgrOpts,
		OpenAPIDefinitionsPath: s.schemaDir,
	}

	appCfg := config.Config{
		OpenApiDefinitionsPath: s.schemaDir,
		LocalDevelopment:       true,
	}
	appCfg.Url.GraphqlSuffix = "graphql"

	testLog.Info("Creating ClusterAccess reconciler")
	s.reconciler, err = clusteraccess.NewClusterAccessReconciler(
		s.ctx,
		appCfg,
		reconcilerOpts,
		ioHandler,
		schemaResolver,
		commonsLogger.ComponentLogger("cluster-access-reconciler"),
	)
	s.Require().NoError(err, "Failed to create reconciler")

	testLog.Info("Starting reconciler manager")
	mgr := s.reconciler.GetManager()
	err = s.reconciler.SetupWithManager(mgr)
	s.Require().NoError(err, "Failed to setup reconciler")

	go func() {
		if err := mgr.Start(s.ctx); err != nil {
			testLog.Error(err, "Manager failed to start")
		}
	}()

	testLog.Info("Waiting for cache sync")
	s.Require().True(mgr.GetCache().WaitForCacheSync(s.ctx), "Cache failed to sync")

	testLog.Info("Creating gateway service")
	s.gateway, err = manager.NewGateway(s.ctx, commonsLogger.ComponentLogger("gateway"), appCfg)
	s.Require().NoError(err, "Failed to create gateway")

	testLog.Info("Starting gateway HTTP server")
	s.gatewayServer = httptest.NewServer(s.gateway)

	testLog.Info("Suite setup complete", "gatewayURL", s.gatewayServer.URL)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	testLog.Info("Suite teardown")

	if s.gatewayServer != nil {
		testLog.Info("Stopping gateway HTTP server")
		s.gatewayServer.Close()
	}

	if s.gateway != nil {
		testLog.Info("Closing gateway service")
		s.gateway.Close()
	}

	if s.cancel != nil {
		s.cancel()
	}
}

func (s *IntegrationTestSuite) uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func (s *IntegrationTestSuite) createClusterAccessForEnvtest(name string) *gatewayv1alpha1.ClusterAccess {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-cert",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"tls.crt": testCfg.CertData,
			"tls.key": testCfg.KeyData,
		},
	}
	s.Require().NoError(s.k8sClient.Create(s.ctx, secret))

	return &gatewayv1alpha1.ClusterAccess{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.ClusterAccessSpec{
			Host: testCfg.Host,
			Auth: &gatewayv1alpha1.AuthConfig{
				ClientCertificateRef: &gatewayv1alpha1.ClientCertificateRef{
					Name:      name + "-cert",
					Namespace: "default",
				},
			},
		},
	}
}

func (s *IntegrationTestSuite) executeGraphQL(clusterName string, req GraphQLRequest) *GraphQLResponse {
	body, err := json.Marshal(req)
	s.Require().NoError(err)

	reqURL, err := url.JoinPath(s.gatewayServer.URL, clusterName, "graphql")
	s.Require().NoError(err)
	testLog.Info("Executing GraphQL request", "url", reqURL, "cluster", clusterName)

	httpReq, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(body))
	s.Require().NoError(err)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(httpReq)
	s.Require().NoError(err)
	defer res.Body.Close()

	s.Require().Equal(http.StatusOK, res.StatusCode, "Expected 200 status code. URL: %s", reqURL)

	var result GraphQLResponse
	err = json.NewDecoder(res.Body).Decode(&result)
	s.Require().NoError(err)

	return &result
}

func (s *IntegrationTestSuite) waitForClusterReady(name string) {
	s.Eventually(func() bool {
		cluster, ok := s.gateway.GetClusterRegistry().GetCluster(name)
		if !ok {
			testLog.Info("Cluster not yet available", "name", name)
			return false
		}
		if cluster == nil {
			testLog.Info("Cluster is nil", "name", name)
			return false
		}
		testLog.Info("Cluster is ready", "name", name)
		return true
	}, 10*time.Second, 500*time.Millisecond, "Cluster %s should be loaded in gateway", name)
}

func (s *IntegrationTestSuite) setupTestCluster(name string) *gatewayv1alpha1.ClusterAccess {
	ca := s.createClusterAccessForEnvtest(name)
	s.Require().NoError(s.k8sClient.Create(s.ctx, ca))
	s.waitForClusterReady(name)
	testLog.Info("Test cluster ready", "name", name)
	return ca
}

func (s *IntegrationTestSuite) cleanupTestCluster(ca *gatewayv1alpha1.ClusterAccess) {
	s.Require().NoError(s.k8sClient.Delete(s.ctx, ca))
	testLog.Info("Cleaned up cluster", "name", ca.Name)
}
