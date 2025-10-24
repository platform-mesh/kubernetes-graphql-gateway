package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	openmfpconfig "github.com/platform-mesh/golang-commons/config"
	"github.com/platform-mesh/golang-commons/logger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/common/config"
)

var (
	appCfg     config.Config
	defaultCfg *openmfpconfig.CommonServiceConfig
	v          *viper.Viper
	log        *logger.Logger
)

var rootCmd = &cobra.Command{
	Use: "listener or gateway",
}

func init() {
	rootCmd.AddCommand(gatewayCmd)
	rootCmd.AddCommand(listenCmd)

	var err error
	v, defaultCfg, err = openmfpconfig.NewDefaultConfig(rootCmd)
	if err != nil {
		panic(err)
	}

	cobra.OnInitialize(func() {
		var err error
		log, err = setupLogger(defaultCfg.Log.Level)
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
	})

	err = openmfpconfig.BindConfigToFlags(v, gatewayCmd, &appCfg)
	if err != nil {
		panic(err)
	}

	err = openmfpconfig.BindConfigToFlags(v, listenCmd, &appCfg)
	if err != nil {
		panic(err)
	}
}

// setupLogger initializes the logger with the given log level
func setupLogger(logLevel string) (*logger.Logger, error) {
	loggerCfg := logger.DefaultConfig()
	loggerCfg.Name = "crdGateway"
	loggerCfg.Level = logLevel
	return logger.New(loggerCfg)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
