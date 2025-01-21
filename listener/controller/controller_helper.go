package controller

import (
	"context"

	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clusterNameFromWorkspace(_ context.Context, o client.Object) []ctrl.Request {
	ws, ok := o.(*kcptenancy.Workspace)
	if !ok || ws.Status.Phase != "Ready" {
		return nil
	}
	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Name: ws.Name,
			},
			ClusterName: ws.Spec.Cluster,
		},
	}
}
