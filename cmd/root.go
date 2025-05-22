package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	openmfpconfig "github.com/openmfp/golang-commons/config"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
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
		initConfig()

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
}

func initConfig() {
	// Top-level defaults
	v.SetDefault("openapi-definitions-path", "./bin/definitions")
	v.SetDefault("enable-kcp", true)
	v.SetDefault("local-development", false)
	v.SetDefault("authenticate-schema-requests", false)

	// Listener
	v.SetDefault("listener-apiexport-workspace", ":root")
	v.SetDefault("listener-apiexport-name", "kcp.io")

	// Gateway
	v.SetDefault("gateway-port", "8080")
	v.SetDefault("gateway-username-claim", "email")
	v.SetDefault("gateway-should-impersonate", true)
	// Gateway Handler config
	v.SetDefault("gateway-handler-pretty", true)
	v.SetDefault("gateway-handler-playground", true)
	v.SetDefault("gateway-handler-graphiql", true)
	// Gateway CORS
	v.SetDefault("gateway-cors-enabled", false)
	v.SetDefault("gateway-cors-allowed-origins", []string{"*"})
	v.SetDefault("gateway-cors-allowed-headers", []string{"*"})
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
