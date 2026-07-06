package bb84

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// bob runs Bob's side of the protocol.
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
// Phase 3 (error estimation): he receives Alice's sacrificed sample,
// counts how many positions disagree with his sifted key, announces that
// count publicly, and computes the QBER. The sampled bits are removed from
// the key.
//
// Bob owns rng and all of his per-run state; nothing is shared with the
// other parties — every exchange goes through a channel.
func bob(ctx context.Context, rng *rand.Rand, quantum <-chan Qubit, cc *classicalChannel) (partyOutput, error) {
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
			return partyOutput{}, fmt.Errorf("bob: quantum reception: %w", ctx.Err())
		}
	}

	// Sifting.
	if err := send(ctx, cc.bases, bases); err != nil {
		return partyOutput{}, fmt.Errorf("bob: announcing bases: %w", err)
	}
	matches, err := recv(ctx, cc.matches)
	if err != nil {
		return partyOutput{}, fmt.Errorf("bob: awaiting matches: %w", err)
	}
	sifted := make([]Bit, 0, len(matches))
	for _, i := range matches {
		if i < 0 || i >= len(outcomes) {
			return partyOutput{}, fmt.Errorf("bob: match position %d out of range [0,%d)", i, len(outcomes))
		}
		sifted = append(sifted, outcomes[i])
	}

	// Error estimation: compare Alice's announced sample against the
	// corresponding sifted bits and publish the mismatch count.
	ann, err := recv(ctx, cc.sample)
	if err != nil {
		return partyOutput{}, fmt.Errorf("bob: awaiting sample: %w", err)
	}
	if len(ann.positions) != len(ann.bits) {
		return partyOutput{}, fmt.Errorf("bob: sample has %d positions but %d bits", len(ann.positions), len(ann.bits))
	}
	mismatches := 0
	for j, p := range ann.positions {
		if p < 0 || p >= len(sifted) {
			return partyOutput{}, fmt.Errorf("bob: sample position %d out of range [0,%d)", p, len(sifted))
		}
		if sifted[p] != ann.bits[j] {
			mismatches++
		}
	}
	if err := send(ctx, cc.mismatches, mismatches); err != nil {
		return partyOutput{}, fmt.Errorf("bob: announcing mismatch count: %w", err)
	}

	out := partyOutput{
		sifted:     sifted,
		key:        removePositions(sifted, ann.positions),
		sampleSize: len(ann.positions),
	}
	if len(ann.positions) > 0 {
		out.qber = float64(mismatches) / float64(len(ann.positions))
	}
	return out, nil
}
