package schemahandler

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/go-logr/logr"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/broadcaster"
	proto "github.com/platform-mesh/kubernetes-graphql-gateway/sdk"
)

type Handler interface {
	Read(ctx context.Context, clusterName string) ([]byte, error)
	Write(ctx context.Context, schema []byte, clusterName string) error
	Delete(ctx context.Context, clusterName string) error
}

type Event struct {
	ClusterName string
	Schema      []byte // nil if Type is SchemaRemoved
	Type        proto.SubscribeResponse_EventType
}

type GRPCHandler struct {
	schemas sync.Map // map[string][]byte
	bus     *broadcaster.Broadcaster[Event]
	proto.UnimplementedSchemaHandlerServer
	log logr.Logger
}

func New(log logr.Logger) *GRPCHandler {
	log = log.WithName("schemahandler").WithValues("handler", "grpc")
	return &GRPCHandler{
		bus:     broadcaster.New[Event](),
		schemas: sync.Map{},
		log:     log,
	}
}

// Delete implements [Handler].
func (g *GRPCHandler) Delete(ctx context.Context, clusterName string) error {
	g.log.V(8).Info("deleting schema for cluster", "cluster", clusterName)
	g.schemas.Delete(clusterName)
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Type:        proto.SubscribeResponse_REMOVED,
	})
	return nil
}

// Read implements [Handler].
func (g *GRPCHandler) Read(ctx context.Context, clusterName string) ([]byte, error) {
	g.log.V(8).Info("reading schema for cluster", "cluster", clusterName)
	value, ok := g.schemas.Load(clusterName)
	if !ok {
		g.log.V(8).Error(fmt.Errorf("schema not found for cluster"), "schema not found", "cluster", clusterName)
		return nil, fs.ErrNotExist
	}
	schema, ok := value.([]byte)
	if !ok {
		return nil, fs.ErrNotExist
	}

	return schema, nil
}

// Write implements [Handler].
func (g *GRPCHandler) Write(ctx context.Context, schema []byte, clusterName string) error {
	g.log.V(8).Info("writing schema for cluster", "cluster", clusterName)
	g.schemas.Store(clusterName, schema)
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Schema:      schema,
		Type:        proto.SubscribeResponse_ADDED,
	})
	return nil
}

func (g *GRPCHandler) Subscribe(req *proto.SubscribeRequest, stream proto.SchemaHandler_SubscribeServer) error {
	g.log.V(8).Info("new schema subscription")
	// Send existing schemas first
	g.schemas.Range(func(key, value any) bool {
		err := stream.Send(&proto.SubscribeResponse{
			ClusterName: key.(string),
			Schema:      string(value.([]byte)),
			EventType:   proto.SubscribeResponse_ADDED,
		})
		if err != nil {
			g.log.Error(err, "failed to send existing schema for cluster", "cluster", key.(string))
			return false
		}
		return true
	})

	// Subscribe to updates
	ch := g.bus.Subscribe(stream.Context())
	for update := range ch {
		g.log.V(8).Info("sending schema update", "cluster", update.ClusterName, "eventType", update.Type.String())
		resp := &proto.SubscribeResponse{
			ClusterName: update.ClusterName,
			Schema:      string(update.Schema),
			EventType:   update.Type,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

var _ Handler = &GRPCHandler{}
