package bb84

import (
	"math"
	"math/rand/v2"
	"testing"
)

// epsilon is the tolerance for floating-point comparisons; never use ==.
const epsilon = 1e-9

func newTestRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, 0x7E57))
}

func TestMeasureSameBasisIsDeterministic(t *testing.T) {
	tests := []struct {
		name  string
		bit   Bit
		basis Basis
	}{
		{"zero rectilinear |0⟩", 0, Rectilinear},
		{"one rectilinear |1⟩", 1, Rectilinear},
		{"zero diagonal |+⟩", 0, Diagonal},
		{"one diagonal |−⟩", 1, Diagonal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := newTestRNG(1)
			q := NewQubit(tt.bit, tt.basis)
			// Repeat: same-basis measurement must never depend on rng.
			for i := 0; i < 100; i++ {
				if got := q.Measure(tt.basis, rng); got != tt.bit {
					t.Fatalf("Measure(%v) = %d, want %d (trial %d)", tt.basis, got, tt.bit, i)
				}
			}
		})
	}
}

func TestMeasureConjugateBasisIsUniform(t *testing.T) {
	tests := []struct {
		name    string
		bit     Bit
		prep    Basis
		measure Basis
	}{
		{"|0⟩ in ×", 0, Rectilinear, Diagonal},
		{"|1⟩ in ×", 1, Rectilinear, Diagonal},
		{"|+⟩ in +", 0, Diagonal, Rectilinear},
		{"|−⟩ in +", 1, Diagonal, Rectilinear},
	}
	const trials = 100000
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := newTestRNG(42)
			q := NewQubit(tt.bit, tt.prep)
			ones := 0
			for i := 0; i < trials; i++ {
				ones += int(q.Measure(tt.measure, rng))
			}
			p := float64(ones) / trials
			// Binomial(trials, 0.5): σ ≈ 0.0016; 6σ ≈ 0.0095.
			if math.Abs(p-0.5) > 0.01 {
				t.Errorf("P(1) = %.4f, want 0.5 ± 0.01", p)
			}
		})
	}
}

func TestNewQubitPanicsOnBadInput(t *testing.T) {
	tests := []struct {
		name  string
		bit   Bit
		basis Basis
	}{
		{"bit out of range", 2, Rectilinear},
		{"basis out of range", 0, Basis(2)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("NewQubit(%d, %d) did not panic", tt.bit, tt.basis)
				}
			}()
			NewQubit(tt.bit, tt.basis)
		})
	}
}

func TestBasisString(t *testing.T) {
	tests := []struct {
		basis Basis
		want  string
	}{
		{Rectilinear, "+"},
		{Diagonal, "×"},
	}
	for _, tt := range tests {
		if got := tt.basis.String(); got != tt.want {
			t.Errorf("Basis(%d).String() = %q, want %q", tt.basis, got, tt.want)
		}
	}
}
