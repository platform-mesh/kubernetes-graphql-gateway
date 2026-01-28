package kcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
)

// InitializingWorkspacesReconciler reconciles LogicalCluster objects in the initializingworkspaces virtual workspace
type InitializingWorkspacesReconciler struct {
	Client              client.Client
	DiscoveryFactory    DiscoveryFactory
	APISchemaResolver   apischema.Resolver
	ClusterPathResolver ClusterPathResolver
	IOHandler           *workspacefile.FileHandler
	Log                 *logger.Logger
}

type ExportedInitializingWorkspacesReconciler = InitializingWorkspacesReconciler

func (r *InitializingWorkspacesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ignore system workspaces
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := r.Log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()
	logger.Info().Msg("reconciling initializing workspace...")

	lc := &kcpcore.LogicalCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, lc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if lc.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	clusterPath, ok := lc.Annotations["kcp.io/path"]
	if !ok {
		logger.Error().Msg("missing kcp.io/path annotation on LogicalCluster")
		return ctrl.Result{}, fmt.Errorf("missing kcp.io/path annotation")
	}

	logger = logger.With().Str("clusterPath", clusterPath).Logger()

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

	currentSchema, err := generateSchemaWithMetadata(
		SchemaGenerationParams{
			ClusterPath:     clusterPath,
			DiscoveryClient: dc,
			RESTMapper:      rm,
		},
		r.APISchemaResolver,
		r.Log,
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate schema")
		return ctrl.Result{}, err
	}

	if err := r.IOHandler.Write(currentSchema, clusterPath); err != nil {
		logger.Error().Err(err).Msg("failed to write schema to filesystem")
		return ctrl.Result{}, err
	}
	logger.Info().Msg("schema file generated and saved")

	parentClusterClt, err := r.ClusterPathResolver.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get parent cluster client")
		return ctrl.Result{}, err
	}

	lcToUpdate := &kcpcore.LogicalCluster{}
	if err := parentClusterClt.Get(ctx, client.ObjectKey{Name: req.Name}, lcToUpdate); err != nil {
		logger.Error().Err(err).Msg("failed to fetch LogicalCluster from parent cluster")
		return ctrl.Result{}, err
	}

	found := false
	for i, initializer := range lcToUpdate.Spec.Initializers {
		if string(initializer) == common.GatewayInitializer {
			lcToUpdate.Spec.Initializers = append(lcToUpdate.Spec.Initializers[:i], lcToUpdate.Spec.Initializers[i+1:]...)
			found = true
			break
		}
	}

	if found {
		logger.Info().Msg("removing gateway initializer from LogicalCluster spec")
		if err := parentClusterClt.Update(ctx, lcToUpdate); err != nil {
			logger.Error().Err(err).Msg("failed to update LogicalCluster in parent cluster")
			return ctrl.Result{}, err
		}
	}

	foundInStatus := false
	for i, initializer := range lcToUpdate.Status.Initializers {
		if string(initializer) == common.GatewayInitializer {
			lcToUpdate.Status.Initializers = append(lcToUpdate.Status.Initializers[:i], lcToUpdate.Status.Initializers[i+1:]...)
			foundInStatus = true
			break
		}
	}

	if foundInStatus {
		logger.Info().Msg("removing gateway initializer from LogicalCluster status")
		if err := parentClusterClt.Status().Update(ctx, lcToUpdate); err != nil {
			logger.Warn().Err(err).Msg("failed to update LogicalCluster status in parent cluster (best effort)")
		}
	}

	return ctrl.Result{}, nil
}

func (r *InitializingWorkspacesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcpcore.LogicalCluster{}).
		Complete(r)
}

// NewInitializingWorkspacesManager creates a manager for the initializingworkspaces virtual workspace
func NewInitializingWorkspacesManager(vwURL string, restConfig *rest.Config, scheme *runtime.Scheme) (ctrl.Manager, error) {
	vwConfig := rest.CopyConfig(restConfig)
	vwConfig.Host = vwURL

	mgr, err := ctrl.NewManager(vwConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		return nil, err
	}

	return mgr, nil
}
