package resource

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/controllers/reconciler"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/schemahandler"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

const (
	controllerName = "resource-schema-controller"
)

// Reconciler reconciles the anchor resource to trigger schema generation
type Reconciler struct {
	manager                     mcmanager.Manager
	opts                        controller.TypedOptions[mcreconcile.Request]
	reconciler                  *reconciler.Reconciler
	anchorResource              string
	anchorProgram               cel.Program // compiled anchorResource, evaluated per listed object
	resourceGVK                 schema.GroupVersionKind
	additionalPathAnnotationKey string

	// Provider specific functions
	clusterMetadataFunc    v1alpha1.ClusterMetadataFunc
	clusterURLResolverFunc v1alpha1.ClusterURLResolver
}

// New returns a new ResourceReconciler
func New(
	_ context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	schemaHandler schemahandler.Handler,
	anchorResource string,
	resourceGVR string,
	additionalPathAnnotationKey string,
	clusterMetadataFunc v1alpha1.ClusterMetadataFunc,
	clusterURLResolverFunc v1alpha1.ClusterURLResolver,
) (*Reconciler, error) {
	r := &Reconciler{
		manager:                     mgr,
		opts:                        opts,
		reconciler:                  reconciler.NewReconciler(schemaHandler),
		anchorResource:              anchorResource,
		additionalPathAnnotationKey: additionalPathAnnotationKey,

		clusterMetadataFunc:    clusterMetadataFunc,
		clusterURLResolverFunc: clusterURLResolverFunc,
	}

	gvr, gr := schema.ParseResourceArg(resourceGVR)
	if gvr == nil {
		gvr = &schema.GroupVersionResource{
			Group:    "",
			Version:  gr.Group,
			Resource: gr.Resource,
		}
	}

	var err error
	r.resourceGVK, err = mgr.GetLocalManager().GetRESTMapper().KindFor(*gvr)
	if err != nil {
		return nil, fmt.Errorf("failed to get GVK for GVR %q: %w", gvr.String(), err)
	}

	return r, nil
}

// Reconcile handles the namespace reconciliation
func (r *Reconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling anchor resource", "resourceName", req.Name, "cluster", req.ClusterName)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	// Strip multi-provider prefix (e.g. "kcp#workspace1" → "workspace1") for
	// downstream use in URLs, schema paths, and metadata lookups.
	clusterName := reconciler.ClusterName(req.ClusterName)

	c := cl.GetClient()
	config := rest.CopyConfig(cl.GetConfig())

	config.Host, err = r.clusterURLResolverFunc(config.Host, clusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve cluster URL: %w", err)
	}

	// If we are running in k8s mode, the cluster name might be empty.
	paths := []string{"default"}
	if clusterName != "" {
		paths = []string{clusterName}
	}

	// Decide keep-vs-cleanup by LISTING anchor-GVK resources, not by Getting the
	// single object that triggered this event. The watch fires on every resource
	// of this GVK (e.g. every APIBinding in the workspace), so a sibling's
	// create/delete must not be read as the anchor appearing/disappearing.
	// Regenerate while ≥1 anchor match exists; clean up only when none remain
	// (e.g. the consumer's own anchor binding is actually gone).
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(r.resourceGVK.GroupVersion().WithKind(r.resourceGVK.Kind + "List"))
	if err := c.List(ctx, ul); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list %s: %w", r.resourceGVK.Kind, err)
	}

	var anchor *unstructured.Unstructured
	for i := range ul.Items {
		if r.matchesAnchor(ul.Items[i].Object) {
			anchor = &ul.Items[i]
			break
		}
	}

	if anchor == nil {
		logger.Info("No anchor resource present, cleaning up schema", "anchor", r.anchorResource)
		// Delete the schema file once no anchor-matching resource remains.
		if err := r.reconciler.Cleanup(ctx, paths); err != nil {
			logger.Error(err, "Failed to cleanup schema")
		}
		return ctrl.Result{}, nil
	}

	// Additional-path annotation is read from the matched anchor object.
	if r.additionalPathAnnotationKey != "" && anchor.GetAnnotations() != nil {
		if additionalPath, ok := anchor.GetAnnotations()[r.additionalPathAnnotationKey]; ok {
			logger.V(4).Info("Found additional path annotation on anchor resource", "annotationKey", r.additionalPathAnnotationKey, "additionalPath", additionalPath)
			paths = append(paths, additionalPath)
		}
	}

	// This is plugable function to get cluster metadata for the given cluster name.
	var metadata *v1alpha1.ClusterMetadata
	if r.clusterMetadataFunc != nil {
		var err error
		metadata, err = r.clusterMetadataFunc(clusterName)
		if err != nil {
			logger.Error(err, "Failed to get cluster metadata for namespace reconciliation", "cluster", req.ClusterName)
			return ctrl.Result{}, fmt.Errorf("failed to get cluster metadata for namespace reconciliation: %w", err)
		}
	} else {
		var err error
		metadata, err = v1alpha1.BuildClusterMetadataFromConfig(config)
		if err != nil {
			logger.Error(err, "Failed to build metadata from config")
			return ctrl.Result{}, fmt.Errorf("failed to build metadata from config: %w", err)
		}
	}

	// Generate schema for the cluster
	if err := r.reconciler.Reconcile(ctx, paths, config, metadata); err != nil {
		logger.Error(err, "Failed to reconcile schema")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled schema for cluster")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr mcmanager.Manager, forOpts ...mcbuilder.ForOption) error {
	env, err := cel.NewEnv(
		cel.Variable("object", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, issues := env.Compile(r.anchorResource)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to compile anchor resource CEL expression: %w", issues.Err())
	}

	if ast.OutputType() != cel.BoolType {
		return fmt.Errorf("anchor resource CEL expression must return a boolean, got: %s", ast.OutputType().String())
	}

	prg, err := env.Program(ast,
		cel.EvalOptions(cel.OptOptimize),
	)
	if err != nil {
		return fmt.Errorf("failed to create CEL program for anchor resource: %w", err)
	}
	r.anchorProgram = prg

	us := unstructured.Unstructured{}
	us.SetGroupVersionKind(r.resourceGVK)

	// No anchor predicate here: we watch every resource of this GVK so a sibling
	// (e.g. another APIBinding in the same workspace) create/delete also triggers
	// a rebuild. Whether the anchor itself is present is decided in Reconcile by
	// listing and evaluating the anchor expression — see matchesAnchor.
	return mcbuilder.ControllerManagedBy(mgr).
		For(&us, forOpts...).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}

// matchesAnchor reports whether obj satisfies the compiled anchor CEL expression.
func (r *Reconciler) matchesAnchor(obj map[string]any) bool {
	out, _, err := r.anchorProgram.Eval(map[string]any{"object": obj})
	if err != nil {
		klog.Error("failure evaluating anchor expression", "err", err.Error())
		return false
	}
	b, ok := out.Value().(bool)
	return ok && b
}
