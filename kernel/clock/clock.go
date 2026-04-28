// Package clock provides a small abstraction for reading the current time,
// so domain and application code stay testable.
package clock

import "time"

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func System() Clock { return systemClock{} }

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func Fixed(t time.Time) Clock { return fixedClock{t} }
