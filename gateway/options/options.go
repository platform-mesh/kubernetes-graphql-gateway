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

	fs.StringVar(&options.SchemasDir, "schemas-dir", options.SchemasDir, "directory to store schema files")
	fs.IntVar(&options.ServerBindPort, "gateway-port", options.ServerBindPort, "port for the GraphQL gateway server")
	fs.StringVar(&options.ServerBindAddress, "gateway-address", options.ServerBindAddress, "address for the GraphQL gateway server")
	fs.BoolVar(&options.PlaygroundEnabled, "enable-playground", options.PlaygroundEnabled, "enable the GraphQL playground")
	fs.StringSliceVar(&options.CORSAllowedOrigins, "cors-allowed-origins", options.CORSAllowedOrigins, "list of allowed origins for CORS")
	fs.StringSliceVar(&options.CORSAllowedHeaders, "cors-allowed-headers", options.CORSAllowedHeaders, "list of allowed headers for CORS")
	fs.StringVar(&options.URLSuffix, "url-suffix", options.URLSuffix, "URL suffix for the GraphQL endpoint")

	fs.BoolVar(&options.DevelopmentDisableAuth, "development-disable-auth", options.DevelopmentDisableAuth, "disable authentication in development mode")
	fs.MarkHidden("development-disable-auth") //nolint:errcheck

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
