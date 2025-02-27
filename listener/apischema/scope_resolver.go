package apischema

import (
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	gvkExtensionKey   = "x-kubernetes-group-version-kind"
	scopeExtensionKey = "x-openmfp-scope"
)

type GroupVersionKind struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

func addScopeInfo(schemas map[string]*spec.Schema, rm meta.RESTMapper) (map[string]*spec.Schema, error) {
	for _, schema := range schemas {
		//skip resources that do not have the GVK extension:
		//assumption: sub-resources do not have GVKs
		if schema.VendorExtensible.Extensions == nil {
			continue
		}
		var gvksVal any
		var ok bool
		if gvksVal, ok = schema.VendorExtensible.Extensions[gvkExtensionKey]; !ok {
			continue
		}
		b, err := json.Marshal(gvksVal)
		if err != nil {
			//TODO: debug log?
			continue
		}
		gvks := make([]*GroupVersionKind, 0, 1)
		if err := json.Unmarshal(b, &gvks); err != nil {
			//TODO: debug log?
			continue
		}

		if len(gvks) != 1 {
			//TODO: debug log?
			continue
		}

		gvk := gvks[0]

		namespaced, err := apiutil.IsGVKNamespaced(k8sschema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		}, rm)

		if err != nil {
			//TODO: debug log?
			continue
		}

		if namespaced {
			schema.VendorExtensible.AddExtension(scopeExtensionKey, apiextensionsv1.NamespaceScoped)
		} else {
			schema.VendorExtensible.AddExtension(scopeExtensionKey, apiextensionsv1.ClusterScoped)
		}

	}
	return schemas, nil
}
