package auth

// ExtractCAFromKubeconfigB64ForTest exposes the extractCAFromKubeconfigB64 method for testing
func (m *MetadataInjector) ExtractCAFromKubeconfigB64ForTest(kubeconfigB64 string) []byte {
	return m.extractCAFromKubeconfigB64(kubeconfigB64)
}
