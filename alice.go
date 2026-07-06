package bb84

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// alice runs Alice's side of the protocol and returns her sifted key.
//
// Phase 1 (quantum transmission): she draws n random bits and n random
// bases, encodes each pair as a Qubit and sends it on the quantum channel,
// closing the channel after the last qubit.
//
// Phase 2 (sifting): she receives Bob's measurement bases on the public
// classical channel, announces the positions where the bases matched, and
// keeps her bits at those positions as the sifted key.
//
// Alice owns rng and all of her per-run state; nothing is shared with the
// other parties — every exchange goes through a channel.
func alice(ctx context.Context, n int, rng *rand.Rand, quantum chan<- Qubit, cc *classicalChannel) ([]Bit, error) {
	bits := make([]Bit, n)
	bases := make([]Basis, n)
	for i := range n {
		bits[i] = randomBit(rng)
		bases[i] = randomBasis(rng)
		if err := send(ctx, quantum, NewQubit(bits[i], bases[i])); err != nil {
			close(quantum)
			return nil, fmt.Errorf("alice: quantum transmission: %w", err)
		}
	}
	close(quantum)

	bobBases, err := recv(ctx, cc.bases)
	if err != nil {
		return nil, fmt.Errorf("alice: awaiting Bob's bases: %w", err)
	}
	if len(bobBases) != n {
		return nil, fmt.Errorf("alice: got %d bases from Bob, want %d", len(bobBases), n)
	}

	matches := make([]int, 0, n)
	for i, b := range bobBases {
		if b == bases[i] {
			matches = append(matches, i)
		}
	}
	if err := send(ctx, cc.matches, matches); err != nil {
		return nil, fmt.Errorf("alice: announcing matches: %w", err)
	}

	sifted := make([]Bit, len(matches))
	for j, i := range matches {
		sifted[j] = bits[i]
	}
	return sifted, nil
}
