package resource_test

import (
	"flag"
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
}

func TestResourceControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceControllerTestSuite))
}

func (suite *ResourceControllerTestSuite) SetupSuite() {
	klog.InitFlags(nil)
	_ = flag.Set("v", "5")

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

	opts := options.NewOptions()
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

	go func() {
		err = listenerConfig.Manager.Start(suite.T().Context())
		suite.Require().NoError(err, "failed to start multi-cluster manager")
	}()
}

func (suite *ResourceControllerTestSuite) TestSchemaGeneration() {
	suite.Eventually(func() bool {
		_, err := os.Stat(filepath.Join(suite.listenerCfg.Options.SchemasDir, "default"))
		return err == nil
	}, 10*time.Second, 500*time.Millisecond, "expected schema file to be generated")
}

func (suite *ResourceControllerTestSuite) TearDownSuite() {
	err := suite.env.Stop()
	suite.Require().NoError(err, "failed to stop test environment")

	err = os.RemoveAll(suite.listenerCfg.Options.SchemasDir)
	suite.Require().NoError(err, "failed to remove schemas directory")
}
