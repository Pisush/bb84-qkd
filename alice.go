package bb84

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// alice runs Alice's side of the protocol.
//
// Phase 1 (quantum transmission): she draws n random bits and n random
// bases, encodes each pair as a Qubit and sends it on the quantum channel,
// closing the channel after the last qubit.
//
// Phase 2 (sifting): she receives Bob's measurement bases on the public
// classical channel, announces the positions where the bases matched, and
// keeps her bits at those positions as the sifted key.
//
// Phase 3 (error estimation): she sacrifices a random sample of the sifted
// key — sampleFraction of it — by announcing those positions and her bit
// values publicly. Bob replies with the number of mismatches, from which
// she computes the QBER. The sampled bits are removed from the key.
//
// Phase 4 (decision): she aborts, discarding the key, if the QBER exceeds
// threshold.
//
// Alice owns rng and all of her per-run state; nothing is shared with the
// other parties — every exchange goes through a channel.
func alice(ctx context.Context, n int, sampleFraction, threshold float64, rng *rand.Rand, quantum chan<- Qubit, cc *classicalChannel) (partyOutput, error) {
	bits := make([]Bit, n)
	bases := make([]Basis, n)
	for i := range n {
		bits[i] = randomBit(rng)
		bases[i] = randomBasis(rng)
		if err := send(ctx, quantum, NewQubit(bits[i], bases[i])); err != nil {
			close(quantum)
			return partyOutput{}, fmt.Errorf("alice: quantum transmission: %w", err)
		}
	}
	close(quantum)

	// Sifting.
	bobBases, err := recv(ctx, cc.bases)
	if err != nil {
		return partyOutput{}, fmt.Errorf("alice: awaiting Bob's bases: %w", err)
	}
	if len(bobBases) != n {
		return partyOutput{}, fmt.Errorf("alice: got %d bases from Bob, want %d", len(bobBases), n)
	}
	matches := make([]int, 0, n)
	for i, b := range bobBases {
		if b == bases[i] {
			matches = append(matches, i)
		}
	}
	if err := send(ctx, cc.matches, matches); err != nil {
		return partyOutput{}, fmt.Errorf("alice: announcing matches: %w", err)
	}
	sifted := make([]Bit, len(matches))
	for j, i := range matches {
		sifted[j] = bits[i]
	}

	// Error estimation: sacrifice a random sample, announce it, and learn
	// from Bob how many positions disagreed.
	k := sampleSize(len(sifted), sampleFraction)
	positions := rng.Perm(len(sifted))[:k]
	ann := sampleAnnouncement{positions: positions, bits: make([]Bit, k)}
	for j, p := range positions {
		ann.bits[j] = sifted[p]
	}
	if err := send(ctx, cc.sample, ann); err != nil {
		return partyOutput{}, fmt.Errorf("alice: announcing sample: %w", err)
	}
	mismatches, err := recv(ctx, cc.mismatches)
	if err != nil {
		return partyOutput{}, fmt.Errorf("alice: awaiting mismatch count: %w", err)
	}
	if mismatches < 0 || mismatches > k {
		return partyOutput{}, fmt.Errorf("alice: mismatch count %d out of range [0,%d]", mismatches, k)
	}

	out := partyOutput{
		sifted:     sifted,
		key:        removePositions(sifted, positions),
		sampleSize: k,
	}
	if k > 0 {
		out.qber = float64(mismatches) / float64(k)
	}
	return out.decide(threshold), nil
}
