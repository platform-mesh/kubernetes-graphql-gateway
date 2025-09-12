package main

import (
	"context"
	"fmt"
	"strings"

	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	fmt.Println("=== Testing Hash to Workspace Name Resolution ===")

	// Setup scheme
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kcpcore.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	// Get config
	restCfg := ctrl.GetConfigOrDie()
	ctx := context.Background()

	// Test cases - these are the actual hashes we discovered
	testCases := []string{
		"1rm6k0jwpd10x4cs", // Should resolve to root:platform-mesh-system
		"9t83owosvx5owii5", // Should resolve to root:orgs:default
		"2kxccgn63y6awh8u", // Should resolve to root:orgs
	}

	for _, clusterHash := range testCases {
		fmt.Printf("\n--- Resolving hash: %s ---\n", clusterHash)

		workspaceName, err := resolveClusterHashToWorkspaceName(ctx, clusterHash, restCfg, scheme)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Printf("✅ Success: %s -> %s\n", clusterHash, workspaceName)
		}
	}
}

func resolveClusterHashToWorkspaceName(ctx context.Context, clusterHash string, restCfg *rest.Config, scheme *runtime.Scheme) (string, error) {
	if clusterHash == "root" {
		return "root", nil
	}

	// Create config for the specific cluster hash
	clusterConfig := rest.CopyConfig(restCfg)

	// Replace /clusters/root with /clusters/{clusterHash}
	clusterConfig.Host = strings.Replace(clusterConfig.Host, "/clusters/root", "/clusters/"+clusterHash, 1)

	fmt.Printf("Connecting to: %s\n", clusterConfig.Host)

	// Create client for this cluster
	clt, err := client.New(clusterConfig, client.Options{Scheme: scheme})
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	// Get the LogicalCluster resource
	lc := &kcpcore.LogicalCluster{}
	if err := clt.Get(ctx, client.ObjectKey{Name: "cluster"}, lc); err != nil {
		return "", fmt.Errorf("failed to get LogicalCluster: %w", err)
	}

	// Extract the workspace path from the kcp.io/path annotation
	if lc.Annotations != nil {
		if path, ok := lc.Annotations["kcp.io/path"]; ok {
			return path, nil
		}
	}

	return "", fmt.Errorf("no kcp.io/path annotation found")
}
