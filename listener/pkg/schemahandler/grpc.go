package schemahandler

import (
	"context"
	"io/fs"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/broadcaster"
	proto "github.com/platform-mesh/kubernetes-graphql-gateway/sdk"

	"k8s.io/klog/v2"
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
}

func New() *GRPCHandler {
	return &GRPCHandler{
		bus:     broadcaster.New[Event](),
		schemas: sync.Map{},
	}
}

// Delete implements [Handler].
func (g *GRPCHandler) Delete(ctx context.Context, clusterName string) error {
	g.schemas.Delete(clusterName)
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Type:        proto.SubscribeResponse_REMOVED,
	})
	return nil
}

// Read implements [Handler].
func (g *GRPCHandler) Read(ctx context.Context, clusterName string) ([]byte, error) {
	value, ok := g.schemas.Load(clusterName)
	if !ok {
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
	g.schemas.Store(clusterName, schema)
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Schema:      schema,
		Type:        proto.SubscribeResponse_ADDED,
	})
	return nil
}

func (s *GRPCHandler) Subscribe(req *proto.SubscribeRequest, stream proto.SchemaHandler_SubscribeServer) error {
	// Send existing schemas first
	s.schemas.Range(func(key, value any) bool {
		err := stream.Send(&proto.SubscribeResponse{
			ClusterName: key.(string),
			Schema:      string(value.([]byte)),
			EventType:   proto.SubscribeResponse_ADDED,
		})
		if err != nil {
			klog.Error(err, "failed to send existing schema for cluster", "cluster", key.(string))
			return false
		}
		return true
	})

	// Subscribe to updates
	ch := s.bus.Subscribe(stream.Context())
	for update := range ch {
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
