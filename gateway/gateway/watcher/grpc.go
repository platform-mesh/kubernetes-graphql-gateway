package watcher

import (
	"context"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/registry"
	"github.com/platform-mesh/kubernetes-graphql-gateway/sdk"
	"google.golang.org/grpc"
)

type Watcher any

type grpcWatcher struct {
	schemaHandler   sdk.SchemaHandlerClient
	clusterRegistry *registry.ClusterRegistry
}

func New(clusterRegistry *registry.ClusterRegistry) (*grpcWatcher, error) {
	conn, err := grpc.NewClient("watcher", nil)
	if err != nil {
		return nil, err
	}

	schemaHandler := sdk.NewSchemaHandlerClient(conn)

	return &grpcWatcher{
		schemaHandler:   schemaHandler,
		clusterRegistry: clusterRegistry,
	}, nil
}

func (w *grpcWatcher) Run(ctx context.Context) error {
	stream, err := w.schemaHandler.Subscribe(ctx, &sdk.SubscribeRequest{})
	if err != nil {
		return err
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}

		switch res.EventType {
		case sdk.SubscribeResponse_CREATED:
			// Handle schema added event
		case sdk.SubscribeResponse_UPDATED:
			// Handle schema updated event
		case sdk.SubscribeResponse_REMOVED:
			// Handle schema deleted event
		}
	}

	return nil
}
