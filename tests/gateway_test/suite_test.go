package gateway_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/graphql-go/graphql"
	"github.com/openmfp/golang-commons/logger"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zapcore"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/kcp"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openmfp/account-operator/api/v1alpha1"
	appConfig "github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/manager"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

func TestMain(m *testing.M) {
	var zapLevel zapcore.Level
	level := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch level {
	case "ERROR":
		zapLevel = zapcore.ErrorLevel
	case "WARN":
		zapLevel = zapcore.WarnLevel
	case "INFO":
		zapLevel = zapcore.InfoLevel
	case "DEBUG", "TRACE":
		zapLevel = zapcore.DebugLevel
	default:
		zapLevel = zapcore.ErrorLevel
	}
	log.SetLogger(zap.New(zap.UseDevMode(false), zap.Level(zapLevel)))
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
	manager       manager.Provider
	server        *httptest.Server
}

func TestCommonTestSuite(t *testing.T) {
	suite.Run(t, new(CommonTestSuite))
}

func (suite *CommonTestSuite) SetupTest() {
	runtimeScheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(runtimeScheme))
	utilruntime.Must(appsv1.AddToScheme(runtimeScheme))
	utilruntime.Must(v1.AddToScheme(runtimeScheme))
	utilruntime.Must(corev1.AddToScheme(runtimeScheme))

	var err error
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			// this is needed for the CRD registration
			filepath.Join("testdata", "crd"),
		},
	}
	suite.restCfg, err = suite.testEnv.Start()
	require.NoError(suite.T(), err)

	suite.appCfg.OpenApiDefinitionsPath, err = os.MkdirTemp("", "watchedDir")
	require.NoError(suite.T(), err)

	suite.appCfg.LocalDevelopment = true
	suite.appCfg.Gateway.Cors.Enabled = true

	suite.log, err = logger.New(logger.DefaultConfig())
	require.NoError(suite.T(), err)

	suite.runtimeClient, err = kcp.NewClusterAwareClientWithWatch(suite.restCfg, client.Options{
		Scheme: runtimeScheme,
	})
	require.NoError(suite.T(), err)

	definitions, err := manager.ReadDefinitionFromFile("./testdata/kubernetes")
	require.NoError(suite.T(), err)

	g, err := schema.New(suite.log, definitions, resolver.New(suite.log, suite.runtimeClient))
	require.NoError(suite.T(), err)

	suite.graphqlSchema = *g.GetSchema()

	suite.manager, err = manager.NewManager(suite.log, suite.restCfg, suite.appCfg)
	require.NoError(suite.T(), err)

	suite.server = httptest.NewServer(suite.manager)
}

func (suite *CommonTestSuite) TearDownTest() {
	require.NoError(suite.T(), os.RemoveAll(suite.appCfg.OpenApiDefinitionsPath))
	require.NoError(suite.T(), suite.testEnv.Stop())
	suite.server.Close()
}
