package targetcluster

import (
	"fmt"
	"strings"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
)

// MatchURL attempts to match the given path against known patterns and extract variables
func MatchURL(path string, appCfg config.Config) (clusterName string, kcpWorkspace string, valid bool) {
	// Try virtual workspace pattern: /virtual-workspace/{virtualWorkspaceName}/{kcpWorkspace}/graphql
	virtualWorkspacePattern := fmt.Sprintf("/%s/{virtualWorkspaceName}/{kcpWorkspace}/%s", appCfg.Url.VirtualWorkspacePrefix, appCfg.Url.GraphqlSuffix)
	if vars := matchPattern(virtualWorkspacePattern, path); vars != nil {
		virtualWorkspaceName := vars["virtualWorkspaceName"]
		kcpWorkspace := vars["kcpWorkspace"]
		if virtualWorkspaceName == "" || kcpWorkspace == "" {
			return "", "", false
		}
		return fmt.Sprintf("%s/%s", appCfg.Url.VirtualWorkspacePrefix, virtualWorkspaceName), kcpWorkspace, true
	}

	// Try regular workspace pattern: /{clusterName}/graphql
	workspacePattern := fmt.Sprintf("/{clusterName}/%s", appCfg.Url.GraphqlSuffix)
	if vars := matchPattern(workspacePattern, path); vars != nil {
		clusterName := vars["clusterName"]
		if clusterName == "" {
			return "", "", false
		}
		return clusterName, "", true
	}

	return "", "", false
}

// matchPattern matches a path against a pattern and extracts variables
func matchPattern(pattern, path string) map[string]string {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return nil
	}

	vars := make(map[string]string)

	for i, patternPart := range patternParts {
		pathPart := pathParts[i]

		if strings.HasPrefix(patternPart, "{") && strings.HasSuffix(patternPart, "}") {
			varName := patternPart[1 : len(patternPart)-1]
			vars[varName] = pathPart
		} else {
			if patternPart != pathPart {
				return nil
			}
		}
	}

	return vars
}
