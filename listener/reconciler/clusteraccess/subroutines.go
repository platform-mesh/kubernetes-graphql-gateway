package clusteraccess

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	commonserrors "github.com/platform-mesh/golang-commons/errors"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"
)

// generateSchemaSubroutine processes ClusterAccess resources and generates schemas
const (
	finalizerName = "gateway.platform-mesh.io/clusteraccess-finalizer"
)

// generateSchemaSubroutine processes ClusterAccess resources and generates schemas
type generateSchemaSubroutine struct {
	reconciler *ClusterAccessReconciler
}

func (s *generateSchemaSubroutine) Process(ctx context.Context, instance runtimeobject.RuntimeObject) (ctrl.Result, commonserrors.OperatorError) {
	clusterAccess, ok := instance.(*gatewayv1alpha1.ClusterAccess)
	if !ok {
		s.reconciler.log.Error().Msg("instance is not a ClusterAccess resource")
		return ctrl.Result{}, commonserrors.NewOperatorError(errors.New("invalid resource type"), false, false)
	}

	clusterAccessName := clusterAccess.GetName()
	s.reconciler.log.Info().Str("clusterAccess", clusterAccessName).Msg("processing ClusterAccess resource")

	// Extract target cluster config from ClusterAccess spec
	targetConfig, clusterName, err := BuildTargetClusterConfigFromTyped(ctx, *clusterAccess, s.reconciler.opts.Client)
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
	targetResolver := apischema.NewCRDResolver(targetDiscovery, targetRM, s.reconciler.log)

	// Generate schema for target cluster
	JSON, err := targetResolver.Resolve(targetDiscovery, targetRM)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to resolve schema")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Create the complete schema file with x-cluster-metadata
	schemaWithMetadata, err := injectClusterMetadata(ctx, JSON, *clusterAccess, s.reconciler.opts.Client, s.reconciler.log)
	if err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to inject cluster metadata")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// If path changed, delete the old schema file referenced in the status
	prevPath := clusterAccess.Status.ObservedPath
	if prevPath != "" && prevPath != clusterName {
		if err := s.reconciler.ioHandler.Delete(prevPath); err != nil {
			// Log and continue; do not fail reconciliation on cleanup issues
			s.reconciler.log.Warn().Err(err).Str("previousPath", prevPath).Str("clusterAccess", clusterAccessName).Msg("failed to delete previous schema file")
		}
	}

	// Write schema to file using cluster name from path or resource name
	if err := s.reconciler.ioHandler.Write(schemaWithMetadata, clusterName); err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to write schema")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Update status.ObservedPath to reflect the current observed path
	if prevPath != clusterName {
		obj := clusterAccess.DeepCopy()
		obj.Status.ObservedPath = clusterName
		if err := s.reconciler.opts.Client.Status().Update(ctx, obj); err != nil {
			// Log but do not fail reconciliation; file has been written already
			s.reconciler.log.Warn().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to update observed path in status")
		} else {
			clusterAccess.Status.ObservedPath = obj.Status.ObservedPath
		}
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

func (s *generateSchemaSubroutine) Finalize(ctx context.Context, instance runtimeobject.RuntimeObject) (ctrl.Result, commonserrors.OperatorError) {
	clusterAccess, ok := instance.(*gatewayv1alpha1.ClusterAccess)
	if !ok {
		s.reconciler.log.Error().Msg("instance is not a ClusterAccess resource in Finalize")
		return ctrl.Result{}, commonserrors.NewOperatorError(errors.New("invalid resource type"), false, false)
	}

	// Determine current and previously used paths
	currentPath := clusterAccess.Spec.Path
	if currentPath == "" {
		currentPath = clusterAccess.GetName()
	}
	prevPath := clusterAccess.Status.ObservedPath

	// Try deleting current path file
	if currentPath != "" {
		if err := s.reconciler.ioHandler.Delete(currentPath); err != nil {
			// Log and continue; do not block finalization just because file was missing or deletion failed
			s.reconciler.log.Warn().Err(err).Str("path", currentPath).Str("clusterAccess", clusterAccess.GetName()).Msg("failed to delete schema file during finalization")
		}
	}
	// If previous differs, try deleting it as well
	if prevPath != "" && prevPath != currentPath {
		if err := s.reconciler.ioHandler.Delete(prevPath); err != nil {
			s.reconciler.log.Warn().Err(err).Str("path", prevPath).Str("clusterAccess", clusterAccess.GetName()).Msg("failed to delete previous schema file during finalization")
		}
	}

	return ctrl.Result{}, nil
}

func (s *generateSchemaSubroutine) GetName() string {
	return "generate-schema"
}

func (s *generateSchemaSubroutine) Finalizers() []string {
	return []string{finalizerName}
}
