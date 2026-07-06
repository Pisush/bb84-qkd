package bb84

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// bob runs Bob's side of the protocol and returns his sifted key.
//
// Phase 1 (quantum transmission): he measures every qubit arriving on the
// quantum channel in a freshly drawn random basis, until the channel is
// closed.
//
// Phase 2 (sifting): he announces his measurement bases on the public
// classical channel, receives from Alice the positions where the bases
// matched, and keeps his measurement outcomes at those positions as the
// sifted key.
//
// Bob owns rng and all of his per-run state; nothing is shared with the
// other parties — every exchange goes through a channel.
func bob(ctx context.Context, rng *rand.Rand, quantum <-chan Qubit, cc *classicalChannel) ([]Bit, error) {
	var (
		outcomes []Bit
		bases    []Basis
	)
receive:
	for {
		select {
		case q, ok := <-quantum:
			if !ok {
				break receive
			}
			basis := randomBasis(rng)
			bases = append(bases, basis)
			outcomes = append(outcomes, q.Measure(basis, rng))
		case <-ctx.Done():
			return nil, fmt.Errorf("bob: quantum reception: %w", ctx.Err())
		}
	}

	if err := send(ctx, cc.bases, bases); err != nil {
		return nil, fmt.Errorf("bob: announcing bases: %w", err)
	}
	matches, err := recv(ctx, cc.matches)
	if err != nil {
		return nil, fmt.Errorf("bob: awaiting matches: %w", err)
	}

	sifted := make([]Bit, 0, len(matches))
	for _, i := range matches {
		if i < 0 || i >= len(outcomes) {
			return nil, fmt.Errorf("bob: match position %d out of range [0,%d)", i, len(outcomes))
		}
		sifted = append(sifted, outcomes[i])
	}
	return sifted, nil
}
