package bb84

import (
	"context"
	"math"
	"testing"
)

func TestRunFullEveQBERNear25Percent(t *testing.T) {
	// A full intercept-resend attack corrupts ~25% of sifted positions.
	// With N=16384 the sample holds ~2048 bits, so the QBER estimate has
	// σ = √(0.25·0.75/2048) ≈ 0.0096; ±0.05 is >5σ. Seeds are fixed, so
	// the test is deterministic regardless.
	tests := []struct {
		name string
		seed uint64
	}{
		{"seed 1", 1},
		{"seed 42", 42},
		{"seed 20260706", 20260706},
	}
	const n = 16384
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Run(context.Background(), Config{N: n, Seed: tt.seed, Eve: true})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if math.Abs(res.QBER-0.25) > 0.05 {
				t.Errorf("QBER = %.4f, want 0.25 ± 0.05", res.QBER)
			}
			// The sample estimate should reflect the true sifted-key
			// error rate, which must also sit near 25%.
			if got := qber(t, res.AliceSifted, res.BobSifted); math.Abs(got-0.25) > 0.05 {
				t.Errorf("true sifted error rate = %.4f, want 0.25 ± 0.05", got)
			}
		})
	}
}

func TestRunPartialEveQBERScales(t *testing.T) {
	// Eve intercepting a fraction f of qubits induces QBER ≈ f/4.
	tests := []struct {
		name     string
		fraction float64
		want     float64
	}{
		{"quarter", 0.25, 0.0625},
		{"half", 0.5, 0.125},
		{"three quarters", 0.75, 0.1875},
	}
	const n = 16384
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Run(context.Background(), Config{
				N: n, Seed: 7, Eve: true, EveFraction: tt.fraction,
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if math.Abs(res.QBER-tt.want) > 0.05 {
				t.Errorf("QBER = %.4f, want %.4f ± 0.05", res.QBER, tt.want)
			}
		})
	}
}

func TestRunNoEveQBERZeroAndKeysMatch(t *testing.T) {
	res, err := Run(context.Background(), Config{N: 4096, Seed: 99})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.QBER > epsilon {
		t.Errorf("QBER = %v, want 0 (± %v)", res.QBER, epsilon)
	}
	if got := qber(t, res.AliceKey, res.BobKey); got > epsilon {
		t.Errorf("final key error rate = %v, want 0 (± %v)", got, epsilon)
	}
	if len(res.AliceKey) == 0 {
		t.Error("final key is empty")
	}
}

func TestRunSampleAccounting(t *testing.T) {
	// The lax threshold keeps the noisy full-Eve run accepted, so the
	// final keys survive for the length accounting below.
	res, err := Run(context.Background(), Config{N: 4096, Seed: 11, Eve: true, QBERThreshold: 0.9})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Default sample fraction is 25% of the sifted key, rounded.
	want := sampleSize(res.SiftedLen, DefaultSampleFraction)
	if res.SampleSize != want {
		t.Errorf("SampleSize = %d, want %d (25%% of %d)", res.SampleSize, want, res.SiftedLen)
	}
	if got := len(res.AliceKey); got != res.SiftedLen-res.SampleSize {
		t.Errorf("len(AliceKey) = %d, want SiftedLen-SampleSize = %d", got, res.SiftedLen-res.SampleSize)
	}
	if got := len(res.BobKey); got != res.SiftedLen-res.SampleSize {
		t.Errorf("len(BobKey) = %d, want SiftedLen-SampleSize = %d", got, res.SiftedLen-res.SampleSize)
	}
}

func TestRunWithEveIsDeterministicForSeed(t *testing.T) {
	cfg := Config{N: 2048, Seed: 321, Eve: true, EveFraction: 0.5}
	first, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if math.Abs(first.QBER-second.QBER) > epsilon {
		t.Fatalf("QBER differs across runs: %v vs %v", first.QBER, second.QBER)
	}
	if len(first.BobKey) != len(second.BobKey) {
		t.Fatalf("key lengths differ across runs: %d vs %d", len(first.BobKey), len(second.BobKey))
	}
	for i := range first.BobKey {
		if first.BobKey[i] != second.BobKey[i] {
			t.Fatalf("Bob's keys differ at position %d", i)
		}
	}
}

func TestRunRejectsBadFractions(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"eve fraction negative", Config{Eve: true, EveFraction: -0.1}},
		{"eve fraction above one", Config{Eve: true, EveFraction: 1.1}},
		{"sample fraction negative", Config{SampleFraction: -0.1}},
		{"sample fraction above one", Config{SampleFraction: 1.5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Run(context.Background(), tt.cfg); err == nil {
				t.Errorf("Run(%+v) succeeded, want error", tt.cfg)
			}
		})
	}
}

func TestRemovePositions(t *testing.T) {
	tests := []struct {
		name      string
		key       []Bit
		positions []int
		want      []Bit
	}{
		{"empty positions", []Bit{1, 0, 1}, nil, []Bit{1, 0, 1}},
		{"remove first", []Bit{1, 0, 1}, []int{0}, []Bit{0, 1}},
		{"remove middle", []Bit{1, 0, 1}, []int{1}, []Bit{1, 1}},
		{"remove all", []Bit{1, 0}, []int{0, 1}, []Bit{}},
		{"duplicates removed once", []Bit{1, 0, 1}, []int{2, 2}, []Bit{1, 0}},
		{"unsorted positions", []Bit{0, 1, 0, 1}, []int{3, 0}, []Bit{1, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removePositions(tt.key, tt.positions)
			if len(got) != len(tt.want) {
				t.Fatalf("removePositions = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("removePositions = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestSampleSize(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		fraction float64
		want     int
	}{
		{"quarter of 2048", 2048, 0.25, 512},
		{"rounds to nearest", 10, 0.25, 3}, // 2.5 rounds up
		{"zero fraction", 100, 0, 0},
		{"full fraction", 100, 1, 100},
		{"empty key", 0, 0.25, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sampleSize(tt.n, tt.fraction); got != tt.want {
				t.Errorf("sampleSize(%d, %v) = %d, want %d", tt.n, tt.fraction, got, tt.want)
			}
		})
	}
}
