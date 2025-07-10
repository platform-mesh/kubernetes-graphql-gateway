package clusteraccess

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/openmfp/golang-commons/controller/lifecycle"
	commonserrors "github.com/openmfp/golang-commons/errors"
	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
)

// generateSchemaSubroutine processes ClusterAccess resources and generates schemas
type generateSchemaSubroutine struct {
	reconciler *ClusterAccessReconciler
}

func (s *generateSchemaSubroutine) Process(ctx context.Context, instance lifecycle.RuntimeObject) (ctrl.Result, commonserrors.OperatorError) {
	clusterAccess, ok := instance.(*gatewayv1alpha1.ClusterAccess)
	if !ok {
		s.reconciler.log.Error().Msg("instance is not a ClusterAccess resource")
		return ctrl.Result{}, commonserrors.NewOperatorError(errors.New("invalid resource type"), false, false)
	}

	clusterAccessName := clusterAccess.GetName()
	s.reconciler.log.Info().Str("clusterAccess", clusterAccessName).Msg("processing ClusterAccess resource")

	// Extract target cluster config from ClusterAccess spec
	targetConfig, clusterName, err := BuildTargetClusterConfigFromTyped(*clusterAccess, s.reconciler.opts.Client)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to build target cluster config")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	s.reconciler.log.Info().Str("clusterAccess", clusterAccessName).Str("host", targetConfig.Host).Str("clusterName", clusterName).Msg("extracted target cluster config")

	// Create discovery client for target cluster
	targetDiscovery, err := discovery.NewDiscoveryClientForConfig(targetConfig)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to create discovery client")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Create REST mapper for target cluster
	targetRM, err := s.restMapperFromConfig(targetConfig)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to create REST mapper")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Create schema resolver for target cluster
	targetResolver := &apischema.CRDResolver{
		DiscoveryInterface: targetDiscovery,
		RESTMapper:         targetRM,
	}

	// Generate schema for target cluster
	JSON, err := targetResolver.Resolve(targetDiscovery, targetRM)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to resolve schema")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Create the complete schema file with x-cluster-metadata
	schemaWithMetadata, err := injectClusterMetadata(JSON, *clusterAccess, s.reconciler.opts.Client, s.reconciler.log)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to inject cluster metadata")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Write schema to file using cluster name from path or resource name
	if err := s.reconciler.ioHandler.Write(schemaWithMetadata, clusterName); err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to write schema")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	s.reconciler.log.Info().Str("clusterAccess", clusterAccessName).Msg("successfully processed ClusterAccess resource")
	return ctrl.Result{}, nil
}

// restMapperFromConfig creates a REST mapper from a config
func (s *generateSchemaSubroutine) restMapperFromConfig(cfg *rest.Config) (meta.RESTMapper, error) {
	httpClt, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, errors.Join(reconciler.ErrCreateHTTPClient, err)
	}
	rm, err := apiutil.NewDynamicRESTMapper(cfg, httpClt)
	if err != nil {
		return nil, errors.Join(reconciler.ErrCreateRESTMapper, err)
	}

	return rm, nil
}

func (s *generateSchemaSubroutine) Finalize(ctx context.Context, instance lifecycle.RuntimeObject) (ctrl.Result, commonserrors.OperatorError) {
	return ctrl.Result{}, nil
}

func (s *generateSchemaSubroutine) GetName() string {
	return "generate-schema"
}

func (s *generateSchemaSubroutine) Finalizers() []string {
	return nil
}
