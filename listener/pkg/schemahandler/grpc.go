package schemahandler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/broadcaster"
	proto "github.com/platform-mesh/kubernetes-graphql-gateway/sdk"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrNotExist = errors.New("schema does not exist")
)

type Handler interface {
	Read(ctx context.Context, clusterName string) ([]byte, error)
	Write(ctx context.Context, schema []byte, clusterName string) error
	Delete(ctx context.Context, clusterName string) error
}

type Event struct {
	ClusterName string
	Schema      string // nil if Type is SchemaRemoved
	Type        proto.SubscribeResponse_EventType
}

type GRPCHandler struct {
	schemas sync.Map // map[string]string
	bus     *broadcaster.Broadcaster[Event]
	proto.UnimplementedSchemaHandlerServer
}

func NewGRPCHandler() *GRPCHandler {
	return &GRPCHandler{
		bus:     broadcaster.New[Event](),
		schemas: sync.Map{},
	}
}

// Delete implements [Handler].
func (g *GRPCHandler) Delete(ctx context.Context, clusterName string) error {
	log := log.FromContext(ctx)
	log.V(8).Info("deleting schema for cluster", "cluster", clusterName)
	g.schemas.Delete(clusterName)
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Type:        proto.SubscribeResponse_REMOVED,
	})
	return nil
}

// Read implements [Handler].
func (g *GRPCHandler) Read(ctx context.Context, clusterName string) ([]byte, error) {
	log := log.FromContext(ctx)
	log.V(8).Info("reading schema for cluster", "cluster", clusterName)

	value, ok := g.schemas.Load(clusterName)
	if !ok {
		log.V(8).Error(fmt.Errorf("schema not found for cluster"), "schema not found", "cluster", clusterName)
		return nil, ErrNotExist
	}

	schema, ok := value.(string)
	if !ok {
		return nil, ErrNotExist
	}

	return []byte(schema), nil
}

// Write implements [Handler].
func (g *GRPCHandler) Write(ctx context.Context, schema []byte, clusterName string) error {
	log := log.FromContext(ctx)
	log.V(8).Info("writing schema for cluster", "cluster", clusterName)

	_, loaded := g.schemas.Swap(clusterName, string(schema))

	eventType := proto.SubscribeResponse_CREATED
	if loaded {
		eventType = proto.SubscribeResponse_UPDATED
	}
	g.bus.Publish(ctx, Event{
		ClusterName: clusterName,
		Schema:      string(schema),
		Type:        eventType,
	})
	return nil
}

func (g *GRPCHandler) Subscribe(req *proto.SubscribeRequest, stream proto.SchemaHandler_SubscribeServer) error {
	log := log.FromContext(stream.Context())
	log.V(8).Info("new schema subscription")
	// Send existing schemas first
	g.schemas.Range(func(key, value any) bool {
		err := stream.Send(&proto.SubscribeResponse{
			ClusterName: key.(string),
			Schema:      value.(string),
			EventType:   proto.SubscribeResponse_CREATED,
		})
		if err != nil {
			log.Error(err, "failed to send existing schema for cluster", "cluster", key.(string))
			return false
		}
		return true
	})

	// Subscribe to updates
	ch := g.bus.Subscribe(stream.Context())
	for update := range ch {
		log.V(8).Info("sending schema update", "cluster", update.ClusterName, "eventType", update.Type.String())
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
