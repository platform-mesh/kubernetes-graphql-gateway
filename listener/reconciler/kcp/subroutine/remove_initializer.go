package subroutine

import (
	"context"
	"fmt"
	"slices"

	"github.com/platform-mesh/golang-commons/controller/lifecycle/runtimeobject"
	"github.com/platform-mesh/golang-commons/controller/lifecycle/subroutine"
	"github.com/platform-mesh/golang-commons/errors"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	"github.com/kcp-dev/sdk/apis/cache/initialization"
	kcpcorev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
)

type removeInitializer struct {
	initializerName string
	mgr             mcmanager.Manager
}

func NewRemoveInitializer(mgr mcmanager.Manager) *removeInitializer {
	return &removeInitializer{
		initializerName: common.GatewayInitializer,
		mgr:             mgr,
	}
}

var _ subroutine.Subroutine = &removeInitializer{}

func (r *removeInitializer) GetName() string { return "RemoveInitializer" }

func (r *removeInitializer) Finalizers(_ runtimeobject.RuntimeObject) []string { return []string{} }

func (r *removeInitializer) Finalize(_ context.Context, _ runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	return ctrl.Result{}, nil
}

func (r *removeInitializer) Process(ctx context.Context, instance runtimeobject.RuntimeObject) (ctrl.Result, errors.OperatorError) {
	lc := instance.(*kcpcorev1alpha1.LogicalCluster)

	initializer := kcpcorev1alpha1.LogicalClusterInitializer(r.initializerName)

	cluster, err := r.mgr.ClusterFromContext(ctx)
	if err != nil {
		return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("unable to get cluster from context: %w", err), true, false)
	}

	foundInSpec := false
	for i, init := range lc.Spec.Initializers {
		if init == initializer {
			lc.Spec.Initializers = append(lc.Spec.Initializers[:i], lc.Spec.Initializers[i+1:]...)
			foundInSpec = true
			break
		}
	}

	foundInStatus := slices.Contains(lc.Status.Initializers, initializer)

	if !foundInSpec && !foundInStatus {
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(lc.DeepCopy())

	if foundInSpec {
		if err := cluster.GetClient().Patch(ctx, lc, patch); err != nil {
			return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("unable to patch out initializers from spec: %w", err), true, true)
		}
		// Refresh patch for status update if needed
		patch = client.MergeFrom(lc.DeepCopy())
	}

	if foundInStatus {
		lc.Status.Initializers = initialization.EnsureInitializerAbsent(initializer, lc.Status.Initializers)
		if err := cluster.GetClient().Status().Patch(ctx, lc, patch); err != nil {
			return ctrl.Result{}, errors.NewOperatorError(fmt.Errorf("unable to patch out initializers from status: %w", err), true, true)
		}
	}

	return ctrl.Result{}, nil
}
