package apischema

import "errors"

var (
	// ErrGetOpenAPIPaths indicates failure to retrieve OpenAPI paths from the API server.
	ErrGetOpenAPIPaths = errors.New("failed to get OpenAPI paths")

	// ErrInvalidGVKFormat indicates the x-kubernetes-group-version-kind extension has an unexpected format.
	ErrInvalidGVKFormat = errors.New("invalid GVK extension format")

	// ErrGetServerPreferred indicates failure to retrieve preferred API resources.
	// Callers may check for this error to handle partial discovery failures.
	ErrGetServerPreferred = errors.New("failed to get server preferred resources")
)
