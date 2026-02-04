package broadcaster_test

import (
	"context"
	"testing"
	"time"

	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/broadcaster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribeAndPublish(t *testing.T) {
	b := broadcaster.New[string]()
	ctx := t.Context()

	ch1 := b.Subscribe(ctx)
	ch2 := b.Subscribe(ctx)
	require.Equal(t, 2, b.SubscriberCount())

	b.Publish(ctx, "hello")

	for _, ch := range []<-chan string{ch1, ch2} {
		select {
		case msg := <-ch:
			assert.Equal(t, "hello", msg)
		case <-time.After(50 * time.Millisecond):
			t.Fatal("timeout waiting for message")
		}
	}
}

func TestSubscribe_ContextCancellation(t *testing.T) {
	b := broadcaster.New[string]()
	ctx, cancel := context.WithCancel(context.Background())

	ch := b.Subscribe(ctx)
	require.Equal(t, 1, b.SubscriberCount())

	cancel()

	assert.Eventually(t, func() bool {
		return b.SubscriberCount() == 0
	}, 100*time.Millisecond, 5*time.Millisecond)

	_, ok := <-ch
	assert.False(t, ok, "channel should be closed after context cancellation")
}

func TestPublish_NonBlocking(t *testing.T) {
	b := broadcaster.New[int]()
	ctx := t.Context()

	ch := b.Subscribe(ctx)

	// Fill the buffer
	b.Publish(ctx, 1)

	// Should not block even though buffer is full
	done := make(chan struct{})
	go func() {
		b.Publish(ctx, 2)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Publish blocked on slow consumer")
	}

	assert.Equal(t, 1, <-ch)
}
