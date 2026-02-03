/*
Copyright 2025 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/spf13/pflag"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Options struct {
	ExtraOptions
}

type ExtraOptions struct {
	// APIExportEndpointSliceName is the name of the APIExport EndpointSlice to watch.
	APIExportEndpointSliceName string
	// WorkspaceSchemaHostOverride is the host override for workspace schema generation.
	WorkspaceSchemaHostOverride string
	// workspaceSchemaKubeconfigOverride is the kubeconfig override for workspace schema generation.
	// If set together with WorkspaceSchemaHostOverride, WorkspaceSchemaHostOverride will take precedence.
	workspaceSchemaKubeconfigOverride string
	// WorkspaceSchemaKubeconfigRestConfig is the rest config built from workspaceSchemaKubeconfigOverride
	WorkspaceSchemaKubeconfigRestConfig *rest.Config
}

type completedOptions struct {
	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	return &Options{
		ExtraOptions: ExtraOptions{
			APIExportEndpointSliceName: "graphql-gateway-apiexports",
		},
	}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.APIExportEndpointSliceName, "apiexport-endpoint-slice-name", options.APIExportEndpointSliceName, "name of the APIExport EndpointSlice to watch")
	fs.StringVar(&options.WorkspaceSchemaHostOverride, "workspace-schema-host-override", options.WorkspaceSchemaHostOverride, "host override for workspace schema generation")
	fs.StringVar(&options.workspaceSchemaKubeconfigOverride, "workspace-schema-kubeconfig-override", options.workspaceSchemaKubeconfigOverride, "kubeconfig override for workspace schema generation. If set together with --workspace-schema-host-override, the host override will take precedence.")
}

func (options *Options) Complete() (*CompletedOptions, error) {
	if options.workspaceSchemaKubeconfigOverride != "" {
		// Load the kubeconfig and build rest config
		config, err := clientcmd.BuildConfigFromFlags("", options.workspaceSchemaKubeconfigOverride)
		if err != nil {
			return nil, fmt.Errorf("failed to build rest config from kubeconfig: %w", err)
		}

		options.WorkspaceSchemaKubeconfigRestConfig = config
	}

	return &CompletedOptions{
		completedOptions: &completedOptions{
			ExtraOptions: options.ExtraOptions,
		},
	}, nil
}

func (options *CompletedOptions) Validate() error {
	if options.workspaceSchemaKubeconfigOverride != "" {
		// Check if kubeconfig file exists
		if _, err := os.Stat(options.workspaceSchemaKubeconfigOverride); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("kubeconfig file does not exist: %s", options.workspaceSchemaKubeconfigOverride)
			}
			return fmt.Errorf("failed to access kubeconfig file: %w", err)
		}
	}

	return nil
}

func (options *CompletedOptions) GetClusterMetadataOverrideFunc() v1alpha1.ClusterMetadataFunc {
	return func(clusterName string) (*v1alpha1.ClusterMetadata, error) {
		metadata := &v1alpha1.ClusterMetadata{}
		if options.WorkspaceSchemaKubeconfigRestConfig != nil {
			// TODO: Convert rest.Config to ClusterMetadata
			// For now, we just return a minimum

			parsed, err := url.Parse(options.WorkspaceSchemaKubeconfigRestConfig.Host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse host from rest config: %w", err)
			}

			parsed.Path = path.Join("clusters", clusterName)

			metadata.Host = parsed.String()

			metadata.CA = &v1alpha1.CAMetadata{
				Data: base64.StdEncoding.EncodeToString(options.WorkspaceSchemaKubeconfigRestConfig.CAData),
			}
			/// REMOVE BEFORE COMMITTING
			metadata.Auth = &v1alpha1.AuthMetadata{
				CertData: base64.StdEncoding.EncodeToString(options.WorkspaceSchemaKubeconfigRestConfig.CertData),
				KeyData:  base64.StdEncoding.EncodeToString(options.WorkspaceSchemaKubeconfigRestConfig.KeyData),
			}
			metadata.Auth.Type = v1alpha1.AuthTypeClientCert
			return metadata, nil
		}
		if options.WorkspaceSchemaHostOverride != "" {
			metadata.Host = options.WorkspaceSchemaHostOverride
		}
		return metadata, nil
	}
}

func (options *CompletedOptions) GetClusterURLResolverFunc() v1alpha1.ClusterURLResolver {
	return func(currentURL string, clusterName string) (string, error) {
		if options.WorkspaceSchemaHostOverride != "" {
			return options.WorkspaceSchemaHostOverride, nil
		}
		parts := strings.Split(currentURL, "/services/")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid current URL format: %s", currentURL)
		}
		newURL := fmt.Sprintf("%s/clusters/%s", parts[0], clusterName)
		return newURL, nil
	}
}
