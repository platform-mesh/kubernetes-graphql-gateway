package clusteraccess

// This file exports internal functions for integration testing

// NewGenerateSchemaSubroutineForTesting creates a generateSchemaSubroutine for testing
func NewGenerateSchemaSubroutineForTesting(reconciler *ClusterAccessReconciler) *generateSchemaSubroutine {
	return &generateSchemaSubroutine{reconciler: reconciler}
}
