package eventbus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recvWithTimeout receives from ch with a timeout, returning ok=false on timeout.
func recvWithTimeout(t *testing.T, ch <-chan Event) (Event, bool) {
	t.Helper()
	select {
	case e, ok := <-ch:
		return e, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil, false
	}
}

func TestPublishSubscribe(t *testing.T) {
	b := NewBus(4)
	defer b.Close()

	ch := b.Subscribe(TagFilePublished)
	want := FilePublished{FileID: 42}

	b.Publish(context.Background(), want)

	got, ok := recvWithTimeout(t, ch)
	if !ok {
		t.Fatal("channel closed unexpectedly")
	}
	gotFP, ok := got.(FilePublished)
	if !ok {
		t.Fatalf("expected FilePublished, got %T", got)
	}
	if gotFP.FileID != want.FileID {
		t.Fatalf("expected FileID %d, got %d", want.FileID, gotFP.FileID)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := NewBus(4)
	defer b.Close()

	ch1 := b.Subscribe(TagFilePublished)
	ch2 := b.Subscribe(TagFilePublished)
	want := FilePublished{FileID: 7}

	b.Publish(context.Background(), want)

	for i, ch := range []<-chan Event{ch1, ch2} {
		got, ok := recvWithTimeout(t, ch)
		if !ok {
			t.Fatalf("subscriber %d channel closed unexpectedly", i)
		}
		if fp, ok := got.(FilePublished); !ok || fp.FileID != want.FileID {
			t.Fatalf("subscriber %d: expected FilePublished{FileID:%d}, got %#v", i, want.FileID, got)
		}
	}
}

func TestMultipleEventTypes(t *testing.T) {
	b := NewBus(4)
	defer b.Close()

	fileCh := b.Subscribe(TagFilePublished)
	ruleCh := b.Subscribe(TagRuleFolderCreated)
	nodeCh := b.Subscribe(TagNodeTreeChanged)

	fileEv := FilePublished{FileID: 1}
	ruleEv := RuleFolderCreated{NodeID: 2}
	nodeEv := NodeTreeChanged{ViewID: 3}

	b.Publish(context.Background(), fileEv)
	b.Publish(context.Background(), ruleEv)
	b.Publish(context.Background(), nodeEv)

	// Each subscriber must receive only its own event type.
	if got, ok := recvWithTimeout(t, fileCh); !ok {
		t.Fatal("file channel closed")
	} else if fp, ok := got.(FilePublished); !ok || fp.FileID != 1 {
		t.Fatalf("file subscriber: expected FilePublished{FileID:1}, got %#v", got)
	}

	if got, ok := recvWithTimeout(t, ruleCh); !ok {
		t.Fatal("rule channel closed")
	} else if rf, ok := got.(RuleFolderCreated); !ok || rf.NodeID != 2 {
		t.Fatalf("rule subscriber: expected RuleFolderCreated{NodeID:2}, got %#v", got)
	}

	if got, ok := recvWithTimeout(t, nodeCh); !ok {
		t.Fatal("node channel closed")
	} else if nt, ok := got.(NodeTreeChanged); !ok || nt.ViewID != 3 {
		t.Fatalf("node subscriber: expected NodeTreeChanged{ViewID:3}, got %#v", got)
	}

	// Ensure no cross-delivery: file subscriber should have nothing else queued.
	select {
	case got := <-fileCh:
		t.Fatalf("file subscriber received unexpected event %#v", got)
	default:
	}
}

func TestPublishAfterClose(t *testing.T) {
	b := NewBus(4)
	b.Subscribe(TagFilePublished)
	b.Close()

	// Must not panic.
	b.Publish(context.Background(), FilePublished{FileID: 1})
	b.Close()
}

func TestWorkerCancellation(t *testing.T) {
	b := NewBus(4)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var handled atomic.Int64
	done := make(chan struct{})

	// Wrap the worker so we can observe when the goroutine exits.
	ch := b.Subscribe(TagFilePublished)
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				handled.Add(1)
				_ = e
			}
		}
	}()

	b.Publish(context.Background(), FilePublished{FileID: 5})
	// Wait for the handler to run.
	deadline := time.Now().Add(2 * time.Second)
	for handled.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if handled.Load() == 0 {
		t.Fatal("worker handler never ran")
	}

	cancel()
	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("worker goroutine did not exit after cancellation")
	}
}

func TestBoundedChannel(t *testing.T) {
	// buffer=1, no subscriber reading. Publishing 3 events should accept only
	// 1 (the buffer) and drop the other 2.
	b := NewBus(1)
	defer b.Close()

	ch := b.Subscribe(TagFilePublished)

	b.Publish(context.Background(), FilePublished{FileID: 1})
	b.Publish(context.Background(), FilePublished{FileID: 2})
	b.Publish(context.Background(), FilePublished{FileID: 3})

	// Exactly one event should be buffered.
	select {
	case <-ch:
	default:
		t.Fatal("expected exactly one buffered event")
	}
	select {
	case <-ch:
		t.Fatal("expected only one event buffered, got a second")
	default:
	}
}

func TestStartWorker(t *testing.T) {
	b := NewBus(4)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var got []int64
	worker := Worker{
		name: "test-worker",
		handler: func(_ context.Context, e Event) {
			if fp, ok := e.(FilePublished); ok {
				mu.Lock()
				got = append(got, fp.FileID)
				mu.Unlock()
			}
		},
	}

	b.StartWorker(ctx, worker, TagFilePublished)
	b.Publish(context.Background(), FilePublished{FileID: 10})
	b.Publish(context.Background(), FilePublished{FileID: 20})

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker handled %d events, want 2", n)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if got[0] != 10 || got[1] != 20 {
		t.Fatalf("worker received events in wrong order: %v", got)
	}
}