package main

import (
	"context"
	"fmt"
	"os"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/options"
	"github.com/spf13/pflag"

	genericapiserver "k8s.io/apiserver/pkg/server"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	err := run(genericapiserver.SetupSignalContext())
	klog.Flush()

	if err != nil {
		fmt.Printf("Error running listener backend: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	options := options.NewOptions()
	options.AddFlags(pflag.CommandLine)
	pflag.Parse()

	// setup logging first
	if err := logsv1.ValidateAndApply(options.Logs, nil); err != nil {
		return err
	}

	// Set up controller-runtime logger early to avoid warnings
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.SetLogger(klog.NewKlogr())

	// create server
	completed, err := options.Complete()
	if err != nil {
		return err
	}
	if err := completed.Validate(); err != nil {
		return err
	}

	// start server
	config, err := listener.NewConfig(completed)
	if err != nil {
		return err
	}
	server, err := listener.NewServer(ctx, config)
	if err != nil {
		return err
	}

	if err := server.Run(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}
