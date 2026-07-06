package bb84

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// qber computes the fraction of positions at which a and b disagree.
// It is a test helper standing in for the public error estimation that
// arrives in a later milestone.
func qber(t *testing.T, a, b []Bit) float64 {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("key lengths differ: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0
	}
	errs := 0
	for i := range a {
		if a[i] != b[i] {
			errs++
		}
	}
	return float64(errs) / float64(len(a))
}

func TestRunNoEveSiftedKeysMatch(t *testing.T) {
	tests := []struct {
		name string
		n    int
		seed uint64
	}{
		{"default N seed 1", 0, 1}, // N=0 → DefaultN
		{"N=4096 seed 42", 4096, 42},
		{"N=8192 seed 7", 8192, 7},
		{"small N=64 seed 3", 64, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Run(context.Background(), Config{N: tt.n, Seed: tt.seed})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			n := tt.n
			if n == 0 {
				n = DefaultN
			}
			if res.N != n {
				t.Errorf("Result.N = %d, want %d", res.N, n)
			}

			// Without an eavesdropper the sifted keys must agree exactly.
			if got := qber(t, res.AliceSifted, res.BobSifted); got > epsilon {
				t.Errorf("QBER = %v, want 0 (± %v)", got, epsilon)
			}

			// Sifting keeps ~50% of positions. Binomial(n, 0.5):
			// σ = √(n/4); allow ±6σ.
			mean := float64(n) / 2
			tol := 6 * math.Sqrt(float64(n)/4)
			if got := float64(len(res.AliceSifted)); math.Abs(got-mean) > tol {
				t.Errorf("sifted length = %v, want %v ± %v", got, mean, tol)
			}
		})
	}
}

func TestRunIsDeterministicForSeed(t *testing.T) {
	cfg := Config{N: 2048, Seed: 1234}
	first, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if len(first.AliceSifted) != len(second.AliceSifted) {
		t.Fatalf("sifted lengths differ across runs: %d vs %d",
			len(first.AliceSifted), len(second.AliceSifted))
	}
	for i := range first.AliceSifted {
		if first.AliceSifted[i] != second.AliceSifted[i] {
			t.Fatalf("sifted keys differ at position %d", i)
		}
	}
}

func TestRunRejectsNegativeN(t *testing.T) {
	if _, err := Run(context.Background(), Config{N: -1}); err == nil {
		t.Error("Run with N=-1 succeeded, want error")
	}
}

func TestRunHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the run even starts

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = Run(ctx, Config{N: 1 << 20, Seed: 5})
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run error = %v, want context.Canceled", err)
	}
}
