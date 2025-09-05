package reconciler

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControllerProvider defines the interface for components that provide controller-runtime managers
// and can set up controllers. This includes both actual reconcilers and manager/coordinator components.
type ControllerProvider interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	SetupWithManager(mgr ctrl.Manager) error
	GetManager() ctrl.Manager
}

// CustomReconciler is an alias for ControllerProvider for backward compatibility
// TODO: Migrate usages to ControllerProvider and remove this alias
type CustomReconciler = ControllerProvider

// ReconcilerOpts contains common options needed by all reconciler strategies
type ReconcilerOpts struct {
	*rest.Config
	*runtime.Scheme
	client.Client
	ManagerOpts            ctrl.Options
	OpenAPIDefinitionsPath string
}
