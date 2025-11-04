package reconciler

import "errors"

// Common errors used across reconciler packages
var (
	ErrCreateRESTMapper = errors.New("failed to create REST mapper")
	ErrCreateHTTPClient = errors.New("failed to create HTTP client")
)
