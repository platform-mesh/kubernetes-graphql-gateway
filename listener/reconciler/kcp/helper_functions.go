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

// extractAPIExportRef extracts workspace path and export name from an APIExport URL
// Returns (workspacePath, exportName, error)
// Expected format: https://host/services/apiexport/{workspace-path}/{export-name}/
func extractAPIExportRef(hostURL string) (string, string, error) {
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if this is an APIExport URL
	if !strings.HasPrefix(parsedURL.Path, "/services/apiexport/") {
		return "", "", fmt.Errorf("not an APIExport URL: %s", hostURL)
	}

	// Split the path and extract workspace path and export name
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	// Expected: ["services", "apiexport", "{workspace-path}", "{export-name}"]
	if len(pathParts) < 4 || pathParts[0] != "services" || pathParts[1] != "apiexport" {
		return "", "", fmt.Errorf("invalid APIExport URL format, expected /services/apiexport/{workspace-path}/{export-name}: %s", hostURL)
	}

	return pathParts[2], pathParts[3], nil
}
