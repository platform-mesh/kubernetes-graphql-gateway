package kcp

import (
	"context"
	"strings"

	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
)

// LogicalClusterReconciler reconciles a LogicalCluster object
type LogicalClusterReconciler struct {
	Client              client.Client
	ClusterPathResolver ClusterPathResolver
	Log                 *logger.Logger
}

func (r *LogicalClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := r.Log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()

	clusterClt, err := r.ClusterPathResolver.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get cluster client")
		return ctrl.Result{}, err
	}

	lc := &kcpcore.LogicalCluster{}
	if err := clusterClt.Get(ctx, client.ObjectKey{Name: req.Name}, lc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if lc.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	workspace := &kcptenancy.Workspace{}
	if err := clusterClt.Get(ctx, client.ObjectKey{Name: req.Name}, workspace); err != nil {
		logger.Error().Err(err).Msg("failed to get workspace resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if workspace.Spec.Type.Name != "account" {
		return ctrl.Result{}, nil
	}

	found := false
	for i, initializer := range lc.Spec.Initializers {
		if string(initializer) == common.GatewayInitializer {
			lc.Spec.Initializers = append(lc.Spec.Initializers[:i], lc.Spec.Initializers[i+1:]...)
			found = true
			break
		}
	}

	if found {
		logger.Info().Msg("removing gateway initializer from LogicalCluster")
		if err := clusterClt.Update(ctx, lc); err != nil {
			logger.Error().Err(err).Msg("failed to update LogicalCluster")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *LogicalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcpcore.LogicalCluster{}).
		Complete(r)
}
