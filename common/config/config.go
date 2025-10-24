package config

type Config struct {
	OpenApiDefinitionsPath string `mapstructure:"openapi-definitions-path" default:"./bin/definitions"`
	EnableKcp              bool   `mapstructure:"enable-kcp" default:"true"`
	LocalDevelopment       bool   `mapstructure:"local-development" default:"false"`

	Url struct {
		VirtualWorkspacePrefix string `mapstructure:"gateway-url-virtual-workspace-prefix" default:"virtual-workspace"`
		DefaultKcpWorkspace    string `mapstructure:"gateway-url-default-kcp-workspace" default:"root"`
		GraphqlSuffix          string `mapstructure:"gateway-url-graphql-suffix" default:"graphql"`
	} `mapstructure:",squash"`

	Listener struct {
		VirtualWorkspacesConfigPath string `mapstructure:"virtual-workspaces-config-path"`
	} `mapstructure:",squash"`

	Gateway struct {
		Port                        string `mapstructure:"gateway-port" default:"8080"`
		UsernameClaim               string `mapstructure:"gateway-username-claim" default:"email"`
		ShouldImpersonate           bool   `mapstructure:"gateway-should-impersonate" default:"true"`
		IntrospectionAuthentication bool   `mapstructure:"gateway-introspection-authentication" default:"false"`

		HandlerCfg struct {
			Pretty     bool `mapstructure:"gateway-handler-pretty" default:"true"`
			Playground bool `mapstructure:"gateway-handler-playground" default:"true"`
			GraphiQL   bool `mapstructure:"gateway-handler-graphiql" default:"true"`
		} `mapstructure:",squash"`

		Cors struct {
			Enabled        bool   `mapstructure:"gateway-cors-enabled" default:"false"`
			AllowedOrigins string `mapstructure:"gateway-cors-allowed-origins" default:"*"`
			AllowedHeaders string `mapstructure:"gateway-cors-allowed-headers" default:"*"`
		} `mapstructure:",squash"`
	} `mapstructure:",squash"`
}
