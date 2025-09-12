package main

import (
	"context"
	"fmt"
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
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
)

func main() {
	fmt.Println("=== KCP Workspace Resolution Debug Script ===")

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

	// Test 1: Direct client approach
	fmt.Println("\n=== Test 1: Direct Client Approach ===")
	testDirectClient(ctx, restCfg, scheme)

	// Test 2: Multicluster provider approach
	fmt.Println("\n=== Test 2: Multicluster Provider Approach ===")
	testMulticlusterProvider(ctx, restCfg, scheme)

	// Test 3: List all workspaces
	fmt.Println("\n=== Test 3: List All Workspaces ===")
	testListWorkspaces(ctx, restCfg, scheme)
}

func testDirectClient(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme) {
	fmt.Println("Creating direct client...")

	clt, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Try to list APIBindings
	fmt.Println("Listing APIBindings...")
	apiBindings := &kcpapis.APIBindingList{}
	if err := clt.List(ctx, apiBindings); err != nil {
		fmt.Printf("Error listing APIBindings: %v\n", err)
	} else {
		fmt.Printf("Found %d APIBindings\n", len(apiBindings.Items))
		for i, binding := range apiBindings.Items {
			fmt.Printf("  %d. Name: %s, Namespace: %s\n", i+1, binding.Name, binding.Namespace)

			// Try to get logical cluster info
			logicalClusterName := logicalcluster.From(&binding).String()
			fmt.Printf("      LogicalCluster: %s\n", logicalClusterName)
		}
	}
}

func testMulticlusterProvider(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme) {
	fmt.Println("Creating multicluster provider...")

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

	fmt.Println("Starting provider in background...")

	// Start the provider in a goroutine
	go func() {
		if err := provider.Run(ctx, mcMgr); err != nil {
			fmt.Printf("Provider error: %v\n", err)
		}
	}()

	// Give it some time to discover clusters
	fmt.Println("Waiting for cluster discovery...")

	// Try to get clusters after a delay
	time.Sleep(10 * time.Second)

	// Try to get discovered clusters
	fmt.Println("Attempting to get discovered clusters...")

	// This is tricky - we need to see what clusters were discovered
	// Let's try to use the multicluster manager to see what's available

	// The multicluster manager should have discovered clusters by now
	// But we need to figure out how to access them
	fmt.Println("Multicluster manager created, but we need to figure out how to access discovered clusters")
}

func testListWorkspaces(ctx context.Context, restCfg *rest.Config, scheme *runtime.Scheme) {
	fmt.Println("Creating client to list workspaces...")

	clt, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Try to list Workspace resources
	fmt.Println("Listing Workspace resources...")
	workspaces := &kcptenancy.WorkspaceList{}
	if err := clt.List(ctx, workspaces); err != nil {
		fmt.Printf("Error listing Workspaces: %v\n", err)
	} else {
		fmt.Printf("Found %d Workspaces\n", len(workspaces.Items))
		for i, workspace := range workspaces.Items {
			fmt.Printf("  %d. Name: %s, Namespace: %s\n", i+1, workspace.Name, workspace.Namespace)

			// Get logical cluster info
			logicalClusterName := logicalcluster.From(&workspace).String()
			fmt.Printf("      LogicalCluster: %s\n", logicalClusterName)

			// Check annotations
			if workspace.Annotations != nil {
				for key, value := range workspace.Annotations {
					if key == "kcp.io/cluster" || key == "kcp.io/path" {
						fmt.Printf("      %s: %s\n", key, value)
					}
				}
			}

			// Check status
			fmt.Printf("      Status: %+v\n", workspace.Status)
		}
	}

	// Try to list LogicalCluster resources
	fmt.Println("\nListing LogicalCluster resources...")
	logicalClusters := &kcpcore.LogicalClusterList{}
	if err := clt.List(ctx, logicalClusters); err != nil {
		fmt.Printf("Error listing LogicalClusters: %v\n", err)
	} else {
		fmt.Printf("Found %d LogicalClusters\n", len(logicalClusters.Items))
		for i, lc := range logicalClusters.Items {
			fmt.Printf("  %d. Name: %s\n", i+1, lc.Name)

			// Get logical cluster info
			logicalClusterName := logicalcluster.From(&lc).String()
			fmt.Printf("      LogicalCluster: %s\n", logicalClusterName)

			// Check annotations
			if lc.Annotations != nil {
				for key, value := range lc.Annotations {
					if key == "kcp.io/cluster" || key == "kcp.io/path" {
						fmt.Printf("      %s: %s\n", key, value)
					}
				}
			}
		}
	}
}
