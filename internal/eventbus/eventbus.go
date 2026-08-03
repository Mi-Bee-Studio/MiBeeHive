// Package eventbus provides a typed, in-process event bus with multiple
// subscribers per event type and a worker framework with graceful shutdown.
//
// The bus uses buffered channels to decouple publishers from subscribers.
// Publish is non-blocking: if a subscriber's buffer is full, the event is
// dropped and a warning is logged rather than blocking the publisher.
package eventbus

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Event tags identify the event type a subscriber is interested in. They are
// passed to Subscribe and derived from an event in Publish.
const (
	TagFilePublished       = "file.published"
	TagFileRemoved         = "file.removed"
	TagFileMetadataChanged = "file.metadata_changed"
	TagRuleFolderCreated   = "rule.folder_created"
	TagRuleFolderUpdated   = "rule.folder_updated"
	TagNodeTreeChanged     = "node.tree_changed"
	TagChannelChanged      = "channel.changed"
)

// Event is the marker interface implemented by all concrete event types.
// The unexported eventTag method keeps the set of event types closed to this
// package while still allowing type-safe dispatch.
type Event interface {
	eventTag()
}

// FilePublished is emitted when a collected file becomes available for supply.
type FilePublished struct{ FileID int64 }

// FileRemoved is emitted when a collected file is deleted.
type FileRemoved struct{ FileID int64 }

// FileMetadataChanged is emitted when a file's metadata is updated.
type FileMetadataChanged struct{ FileID int64 }

// RuleFolderCreated is emitted when a rule folder is created.
type RuleFolderCreated struct{ NodeID int64 }

// RuleFolderUpdated is emitted when a rule folder is updated.
type RuleFolderUpdated struct{ NodeID int64 }

// NodeTreeChanged is emitted when the rule node tree changes.
type NodeTreeChanged struct{ ViewID int64 }

// ChannelChanged is emitted when a channel is changed.
type ChannelChanged struct{ ChannelID int64 }

func (FilePublished) eventTag()       {}
func (FileRemoved) eventTag()         {}
func (FileMetadataChanged) eventTag() {}
func (RuleFolderCreated) eventTag()   {}
func (RuleFolderUpdated) eventTag()   {}
func (NodeTreeChanged) eventTag()     {}
func (ChannelChanged) eventTag()      {}

// Bus routes typed events from publishers to subscribers. It is safe for
// concurrent use by multiple goroutines.
type Bus struct {
	// subscribers maps an event tag to the list of subscriber channels.
	subscribers map[string][]chan Event
	// bufferSize is the capacity of each subscriber channel.
	bufferSize int
	mu         sync.RWMutex
	closed     atomic.Bool
}

// NewBus creates a Bus whose subscriber channels are buffered with the given
// bufferSize. bufferSize must be >= 1 to prevent unbuffered (OOM-prone) sends.
func NewBus(bufferSize int) *Bus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &Bus{
		subscribers: make(map[string][]chan Event),
		bufferSize:  bufferSize,
	}
}

// Subscribe registers a new subscriber for the given event tag and returns a
// receive-only channel on which matching events will be delivered. The channel
// is buffered with the bus's buffer size. If the bus is already closed, the
// returned channel is closed immediately.
func (b *Bus) Subscribe(eventTag string) <-chan Event {
	ch := make(chan Event, b.bufferSize)

	b.mu.Lock()
	if b.closed.Load() {
		b.mu.Unlock()
		close(ch)
		return ch
	}
	b.subscribers[eventTag] = append(b.subscribers[eventTag], ch)
	b.mu.Unlock()
	return ch
}

// Publish delivers e to every subscriber of its event tag. The send is
// non-blocking: if a subscriber's buffer is full, the event is dropped for
// that subscriber and a warning is logged. Publishing to a closed bus is a
// no-op.
func (b *Bus) Publish(ctx context.Context, e Event) {
	if b.closed.Load() {
		return
	}

	tag := eventTagOf(e)

	b.mu.RLock()
	subs := b.subscribers[tag]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		case <-ctx.Done():
			return
		default:
			slog.Warn("eventbus: subscriber buffer full, dropping event",
				"event_tag", tag,
				"event", eventName(e),
			)
		}
	}
}

// Close closes all subscriber channels and marks the bus as closed. It is safe
// to call multiple times; subsequent calls are no-ops.
func (b *Bus) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for tag, subs := range b.subscribers {
		for _, ch := range subs {
			close(ch)
		}
		delete(b.subscribers, tag)
	}
}

// Worker runs a handler for events of a single tag. StartWorker subscribes to
// eventTag and runs handler in a goroutine until ctx is cancelled or the
// subscriber channel is closed.
type Worker struct {
	name    string
	handler func(ctx context.Context, e Event)
}

// StartWorker subscribes to eventTag and runs worker.handler in a background
// goroutine. The goroutine exits when ctx is cancelled or the bus is closed.
func (b *Bus) StartWorker(ctx context.Context, worker Worker, eventTag string) {
	ch := b.Subscribe(eventTag)
	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("bus: worker stopped", "worker", worker.name)
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				worker.handler(ctx, e)
			}
		}
	}()
}

// eventTagOf returns the event tag string for an event.
func eventTagOf(e Event) string {
	switch e.(type) {
	case FilePublished:
		return TagFilePublished
	case FileRemoved:
		return TagFileRemoved
	case FileMetadataChanged:
		return TagFileMetadataChanged
	case RuleFolderCreated:
		return TagRuleFolderCreated
	case RuleFolderUpdated:
		return TagRuleFolderUpdated
	case NodeTreeChanged:
		return TagNodeTreeChanged
	case ChannelChanged:
		return TagChannelChanged
	default:
		return "unknown"
	}
}

// eventName returns a human-readable name for an event for logging.
func eventName(e Event) string {
	switch e.(type) {
	case FilePublished:
		return "FilePublished"
	case FileRemoved:
		return "FileRemoved"
	case FileMetadataChanged:
		return "FileMetadataChanged"
	case RuleFolderCreated:
		return "RuleFolderCreated"
	case RuleFolderUpdated:
		return "RuleFolderUpdated"
	case NodeTreeChanged:
		return "NodeTreeChanged"
	case ChannelChanged:
		return "ChannelChanged"
	default:
		return "Unknown"
	}
}