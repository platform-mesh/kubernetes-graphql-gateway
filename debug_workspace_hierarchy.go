package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/kcp-dev/multicluster-provider/apiexport"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

func main() {
	fmt.Println("=== KCP Workspace Hierarchy Debug Script ===")

	// Setup scheme
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kcpapis.AddToScheme(scheme))
	utilruntime.Must(kcpcore.AddToScheme(scheme))
	utilruntime.Must(kcptenancy.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	// Get config
	restCfg := ctrl.GetConfigOrDie()
	fmt.Printf("KCP Host: %s\n", restCfg.Host)

	ctx := context.Background()

	// Test exploring workspace hierarchy
	fmt.Println("\n=== Exploring Workspace Hierarchy ===")
	exploreWorkspaceHierarchy(ctx, restCfg, scheme)

	// Test multicluster provider with actual reconciler
	fmt.Println("\n=== Testing Multicluster Provider with Reconciler ===")
	testMulticlusterProviderWithReconciler(ctx, restCfg, scheme)
}

func exploreWorkspaceHierarchy(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme) {
	// Try to access different workspaces directly
	workspaces := []string{
		"root",
		"root:orgs",
		"root:orgs:default",
		"root:orgs:default:1",
		"root:platform-mesh-system",
	}

	for _, workspace := range workspaces {
		fmt.Printf("\n--- Testing workspace: %s ---\n", workspace)

		// Create config for this specific workspace
		workspaceConfig := rest.CopyConfig(restCfg)

		// Modify the URL to point to the specific workspace
		if workspace != "root" {
			// Replace /clusters/root with /clusters/{workspace}
			workspaceConfig.Host = strings.Replace(workspaceConfig.Host, "/clusters/root", "/clusters/"+workspace, 1)
		}

		fmt.Printf("Workspace URL: %s\n", workspaceConfig.Host)

		// Try to create a client for this workspace
		clt, err := client.New(workspaceConfig, client.Options{Scheme: scheme})
		if err != nil {
			fmt.Printf("Error creating client for workspace %s: %v\n", workspace, err)
			continue
		}

		// Try to list APIBindings in this workspace
		apiBindings := &kcpapis.APIBindingList{}
		if err := clt.List(ctx, apiBindings); err != nil {
			fmt.Printf("Error listing APIBindings in workspace %s: %v\n", workspace, err)
		} else {
			fmt.Printf("Found %d APIBindings in workspace %s\n", len(apiBindings.Items), workspace)
			for _, binding := range apiBindings.Items {
				logicalClusterName := logicalcluster.From(&binding).String()
				fmt.Printf("  - %s (LogicalCluster: %s)\n", binding.Name, logicalClusterName)
			}
		}

		// Try to get the LogicalCluster resource
		lc := &kcpcore.LogicalCluster{}
		if err := clt.Get(ctx, client.ObjectKey{Name: "cluster"}, lc); err != nil {
			fmt.Printf("Error getting LogicalCluster in workspace %s: %v\n", workspace, err)
		} else {
			logicalClusterName := logicalcluster.From(lc).String()
			fmt.Printf("LogicalCluster name: %s\n", logicalClusterName)

			// Check annotations
			if lc.Annotations != nil {
				if path, ok := lc.Annotations["kcp.io/path"]; ok {
					fmt.Printf("kcp.io/path annotation: %s\n", path)
				}
				if cluster, ok := lc.Annotations["kcp.io/cluster"]; ok {
					fmt.Printf("kcp.io/cluster annotation: %s\n", cluster)
				}
			}
		}
	}
}

func testMulticlusterProviderWithReconciler(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme) {
	fmt.Println("Setting up multicluster provider with reconciler...")

	// Create the apiexport provider
	provider, err := apiexport.New(restCfg, apiexport.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		return
	}

	// Create multicluster manager
	mcMgr, err := mcmanager.New(restCfg, provider, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		fmt.Printf("Error creating multicluster manager: %v\n", err)
		return
	}

	// Create a simple reconciler to see what clusters are discovered
	reconciler := &TestReconciler{}

	// Setup the multicluster APIBinding controller
	err = mcbuilder.ControllerManagedBy(mcMgr).
		Named("test-apibinding-controller").
		For(&kcpapis.APIBinding{}).
		Complete(mcreconcile.Func(reconciler.Reconcile))
	if err != nil {
		fmt.Printf("Error setting up controller: %v\n", err)
		return
	}

	fmt.Println("Starting provider and waiting for discoveries...")

	// Start the provider in a goroutine
	go func() {
		if err := provider.Run(ctx, mcMgr); err != nil {
			fmt.Printf("Provider error: %v\n", err)
		}
	}()

	// Wait for some discoveries
	time.Sleep(15 * time.Second)

	fmt.Println("Finished waiting for discoveries")
}

type TestReconciler struct{}

func (r *TestReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	fmt.Printf("🎯 DISCOVERED CLUSTER: %s (Name: %s, Namespace: %s)\n",
		req.ClusterName, req.Name, req.Namespace)

	// This is the key - req.ClusterName should contain the cluster hash!
	// Let's see what we get here

	return ctrl.Result{}, nil
}
