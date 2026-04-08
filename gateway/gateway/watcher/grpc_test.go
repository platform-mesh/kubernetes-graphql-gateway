package watcher_test

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/watcher"
	proto "github.com/platform-mesh/kubernetes-graphql-gateway/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeHandler records schema events from the watcher.
type fakeHandler struct {
	mu       sync.Mutex
	changed  map[string][]byte
	deleted  []string
	changeCh chan string
}

func newFakeHandler() *fakeHandler {
	return &fakeHandler{
		changed:  make(map[string][]byte),
		changeCh: make(chan string, 10),
	}
}

func (h *fakeHandler) OnSchemaChanged(_ context.Context, cluster string, schema []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.changed[cluster] = schema
	h.changeCh <- cluster
}

func (h *fakeHandler) OnSchemaDeleted(_ context.Context, cluster string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deleted = append(h.deleted, cluster)
}

// fakeSchemaServer is a gRPC server that can control stream behavior.
type fakeSchemaServer struct {
	proto.UnimplementedSchemaHandlerServer
	mu          sync.Mutex
	subscribers []proto.SchemaHandler_SubscribeServer
	subscribeCh chan struct{}
}

func newFakeSchemaServer() *fakeSchemaServer {
	return &fakeSchemaServer{
		subscribeCh: make(chan struct{}, 10),
	}
}

func (s *fakeSchemaServer) Subscribe(_ *proto.SubscribeRequest, stream proto.SchemaHandler_SubscribeServer) error {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, stream)
	s.mu.Unlock()

	s.subscribeCh <- struct{}{}

	// Block until client disconnects
	<-stream.Context().Done()
	return nil
}

func (s *fakeSchemaServer) send(resp *proto.SubscribeResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subscribers {
		if err := sub.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

func startFakeServer(t *testing.T) (*fakeSchemaServer, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	fake := newFakeSchemaServer()
	proto.RegisterSchemaHandlerServer(srv, fake)

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(func() { srv.GracefulStop() })

	return fake, lis.Addr().String()
}

func TestGRPCWatcher_ConnectsAndReceives(t *testing.T) {
	fake, addr := startFakeServer(t)
	handler := newFakeHandler()
	var connected atomic.Bool

	gw, err := watcher.NewGRPCWatcher(watcher.GRPCWatcherConfig{Address: addr}, handler, &connected)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = gw.Run(ctx)
	}()

	// Wait for subscription
	select {
	case <-fake.subscribeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for subscribe")
	}

	assert.True(t, connected.Load())

	// Send a schema update
	err = fake.send(&proto.SubscribeResponse{
		ClusterName: "test-cluster",
		Schema:      []byte("schema-data"),
		EventType:   proto.SubscribeResponse_CREATED,
	})
	require.NoError(t, err)

	select {
	case cluster := <-handler.changeCh:
		assert.Equal(t, "test-cluster", cluster)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for schema change")
	}

	cancel()
}

func TestGRPCWatcher_WaitsForServer(t *testing.T) {
	// Start watcher before server is up — it should wait and connect once server starts.
	handler := newFakeHandler()
	var connected atomic.Bool

	// Pick a port but don't listen on it yet
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	_ = lis.Close() // free the port

	gw, err := watcher.NewGRPCWatcher(watcher.GRPCWatcherConfig{Address: addr}, handler, &connected)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = gw.Run(ctx)
	}()

	// Watcher should not be connected yet
	time.Sleep(100 * time.Millisecond)
	assert.False(t, connected.Load())

	// Now start the server on the same address
	lis2, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	srv := grpc.NewServer()
	fake := newFakeSchemaServer()
	proto.RegisterSchemaHandlerServer(srv, fake)
	go func() {
		_ = srv.Serve(lis2)
	}()
	t.Cleanup(func() { srv.GracefulStop() })

	// Wait for subscribe
	select {
	case <-fake.subscribeCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for subscribe after server started")
	}

	assert.True(t, connected.Load())
	cancel()
}

func TestGRPCWatcher_ContextCancellation(t *testing.T) {
	handler := newFakeHandler()
	var connected atomic.Bool

	gw, err := watcher.NewGRPCWatcher(watcher.GRPCWatcherConfig{Address: "127.0.0.1:0"}, handler, &connected)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		done <- gw.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to return after cancel")
	}
}

// failOnceServer returns an error on the first Subscribe call, then works normally.
type failOnceServer struct {
	proto.UnimplementedSchemaHandlerServer
	mu          sync.Mutex
	callCount   int
	subscribeCh chan struct{}
}

func (s *failOnceServer) Subscribe(_ *proto.SubscribeRequest, stream proto.SchemaHandler_SubscribeServer) error {
	s.mu.Lock()
	s.callCount++
	count := s.callCount
	s.mu.Unlock()

	if count == 1 {
		return status.Error(codes.Unavailable, "not ready yet")
	}

	s.subscribeCh <- struct{}{}
	<-stream.Context().Done()
	return nil
}

func TestGRPCWatcher_RetriesAfterSubscribeError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	fake := &failOnceServer{subscribeCh: make(chan struct{}, 1)}
	srv := grpc.NewServer()
	proto.RegisterSchemaHandlerServer(srv, fake)
	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(func() { srv.GracefulStop() })

	handler := newFakeHandler()
	var connected atomic.Bool
	gw, err := watcher.NewGRPCWatcher(watcher.GRPCWatcherConfig{Address: lis.Addr().String()}, handler, &connected)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = gw.Run(ctx)
	}()

	// Should eventually connect after the first failure
	select {
	case <-fake.subscribeCh:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for retry subscribe")
	}

	assert.True(t, connected.Load())
	cancel()
}
