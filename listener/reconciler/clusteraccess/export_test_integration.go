package clusteraccess

// Integration testing exports for cross-package access
// Unit tests within this package should use export_test.go instead

// ClusterAccessReconcilerPublic exposes the reconciler for integration testing
type ClusterAccessReconcilerPublic = ClusterAccessReconciler

// GenerateSchemaSubroutinePublic exposes the subroutine for integration testing
type GenerateSchemaSubroutinePublic = generateSchemaSubroutine

// NewGenerateSchemaSubroutineForTesting creates a new subroutine for integration testing
func NewGenerateSchemaSubroutineForTesting(reconciler *ClusterAccessReconciler) *GenerateSchemaSubroutinePublic {
	return &generateSchemaSubroutine{reconciler: reconciler}
}
