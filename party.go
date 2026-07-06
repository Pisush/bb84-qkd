package bb84

// partyOutput is everything one party privately knows at the end of a
// session: its sifted key, the final key after the error-check sample was
// removed (nil if the run was aborted), its estimate of the quantum bit
// error rate, and its abort/accept decision.
type partyOutput struct {
	sifted     []Bit
	key        []Bit
	sampleSize int
	qber       float64
	accepted   bool
}

// decide applies the protocol's phase-4 decision to a party's provisional
// output: abort — discarding the key — when the estimated QBER exceeds
// threshold. Both parties call it with identical public inputs, so their
// decisions always agree.
func (o partyOutput) decide(threshold float64) partyOutput {
	o.accepted = o.qber <= threshold
	if !o.accepted {
		o.key = nil
	}
	return o
}

// removePositions returns key with the bits at the given positions removed,
// preserving order. Positions must be valid indices into key; duplicates
// are tolerated (removed once).
func removePositions(key []Bit, positions []int) []Bit {
	if len(positions) == 0 {
		return key
	}
	drop := make(map[int]bool, len(positions))
	for _, p := range positions {
		drop[p] = true
	}
	kept := make([]Bit, 0, len(key)-len(drop))
	for i, b := range key {
		if !drop[i] {
			kept = append(kept, b)
		}
	}
	return kept
}

// sampleSize returns how many sifted bits are sacrificed for error
// estimation: fraction of n, rounded to nearest, clamped to [0, n].
func sampleSize(n int, fraction float64) int {
	k := int(float64(n)*fraction + 0.5)
	if k < 0 {
		k = 0
	}
	if k > n {
		k = n
	}
	return k
}
