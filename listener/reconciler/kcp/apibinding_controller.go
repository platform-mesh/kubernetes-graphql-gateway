package kcp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmfp/golang-commons/logger"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile"
)

// APIBindingReconciler reconciles an APIBinding object
type APIBindingReconciler struct {
	Client              client.Client
	Scheme              *runtime.Scheme
	RestConfig          *rest.Config
	IOHandler           workspacefile.IOHandler
	DiscoveryFactory    DiscoveryFactory
	APISchemaResolver   apischema.Resolver
	ClusterPathResolver ClusterPathResolver
	Log                 *logger.Logger
}

func (r *APIBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ignore system workspaces (e.g. system:shard)
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := r.Log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()

	clusterClt, err := r.ClusterPathResolver.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get cluster client")
		return ctrl.Result{}, err
	}

	clusterPath, err := PathForCluster(req.ClusterName, clusterClt)
	if err != nil {
		if errors.Is(err, ErrClusterIsDeleted) {
			logger.Info().Msg("cluster is deleted, triggering cleanup")
			if err = r.IOHandler.Delete(clusterPath); err != nil {
				logger.Error().Err(err).Msg("failed to delete workspace file after cluster deletion")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		logger.Error().Err(err).Msg("failed to get cluster path")
		return ctrl.Result{}, err
	}

	logger = logger.With().Str("clusterPath", clusterPath).Logger()
	logger.Info().Msg("starting reconciliation...")

	dc, err := r.DiscoveryFactory.ClientForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create discovery client for cluster")
		return ctrl.Result{}, err
	}

	rm, err := r.DiscoveryFactory.RestMapperForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create rest mapper for cluster")
		return ctrl.Result{}, err
	}

	// Generate current schema
	currentSchema, err := r.generateCurrentSchema(dc, rm, clusterPath)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Read existing schema (if it exists)
	savedSchema, err := r.IOHandler.Read(clusterPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		logger.Error().Err(err).Msg("failed to read existing schema file")
		return ctrl.Result{}, err
	}

	// Write if file doesn't exist or content has changed
	if errors.Is(err, fs.ErrNotExist) || !bytes.Equal(currentSchema, savedSchema) {
		if err := r.IOHandler.Write(currentSchema, clusterPath); err != nil {
			logger.Error().Err(err).Msg("failed to write schema to filesystem")
			return ctrl.Result{}, err
		}
		logger.Info().Msg("schema file updated")
	}

	return ctrl.Result{}, nil
}

// generateCurrentSchema is a subroutine that resolves the current API schema and injects KCP metadata
func (r *APIBindingReconciler) generateCurrentSchema(dc discovery.DiscoveryInterface, rm meta.RESTMapper, clusterPath string) ([]byte, error) {
	// Use shared schema generation logic
	return generateSchemaWithMetadata(
		SchemaGenerationParams{
			ClusterPath:     clusterPath,
			DiscoveryClient: dc,
			RESTMapper:      rm,
			// No HostOverride for regular workspaces - uses environment kubeconfig
		},
		r.APISchemaResolver,
		r.Log,
	)
}
func (r *APIBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcpapis.APIBinding{}).
		Complete(r)
}
