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

const lastSchemaFilenameAnnotation = "platform-mesh.io/last-schema-filename"

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

	// Before writing, check if the schema filename changed compared to previous reconcile.
	ann := clusterAccess.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	prevFile := ann[lastSchemaFilenameAnnotation]
	if prevFile != "" && prevFile != clusterName {
		if err := s.reconciler.ioHandler.Delete(prevFile); err != nil {
			s.reconciler.log.Warn().Err(err).Str("prevFile", prevFile).Str("clusterAccess", clusterAccessName).Msg("failed to delete previous schema file; continuing")
		} else {
			s.reconciler.log.Info().Str("deletedFile", prevFile).Str("clusterAccess", clusterAccessName).Msg("deleted previous schema file after path change")
		}
	}

	// Write schema to file using cluster name from path or resource name
	if err := s.reconciler.ioHandler.Write(schemaWithMetadata, clusterName); err != nil {
		s.reconciler.log.Error().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to write schema")
		return ctrl.Result{}, commonserrors.NewOperatorError(err, false, false)
	}

	// Update the annotation to reflect the current schema filename
	if prevFile != clusterName {
		ann[lastSchemaFilenameAnnotation] = clusterName
		clusterAccess.SetAnnotations(ann)
		if err := s.reconciler.opts.Client.Update(ctx, clusterAccess); err != nil {
			s.reconciler.log.Warn().Err(err).Str("clusterAccess", clusterAccessName).Msg("failed to update annotation with last schema filename")
		}
	}

	s.reconciler.log.Info().Str("clusterAccess", clusterAccessName).Str("schemaFile", clusterName).Msg("successfully processed ClusterAccess resource")
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

	// Determine which file to delete: prefer the recorded annotation, fallback to computed name
	ann := clusterAccess.GetAnnotations()
	var filename string
	if ann != nil && ann[lastSchemaFilenameAnnotation] != "" {
		filename = ann[lastSchemaFilenameAnnotation]
	} else {
		// compute from spec.path or name
		filename = clusterAccess.Spec.Path
		if filename == "" {
			filename = clusterAccess.GetName()
		}
	}

	if filename == "" {
		return ctrl.Result{}, nil
	}

	if err := s.reconciler.ioHandler.Delete(filename); err != nil {
		s.reconciler.log.Warn().Err(err).Str("clusterAccess", clusterAccess.GetName()).Str("file", filename).Msg("failed to delete schema file on finalize")
		// Do not block finalization
		return ctrl.Result{}, nil
	}

	s.reconciler.log.Info().Str("clusterAccess", clusterAccess.GetName()).Str("file", filename).Msg("deleted schema file on finalize")
	return ctrl.Result{}, nil
}

func (s *generateSchemaSubroutine) GetName() string {
	return "generate-schema"
}

func (s *generateSchemaSubroutine) Finalizers() []string {
	return nil
}
