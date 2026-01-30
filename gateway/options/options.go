package options

import (
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
)

type Options struct {
	Logs *logs.Options

	ExtraOptions
}

type ExtraOptions struct {
	// SchemasDir is the directory to store schema files.
	SchemasDir string
	// ServerBindAddress is the address for the GraphQL gateway server.
	ServerBindAddress string
	// ServerBindPort is the port for the GraphQL gateway server.
	ServerBindPort int
	// PlaygroundEnabled indicates whether to enable the GraphQL playground.
	PlaygroundEnabled bool
	// CORSAllowedOrigins is the list of allowed origins for CORS.
	CORSAllowedOrigins []string
	// CORSAllowedHeaders is the list of allowed headers for CORS.
	CORSAllowedHeaders []string
	// URLSuffix is the URL suffix for the GraphQL endpoint.
	// For example, if set to "/graphql", the endpoint will be available at /graphql.
	URLSuffix string
	// DevelopmentDisableAuth indicates whether to disable authentication in development mode.
	DevelopmentDisableAuth bool

	GraphQLPretty     bool
	GraphQLPlayground bool
	GraphQLGraphiQL   bool
}

type completedOptions struct {
	Logs *logs.Options

	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	// Default to -v=2
	logs := logs.NewOptions()
	logs.Verbosity = logsv1.VerbosityLevel(2)

	opts := &Options{
		Logs: logs,

		ExtraOptions: ExtraOptions{
			SchemasDir:         "_output/schemas",
			ServerBindAddress:  "0.0.0.0",
			ServerBindPort:     8080,
			PlaygroundEnabled:  false,
			CORSAllowedOrigins: []string{},
			CORSAllowedHeaders: []string{},
			URLSuffix:          "/graphql",

			DevelopmentDisableAuth: false,
		},
	}
	return opts
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)

	fs.StringVar(&options.ExtraOptions.SchemasDir, "schemas-dir", options.ExtraOptions.SchemasDir, "directory to store schema files")
	fs.IntVar(&options.ExtraOptions.ServerBindPort, "gateway-port", options.ExtraOptions.ServerBindPort, "port for the GraphQL gateway server")
	fs.StringVar(&options.ExtraOptions.ServerBindAddress, "gateway-address", options.ExtraOptions.ServerBindAddress, "address for the GraphQL gateway server")
	fs.BoolVar(&options.ExtraOptions.PlaygroundEnabled, "enable-playground", options.ExtraOptions.PlaygroundEnabled, "enable the GraphQL playground")
	fs.StringSliceVar(&options.ExtraOptions.CORSAllowedOrigins, "cors-allowed-origins", options.ExtraOptions.CORSAllowedOrigins, "list of allowed origins for CORS")
	fs.StringSliceVar(&options.ExtraOptions.CORSAllowedHeaders, "cors-allowed-headers", options.ExtraOptions.CORSAllowedHeaders, "list of allowed headers for CORS")
	fs.StringVar(&options.ExtraOptions.URLSuffix, "url-suffix", options.ExtraOptions.URLSuffix, "URL suffix for the GraphQL endpoint")

	fs.BoolVar(&options.ExtraOptions.DevelopmentDisableAuth, "development-disable-auth", options.ExtraOptions.DevelopmentDisableAuth, "disable authentication in development mode")
	fs.MarkHidden("development-disable-auth")

}

func (options *Options) Complete() (*CompletedOptions, error) {
	co := &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			ExtraOptions: options.ExtraOptions,
		},
	}

	return co, nil
}

func (options *CompletedOptions) Validate() error {
	return nil
}
