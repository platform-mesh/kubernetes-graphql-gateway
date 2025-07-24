package config

type Config struct {
	OpenApiDefinitionsPath      string `mapstructure:"openapi-definitions-path"`
	EnableKcp                   bool   `mapstructure:"enable-kcp"`
	LocalDevelopment            bool   `mapstructure:"local-development"`
	IntrospectionAuthentication bool   `mapstructure:"introspection-authentication"`

	Url struct {
		VirtualWorkspacePrefix string `mapstructure:"gateway-url-virtual-workspace-prefix"`
		DefaultKcpWorkspace    string `mapstructure:"gateway-url-default-kcp-workspace"`
		GraphqlSuffix          string `mapstructure:"gateway-url-graphql-suffix"`
	} `mapstructure:",squash"`

	Listener struct {
		VirtualWorkspacesConfigPath string `mapstructure:"virtual-workspaces-config-path"`
	} `mapstructure:",squash"`

	Gateway struct {
		Port              string `mapstructure:"gateway-port"`
		UsernameClaim     string `mapstructure:"gateway-username-claim"`
		ShouldImpersonate bool   `mapstructure:"gateway-should-impersonate"`

		HandlerCfg struct {
			Pretty     bool `mapstructure:"gateway-handler-pretty"`
			Playground bool `mapstructure:"gateway-handler-playground"`
			GraphiQL   bool `mapstructure:"gateway-handler-graphiql"`
		} `mapstructure:",squash"`

		Cors struct {
			Enabled        bool   `mapstructure:"gateway-cors-enabled"`
			AllowedOrigins string `mapstructure:"gateway-cors-allowed-origins"`
			AllowedHeaders string `mapstructure:"gateway-cors-allowed-headers"`
		} `mapstructure:",squash"`
	} `mapstructure:",squash"`
}
