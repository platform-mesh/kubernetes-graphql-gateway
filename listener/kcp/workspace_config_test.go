package kcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombineBaseURLAndPath(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		pathURL  string
		expected string
		err      error
	}{
		{
			name:     "success",
			baseURL:  "https://openmfp-kcp-front-proxy.openmfp-system:8443/clusters/root",
			pathURL:  "https://kcp.dev.local:8443/services/apiexport/root/kubernetes.graphql.gateway",
			expected: "https://openmfp-kcp-front-proxy.openmfp-system:8443/services/apiexport/root/kubernetes.graphql.gateway",
		},
		{
			name:     "success_base_with_port",
			baseURL:  "https://example.com:8080",
			pathURL:  "/api/resource",
			expected: "https://example.com:8080/api/resource",
		},
		{
			name:     "success_base_with_subpath_relative_path",
			baseURL:  "https://example.com/base",
			pathURL:  "api/resource",
			expected: "https://example.com/api/resource",
		},
		{
			name:     "success_base_with_subpath_absolute_path",
			baseURL:  "https://example.com/base",
			pathURL:  "/api/resource",
			expected: "https://example.com/api/resource",
		},
		{
			name:     "success_empty_path_url",
			baseURL:  "https://example.com",
			pathURL:  "",
			expected: "https://example.com/",
		},
		{
			name:    "error_invalid_base_url",
			baseURL: "ht@tp://bad_url",
			pathURL: "/api/resource",
			err:     errors.Join(ErrInvalidURL, errors.New("parse \"ht@tp://bad_url\": first path segment in URL cannot contain colon")),
		},
		{
			name:    "error_invalid_path_url",
			baseURL: "https://example.com",
			pathURL: "ht@tp://bad_url",
			err:     errors.Join(ErrInvalidURL, errors.New("parse \"ht@tp://bad_url\": first path segment in URL cannot contain colon")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := combineBaseURLAndPath(tt.baseURL, tt.pathURL)

			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
