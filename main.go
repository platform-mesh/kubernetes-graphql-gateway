package main

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/platform-mesh/kubernetes-graphql-gateway/cmd/gateway"
	"github.com/platform-mesh/kubernetes-graphql-gateway/cmd/listener"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kubernetes-graphql-gateway",
		Short: "Kubernetes GraphQL Gateway",
	}

	rootCmd.AddCommand(listener.NewCommand())
	rootCmd.AddCommand(gateway.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		klog.Flush()
		os.Exit(1)
	}
	klog.Flush()
}
