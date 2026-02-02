package subroutine

import (
	"context"
	"fmt"
	"strings"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	lifecyclesubroutine "github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"

	kcpcorev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
)

type DiscoveryFactory interface {
	ClientForCluster(name string) (discovery.DiscoveryInterface, error)
	RestMapperForCluster(name string) (meta.RESTMapper, error)
}

type SchemaGenerationParams struct {
	ClusterPath     string
	DiscoveryClient discovery.DiscoveryInterface
	RESTMapper      meta.RESTMapper
	HostOverride    string
}

type SchemaGeneratorFunc func(params SchemaGenerationParams, apiSchemaResolver apischema.Resolver, log *logger.Logger) ([]byte, error)

type schemaGenerator struct {
	discoveryFactory    DiscoveryFactory
	apiSchemaResolver   apischema.Resolver
	ioHandler           *workspacefile.FileHandler
	schemaGeneratorFunc SchemaGeneratorFunc
	log                 *logger.Logger
}

func NewSchemaGenerator(
	discoveryFactory DiscoveryFactory,
	apiSchemaResolver apischema.Resolver,
	ioHandler *workspacefile.FileHandler,
	schemaGeneratorFunc SchemaGeneratorFunc,
	log *logger.Logger,
) *schemaGenerator {
	return &schemaGenerator{
		discoveryFactory:    discoveryFactory,
		apiSchemaResolver:   apiSchemaResolver,
		ioHandler:           ioHandler,
		schemaGeneratorFunc: schemaGeneratorFunc,
		log:                 log,
	}
}

var _ lifecyclesubroutine.Subroutine = &schemaGenerator{}

func (s *schemaGenerator) GetName() string { return "SchemaGenerator" }

func (s *schemaGenerator) Finalizers(_ runtimeobject.RuntimeObject) []string { return []string{} }

func (s *schemaGenerator) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (s *schemaGenerator) Process(ctx context.Context, instance runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	lc := instance.(*kcpcorev1alpha1.LogicalCluster)

	// ignore system workspaces
	if strings.HasPrefix(lc.Name, "system") {
		return ctrl.Result{}, nil
	}

	clusterPath, ok := lc.Annotations["kcp.io/path"]
	if !ok {
		s.log.Error().Msg("missing kcp.io/path annotation on LogicalCluster")
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("missing kcp.io/path annotation"), false, false)
	}

	s.log.Info().Str("clusterPath", clusterPath).Msg("generating schema for initializing workspace...")

	dc, err := s.discoveryFactory.ClientForCluster(clusterPath)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("failed to create discovery client for cluster: %w", err), true, true)
	}

	rm, err := s.discoveryFactory.RestMapperForCluster(clusterPath)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("failed to create rest mapper for cluster: %w", err), true, true)
	}

	currentSchema, err := s.schemaGeneratorFunc(
		SchemaGenerationParams{
			ClusterPath:     clusterPath,
			DiscoveryClient: dc,
			RESTMapper:      rm,
		},
		s.apiSchemaResolver,
		s.log,
	)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("failed to generate schema: %w", err), true, true)
	}

	if err := s.ioHandler.Write(currentSchema, clusterPath); err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("failed to write schema to filesystem: %w", err), true, true)
	}

	s.log.Info().Str("clusterPath", clusterPath).Msg("schema file generated and saved")

	return ctrl.Result{}, nil
}
