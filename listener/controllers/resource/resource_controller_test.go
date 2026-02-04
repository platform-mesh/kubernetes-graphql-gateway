package resource_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/controllers/resource"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/options"
	"github.com/stretchr/testify/suite"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
	"sigs.k8s.io/multicluster-runtime/providers/single"
)

type ResourceControllerTestSuite struct {
	suite.Suite

	env         *envtest.Environment
	listenerCfg *listener.Config
	cancel      context.CancelFunc
}

func TestResourceControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceControllerTestSuite))
}

func (suite *ResourceControllerTestSuite) SetupSuite() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.SetLogger(klog.NewKlogr())

	suite.env = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := suite.env.Start()
	suite.Require().NoError(err, "failed to start test environment")

	tmpDir := suite.T().TempDir()

	// Write the kubeconfig bytes to a temp file for the listener config
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err = os.WriteFile(kubeconfigPath, suite.env.KubeConfig, 0600)
	suite.Require().NoError(err, "failed to write kubeconfig")

	opts := options.NewOptions()
	opts.KubeConfig = kubeconfigPath
	opts.SchemasDir = filepath.Join(tmpDir, "schemas")

	completedOpts, err := opts.Complete()
	suite.Require().NoError(err, "failed to complete options")

	listenerConfig, err := listener.NewConfig(completedOpts)
	suite.Require().NoError(err, "failed to create listener config")

	defaultCluster, err := cluster.New(cfg)
	suite.Require().NoError(err, "failed to create default cluster")

	listenerConfig.Provider = single.New("default", defaultCluster)

	r, err := resource.New(
		suite.T().Context(),
		listenerConfig.Manager,
		controller.TypedOptions[mcreconcile.Request]{},
		listenerConfig.SchemaHandler,
		listenerConfig.SchemaResolver,
		listenerConfig.Options.AnchorResource,
		listenerConfig.Options.ResourceGVR,
		listenerConfig.Options.ClusterMetadataFunc,
		listenerConfig.Options.ClusterURLResolverFunc,
	)
	suite.Require().NoError(err, "failed to create resource reconciler")
	err = r.SetupWithManager(listenerConfig.Manager)
	suite.Require().NoError(err, "failed to setup resource reconciler with manager")

	suite.listenerCfg = listenerConfig

	ctx, cancel := context.WithCancel(suite.T().Context())
	suite.cancel = cancel

	go func() {
		err = listenerConfig.Manager.Start(ctx)
		suite.Require().NoError(err, "failed to start multi-cluster manager")
	}()
}

func (suite *ResourceControllerTestSuite) TestSchemaGeneration() {
	suite.Eventually(func() bool {
		_, err := os.Stat(filepath.Join(suite.listenerCfg.Options.SchemasDir, "default"))
		return err == nil
	}, 5*time.Second, 500*time.Millisecond, "expected schema file to be generated")
}

func (suite *ResourceControllerTestSuite) TearDownSuite() {
	// Cancel the manager context first to allow graceful shutdown
	suite.cancel()

	err := suite.env.Stop()
	suite.Require().NoError(err, "failed to stop test environment")
}
