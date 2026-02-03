package gateway

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/options"

	genericapiserver "k8s.io/apiserver/pkg/server"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type command struct {
	options *options.Options
}

func NewCommand() *cobra.Command {
	c := &command{
		options: options.NewOptions(),
	}

	cmd := &cobra.Command{
		Use:   "gateway",
		Short: "Run the gateway server",
		RunE:  c.run,
	}

	c.options.AddFlags(cmd.Flags())
	return cmd
}

func (c *command) run(cmd *cobra.Command, args []string) error {
	if err := logsv1.ValidateAndApply(c.options.Logs, nil); err != nil {
		return err
	}
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log.SetLogger(klog.NewKlogr())

	completed, err := c.options.Complete()
	if err != nil {
		return err
	}
	if err := completed.Validate(); err != nil {
		return err
	}

	config, err := gateway.NewConfig(completed)
	if err != nil {
		return err
	}
	server, err := gateway.NewServer(config)
	if err != nil {
		return err
	}

	ctx := genericapiserver.SetupSignalContext()
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("error running gateway: %w", err)
	}

	<-ctx.Done()
	return nil
}
