package reconciler

import (
	"strings"

	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
)

// ClusterName strips the multi-provider prefix from a cluster name.
// The multi.Provider from multicluster-runtime prefixes cluster names as
// "providerName#clusterName". This function returns the original cluster name.
func ClusterName(name multicluster.ClusterName) string {
	if _, after, ok := strings.Cut(string(name), "#"); ok {
		return after
	}
	return string(name)
}
