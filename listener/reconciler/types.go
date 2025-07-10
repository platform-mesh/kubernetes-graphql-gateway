package reconciler

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CustomReconciler defines the interface that all reconcilers must implement
type CustomReconciler interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	SetupWithManager(mgr ctrl.Manager) error
	GetManager() ctrl.Manager
}

// ReconcilerOpts contains common options needed by all reconciler strategies
type ReconcilerOpts struct {
	*rest.Config
	*runtime.Scheme
	client.Client
	ManagerOpts            ctrl.Options
	OpenAPIDefinitionsPath string
}
