package resolver

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func GetStringArg(args map[string]any, key string, required bool) (string, error) {
	return getStringArg(args, key, required)
}

func GetBoolArg(args map[string]any, key string, required bool) (bool, error) {
	return getBoolArg(args, key, required)
}

func CompareUnstructured(a, b unstructured.Unstructured, fieldPath string) int {
	return compareUnstructured(a, b, fieldPath)
}
