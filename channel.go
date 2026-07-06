package bb84

import (
	"context"
	"errors"
)

// errClosed reports that the other party closed a classical channel before
// the expected message arrived.
var errClosed = errors.New("bb84: classical channel closed unexpectedly")

// classicalChannel models the authenticated public classical channel of
// BB84. Each field carries exactly one phase of the public discussion, and
// the message direction is fixed by which end each party holds. Only basis
// information travels here in this milestone; sifted-key bit values never
// do (later milestones add the sacrificed error-check sample, which is the
// sole exception the protocol allows).
type classicalChannel struct {
	// bases carries Bob → Alice: the measurement basis Bob used for each
	// position, in transmission order.
	bases chan []Basis
	// matches carries Alice → Bob: the positions at which Bob's basis
	// matched Alice's preparation basis (the sifted positions).
	matches chan []int
}

// newClassicalChannel wires up the public discussion channels. They are
// unbuffered: a public announcement is only "made" once the counterpart
// has received it.
func newClassicalChannel() *classicalChannel {
	return &classicalChannel{
		bases:   make(chan []Basis),
		matches: make(chan []int),
	}
}

// send delivers v on ch, or gives up when ctx is cancelled.
func send[T any](ctx context.Context, ch chan<- T, v T) error {
	select {
	case ch <- v:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// recv receives the next value from ch, or gives up when ctx is cancelled.
// A closed channel yields errClosed.
func recv[T any](ctx context.Context, ch <-chan T) (T, error) {
	select {
	case v, ok := <-ch:
		if !ok {
			var zero T
			return zero, errClosed
		}
		return v, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}
