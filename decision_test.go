package bb84

import (
	"context"
	"math"
	"runtime"
	"testing"
	"time"
)

func TestRunDecision(t *testing.T) {
	tests := []struct {
		name         string
		cfg          Config
		wantAccepted bool
	}{
		{
			name:         "no Eve accepts at default threshold",
			cfg:          Config{N: 4096, Seed: 1},
			wantAccepted: true,
		},
		{
			name:         "full Eve aborts at default threshold",
			cfg:          Config{N: 4096, Seed: 1, Eve: true},
			wantAccepted: false,
		},
		{
			name:         "full Eve accepted under an absurdly lax threshold",
			cfg:          Config{N: 4096, Seed: 1, Eve: true, QBERThreshold: 0.9},
			wantAccepted: true,
		},
		{
			// f = 0.25 → QBER ≈ 6.25%, safely below the 11% default
			// (sample ≈ 512, σ ≈ 1.1 percentage points; seeded anyway).
			name:         "quarter Eve slips under the default threshold",
			cfg:          Config{N: 4096, Seed: 1, Eve: true, EveFraction: 0.25},
			wantAccepted: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Run(context.Background(), tt.cfg)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if res.Accepted != tt.wantAccepted {
				t.Errorf("Accepted = %v (QBER %.4f), want %v", res.Accepted, res.QBER, tt.wantAccepted)
			}
			if tt.wantAccepted {
				if len(res.AliceKey) == 0 || len(res.BobKey) == 0 {
					t.Error("accepted run has empty final keys")
				}
			} else {
				if res.AliceKey != nil || res.BobKey != nil {
					t.Error("aborted run kept final keys; they must be discarded")
				}
				if res.QBER <= DefaultQBERThreshold {
					t.Errorf("aborted with QBER %.4f ≤ threshold %.2f", res.QBER, DefaultQBERThreshold)
				}
			}
		})
	}
}

func TestPartyOutputDecide(t *testing.T) {
	tests := []struct {
		name         string
		qber         float64
		threshold    float64
		wantAccepted bool
	}{
		{"zero QBER accepts", 0, 0.11, true},
		{"QBER below threshold accepts", 0.10, 0.11, true},
		{"QBER exactly at threshold accepts", 0.11, 0.11, true},
		{"QBER above threshold aborts", 0.12, 0.11, false},
		{"any error aborts under tiny threshold", 0.01, 1e-12, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := partyOutput{key: []Bit{1, 0}, qber: tt.qber}.decide(tt.threshold)
			if out.accepted != tt.wantAccepted {
				t.Errorf("decide(%v) with QBER %v: accepted = %v, want %v",
					tt.threshold, tt.qber, out.accepted, tt.wantAccepted)
			}
			if !tt.wantAccepted && out.key != nil {
				t.Error("aborted output kept its key")
			}
		})
	}
}

// goroutineCount reports the current goroutine count after giving any
// stragglers a moment to unwind, polling until the count stabilizes at or
// below want, or the deadline passes.
func goroutineCount(want int) int {
	deadline := time.Now().Add(2 * time.Second)
	for {
		runtime.GC() // nudge finalizers, then let the scheduler settle
		n := runtime.NumGoroutine()
		if n <= want || time.Now().After(deadline) {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunLeaksNoGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()

	// Completed runs, with and without Eve.
	for seed := uint64(0); seed < 5; seed++ {
		if _, err := Run(context.Background(), Config{N: 1024, Seed: seed, Eve: seed%2 == 0}); err != nil {
			t.Fatalf("Run(seed=%d): %v", seed, err)
		}
	}

	// A run cancelled mid-flight.
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := Run(ctx, Config{N: 1 << 22, Seed: 9, Eve: true})
		errCh <- err
	}()
	time.Sleep(20 * time.Millisecond) // let the transmission get going
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("cancelled Run returned nil error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled Run did not return")
	}

	if after := goroutineCount(before); after > before {
		t.Errorf("goroutine leak: %d before, %d after", before, after)
	}
}

func TestRunAbortStillReportsEstimates(t *testing.T) {
	res, err := Run(context.Background(), Config{N: 8192, Seed: 3, Eve: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Accepted {
		t.Fatalf("full Eve run accepted (QBER %.4f)", res.QBER)
	}
	// Diagnostics must survive the abort so the operator can see why.
	if res.SiftedLen == 0 || res.SampleSize == 0 {
		t.Errorf("aborted run lost diagnostics: SiftedLen=%d SampleSize=%d", res.SiftedLen, res.SampleSize)
	}
	if math.Abs(res.QBER-0.25) > 0.06 {
		t.Errorf("QBER = %.4f, want ≈ 0.25", res.QBER)
	}
}
