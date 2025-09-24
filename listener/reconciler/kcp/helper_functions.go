package kcp

import (
	"fmt"
	"net/url"
	"strings"
)

// stripAPIExportPath removes APIExport virtual workspace paths from a URL to get the base KCP host
func stripAPIExportPath(hostURL string) string {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		// If we can't parse the URL, return it as-is
		return hostURL
	}

	// Check if the path contains an APIExport pattern: /services/apiexport/...
	if strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		// Strip the APIExport path to get the base KCP host
		parsedURL.Path = ""
		return parsedURL.String()
	}

	// If it's not an APIExport URL, return as-is
	return hostURL
}

// APIExportInfo contains the parsed components of an APIExport URL
type APIExportInfo struct {
	WorkspacePath string
	ExportName    string
}

// extractAPIExportInfo extracts workspace path and export name from an APIExport URL
// Expected format: https://host/services/apiexport/{workspace-path}/{export-name}/
func extractAPIExportInfo(hostURL string) (*APIExportInfo, error) {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if this is an APIExport URL
	if !strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		return nil, fmt.Errorf("not an APIExport URL: %s", hostURL)
	}

	// Split the path and extract workspace path and export name
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	// Expected: ["services", "apiexport", "{workspace-path}", "{export-name}"]
	if len(pathParts) < 4 || pathParts[0] != "services" || pathParts[1] != "apiexport" {
		return nil, fmt.Errorf("invalid APIExport URL format, expected /services/apiexport/{workspace-path}/{export-name}: %s", hostURL)
	}

	return &APIExportInfo{
		WorkspacePath: pathParts[2],
		ExportName:    pathParts[3],
	}, nil
}

// extractClusterHashFromAPIExportURL extracts the cluster hash from an APIExport URL
// DEPRECATED: Use extractAPIExportInfo instead for proper workspace path and export name parsing
// Expected format: https://host/services/apiexport/{cluster-hash}/{apiexport-name}/
func extractClusterHashFromAPIExportURL(hostURL string) string {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		return ""
	}

	// Check if this is an APIExport URL
	if !strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		return ""
	}

	// Split the path and extract the cluster hash
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	// Expected: ["services", "apiexport", "{cluster-hash}", "{apiexport-name}"]
	if len(pathParts) >= 3 && pathParts[0] == "services" && pathParts[1] == "apiexport" {
		return pathParts[2] // This is the cluster hash
	}

	return ""
}
