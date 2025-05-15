package config

type Config struct {
	OpenApiDefinitionsPath string `mapstructure:"openapi-definitions-path"`
	EnableKcp              bool   `mapstructure:"enable-kcp"`
	LocalDevelopment       bool   `mapstructure:"local-development"`

	Listener struct {
		// Listener fields will be added here
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
			Enabled        bool     `mapstructure:"gateway-cors-enabled"`
			AllowedOrigins []string `mapstructure:"gateway-cors-allowed-origins"`
			AllowedHeaders []string `mapstructure:"gateway-cors-allowed-headers"`
		} `mapstructure:",squash"`
	} `mapstructure:",squash"`
}
