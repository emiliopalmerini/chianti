// Package eventbus provides a synchronous in-process pub/sub used for
// cross-slice reactive rules. Handlers run in registration order; a handler
// error or panic is logged but does not abort subsequent handlers. Publish
// never returns an error: the bus fires after the producer has committed,
// so handler failure must not surface to the caller.
package eventbus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type Event interface {
	EventName() string
}

type Handler func(ctx context.Context, evt Event) error

type Bus interface {
	Subscribe(eventName string, h Handler)
	Publish(ctx context.Context, evt Event)
}

type bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	logger   *slog.Logger
}

func New() Bus {
	return &bus{
		handlers: make(map[string][]Handler),
		logger:   slog.Default(),
	}
}

func NewWithLogger(logger *slog.Logger) Bus {
	return &bus{
		handlers: make(map[string][]Handler),
		logger:   logger,
	}
}

func (b *bus) Subscribe(eventName string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventName] = append(b.handlers[eventName], h)
}

func (b *bus) Publish(ctx context.Context, evt Event) {
	name := evt.EventName()
	b.mu.RLock()
	hs := make([]Handler, len(b.handlers[name]))
	copy(hs, b.handlers[name])
	b.mu.RUnlock()
	for i, h := range hs {
		b.invoke(ctx, name, i, h, evt)
	}
}

// invoke runs a single handler with panic recovery so a crashing subscriber
// cannot unwind into the producer goroutine, which has already committed.
func (b *bus) invoke(ctx context.Context, name string, idx int, h Handler, evt Event) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("eventbus handler panic",
				"event", name,
				"handler_index", idx,
				"panic", fmt.Sprint(r),
			)
		}
	}()
	if err := h(ctx, evt); err != nil {
		b.logger.Warn("eventbus handler error",
			"event", name,
			"handler_index", idx,
			"err", err,
		)
	}
}
