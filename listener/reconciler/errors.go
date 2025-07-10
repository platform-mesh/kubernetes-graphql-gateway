package reconciler

import "errors"

// Common errors used across reconciler packages
var (
	ErrCreateIOHandler  = errors.New("failed to create IO Handler")
	ErrCreateRESTMapper = errors.New("failed to create REST mapper")
	ErrCreateHTTPClient = errors.New("failed to create HTTP client")
	ErrGenerateSchema   = errors.New("failed to generate schema")
	ErrResolveSchema    = errors.New("failed to resolve server JSON schema")
	ErrReadJSON         = errors.New("failed to read JSON from filesystem")
	ErrWriteJSON        = errors.New("failed to write JSON to filesystem")
)
