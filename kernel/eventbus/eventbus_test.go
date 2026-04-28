package eventbus_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/emiliopalmerini/chianti/kernel/eventbus"
)

type fakeEvent struct{ name string }

func (e fakeEvent) EventName() string { return e.name }

func TestPublishInvokesHandlers(t *testing.T) {
	bus := eventbus.New()
	var count int
	bus.Subscribe("ping", func(ctx context.Context, evt eventbus.Event) error {
		count++
		return nil
	})
	bus.Publish(context.Background(), fakeEvent{name: "ping"})
	if count != 1 {
		t.Errorf("expected 1 call, got %d", count)
	}
}

func TestPublishContinuesAfterHandlerError(t *testing.T) {
	bus := eventbus.New()
	var second bool
	bus.Subscribe("boom", func(ctx context.Context, evt eventbus.Event) error {
		return errors.New("fail")
	})
	bus.Subscribe("boom", func(ctx context.Context, evt eventbus.Event) error {
		second = true
		return nil
	})
	bus.Publish(context.Background(), fakeEvent{name: "boom"})
	if !second {
		t.Error("second handler was not called after first handler failed")
	}
}

func TestPublishUnknownEvent(t *testing.T) {
	bus := eventbus.New()
	bus.Publish(context.Background(), fakeEvent{name: "nope"})
}

func TestPublishRecoversFromHandlerPanic(t *testing.T) {
	bus := eventbus.New()
	var second bool
	bus.Subscribe("crash", func(ctx context.Context, evt eventbus.Event) error {
		panic("kaboom")
	})
	bus.Subscribe("crash", func(ctx context.Context, evt eventbus.Event) error {
		second = true
		return nil
	})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Publish must not propagate panic, got %v", r)
		}
	}()
	bus.Publish(context.Background(), fakeEvent{name: "crash"})
	if !second {
		t.Error("second handler was not called after first handler panicked")
	}
}

func TestPublishRunsHandlersInRegistrationOrder(t *testing.T) {
	bus := eventbus.New()
	var order []int
	for i := 0; i < 5; i++ {
		i := i
		bus.Subscribe("seq", func(ctx context.Context, evt eventbus.Event) error {
			order = append(order, i)
			return nil
		})
	}
	bus.Publish(context.Background(), fakeEvent{name: "seq"})
	for i, v := range order {
		if v != i {
			t.Fatalf("handler order mismatch at %d: got %d", i, v)
		}
	}
}

func TestSubscribeDuringPublishDoesNotRunForInFlightEvent(t *testing.T) {
	bus := eventbus.New()
	var lateRan bool
	bus.Subscribe("snap", func(ctx context.Context, evt eventbus.Event) error {
		bus.Subscribe("snap", func(ctx context.Context, evt eventbus.Event) error {
			lateRan = true
			return nil
		})
		return nil
	})
	bus.Publish(context.Background(), fakeEvent{name: "snap"})
	if lateRan {
		t.Error("handler subscribed during Publish must not run for the in-flight event")
	}
}

func TestPublishIsSafeFromConcurrentGoroutines(t *testing.T) {
	bus := eventbus.New()
	var mu sync.Mutex
	var count int
	bus.Subscribe("race", func(ctx context.Context, evt eventbus.Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(context.Background(), fakeEvent{name: "race"})
		}()
	}
	wg.Wait()

	if count != 50 {
		t.Errorf("expected 50 calls, got %d", count)
	}
}
