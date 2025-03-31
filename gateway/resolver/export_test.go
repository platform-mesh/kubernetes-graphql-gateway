package resolver

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func (r *Service) GetOriginalGroupName(key string) string {
	return r.getOriginalGroupName(key)
}

func (r *Service) GetGroupName(key string) string {
	return r.groupNames[key]
}

func (r *Service) SetGroupNames(names map[string]string) {
	r.groupNames = names
}

func GetStringArg(args map[string]interface{}, key string, required bool) (string, error) {
	return getStringArg(args, key, required)
}

func GetBoolArg(args map[string]interface{}, key string, required bool) (bool, error) {
	return getBoolArg(args, key, required)
}

func CompareUnstructured(a, b unstructured.Unstructured, fieldPath string) int {
	return compareUnstructured(a, b, fieldPath)
}
