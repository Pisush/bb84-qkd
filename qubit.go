// Package bb84 simulates the BB84 quantum key distribution protocol with
// Alice, Bob, and (in later milestones) an eavesdropper Eve running as
// concurrent goroutines that communicate exclusively over channels — no
// shared memory between parties, mirroring their physical separation.
package bb84

import "math/rand/v2"

// Bit is a classical bit with value 0 or 1.
type Bit uint8

// Basis identifies one of the two conjugate BB84 preparation and
// measurement bases.
type Basis uint8

const (
	// Rectilinear is the + basis: bit 0 encodes |0⟩, bit 1 encodes |1⟩.
	Rectilinear Basis = iota
	// Diagonal is the × basis: bit 0 encodes |+⟩, bit 1 encodes |−⟩.
	Diagonal
)

// String returns "+" for Rectilinear and "×" for Diagonal.
func (b Basis) String() string {
	if b == Rectilinear {
		return "+"
	}
	return "×"
}

// Qubit is a single photon-like carrier prepared with one bit in one basis.
// It is the only message type that travels on the quantum channel.
//
// Representation choice: BB84 uses only the four states |0⟩, |1⟩, |+⟩ and
// |−⟩, and each qubit is measured exactly once, in one of the two bases.
// For these states the Born rule collapses to two cases: measuring in the
// preparation basis returns the encoded bit with probability 1, and
// measuring in the conjugate basis returns 0 or 1 with probability 1/2
// each (|⟨0|+⟩|² = |⟨0|−⟩|² = 1/2, and symmetrically). A 2-amplitude
// complex128 state vector would therefore carry no information beyond
// (bit, basis), so Qubit stores exactly that pair and Measure implements
// the resulting statistics directly.
//
// The fields are unexported: no party can read the encoded bit or the
// preparation basis without measuring, which enforces the protocol's
// physical separation in the type system.
type Qubit struct {
	bit   Bit
	basis Basis
}

// NewQubit prepares a qubit encoding bit in basis. It panics if bit or
// basis is out of range, since that is a programmer error.
func NewQubit(bit Bit, basis Basis) Qubit {
	if bit > 1 {
		panic("bb84: bit must be 0 or 1")
	}
	if basis > Diagonal {
		panic("bb84: basis must be Rectilinear or Diagonal")
	}
	return Qubit{bit: bit, basis: basis}
}

// Measure measures the qubit in basis b, drawing quantum randomness from
// rng. If b matches the preparation basis, the encoded bit is returned;
// otherwise the outcome is uniformly random, per the Born rule for the
// four BB84 states.
//
// Physically a measurement destroys the photon's prepared state; the
// simulator relies on each qubit being measured at most once by whoever
// receives it from the quantum channel, which the protocol code upholds.
func (q Qubit) Measure(b Basis, rng *rand.Rand) Bit {
	if b == q.basis {
		return q.bit
	}
	return Bit(rng.IntN(2))
}

// randomBit draws a uniformly random classical bit from rng.
func randomBit(rng *rand.Rand) Bit { return Bit(rng.IntN(2)) }

// randomBasis draws Rectilinear or Diagonal from rng with equal probability.
func randomBasis(rng *rand.Rand) Basis { return Basis(rng.IntN(2)) }
