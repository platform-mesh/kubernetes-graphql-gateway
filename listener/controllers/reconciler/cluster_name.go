package reconciler

import "strings"

// ClusterName strips the multi-provider prefix from a cluster name.
// The multi.Provider from multicluster-runtime prefixes cluster names as
// "providerName#clusterName". This function returns the original cluster name.
func ClusterName(name string) string {
	if _, after, ok := strings.Cut(name, "#"); ok {
		return after
	}
	return name
}
