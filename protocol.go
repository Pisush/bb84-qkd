package bb84

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
)

// Defaults for Config fields left at their zero value.
const (
	// DefaultN is the default number of qubits Alice transmits in a run.
	DefaultN = 4096
	// DefaultSampleFraction is the default fraction of sifted bits
	// sacrificed for error estimation.
	DefaultSampleFraction = 0.25
	// DefaultEveFraction is the fraction of qubits Eve intercepts when
	// she is enabled and no fraction is given: all of them.
	DefaultEveFraction = 1.0
)

// Config configures a protocol run. The zero value plus a Seed is valid:
// unset fields fall back to the documented defaults.
type Config struct {
	// N is the number of qubits Alice transmits. If 0, DefaultN is used.
	// Negative values are rejected by Run.
	N int
	// Seed seeds the deterministic random number generators. Each party
	// derives its own independent generator from it, so a run is fully
	// reproducible given Config.
	Seed uint64
	// Eve, when true, routes the quantum channel through an
	// intercept-resend eavesdropper.
	Eve bool
	// EveFraction is the probability that Eve attacks any given qubit,
	// in [0, 1]. It is only meaningful when Eve is true. If 0 it
	// defaults to DefaultEveFraction (a full intercept-resend attack);
	// to model no eavesdropping, leave Eve false.
	EveFraction float64
	// SampleFraction is the fraction of the sifted key sacrificed for
	// error estimation, in [0, 1]. If 0, DefaultSampleFraction is used.
	SampleFraction float64
}

// withDefaults returns cfg with unset fields replaced by defaults.
func (cfg Config) withDefaults() Config {
	if cfg.N == 0 {
		cfg.N = DefaultN
	}
	if cfg.SampleFraction == 0 {
		cfg.SampleFraction = DefaultSampleFraction
	}
	if cfg.Eve && cfg.EveFraction == 0 {
		cfg.EveFraction = DefaultEveFraction
	}
	return cfg
}

// validate rejects out-of-range fields. It runs after withDefaults.
func (cfg Config) validate() error {
	if cfg.N < 0 {
		return fmt.Errorf("bb84: config N must be non-negative, got %d", cfg.N)
	}
	if cfg.SampleFraction < 0 || cfg.SampleFraction > 1 {
		return fmt.Errorf("bb84: config SampleFraction must be in [0,1], got %v", cfg.SampleFraction)
	}
	if cfg.EveFraction < 0 || cfg.EveFraction > 1 {
		return fmt.Errorf("bb84: config EveFraction must be in [0,1], got %v", cfg.EveFraction)
	}
	return nil
}

// Result summarizes a protocol run from the test bench's omniscient point
// of view: it aggregates both parties' private outputs so callers can
// verify the protocol. In a real deployment each party would know only its
// own key and the public discussion.
type Result struct {
	// N is the number of qubits transmitted.
	N int
	// SiftedLen is the length of the sifted key (before the error-check
	// sample was sacrificed).
	SiftedLen int
	// SampleSize is how many sifted bits were publicly compared, and
	// therefore discarded from the final keys.
	SampleSize int
	// QBER is the quantum bit error rate estimated from the sacrificed
	// sample: mismatches / SampleSize (0 if SampleSize is 0). Both
	// parties compute the same value from the public discussion. With no
	// eavesdropper and a noiseless channel it is 0; a full
	// intercept-resend attack pushes it to ~25%.
	QBER float64
	// AliceSifted and BobSifted are the parties' sifted keys, before the
	// sample was removed. With no eavesdropper they are identical.
	AliceSifted []Bit
	BobSifted   []Bit
	// AliceKey and BobKey are the final keys: the sifted keys with the
	// sacrificed sample removed. They are what a real deployment would
	// feed into information reconciliation and privacy amplification.
	AliceKey []Bit
	BobKey   []Bit
}

// Run executes one BB84 session: quantum transmission of cfg.N qubits from
// Alice to Bob (optionally routed through Eve), public basis
// reconciliation (sifting), and public error estimation on a sacrificed
// sample.
//
// Alice, Bob and Eve run as goroutines communicating only over channels.
// Run waits for all of them to finish before returning — cancel ctx to
// shut a session down early — so no goroutines outlive the call.
func Run(ctx context.Context, cfg Config) (Result, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return Result{}, err
	}

	// Independent, deterministic randomness per party. The second PCG
	// word is a fixed per-party tag so the streams never coincide.
	aliceRNG := rand.New(rand.NewPCG(cfg.Seed, 0xA11CE))
	bobRNG := rand.New(rand.NewPCG(cfg.Seed, 0xB0B))
	eveRNG := rand.New(rand.NewPCG(cfg.Seed, 0xE7E))

	// ctx is fanned out so that one party failing (or the caller
	// cancelling) unblocks everyone; otherwise a counterpart could wait
	// forever on a message that will never come.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cc := newClassicalChannel()

	// The quantum channel: Alice's photons either reach Bob directly or
	// are routed through Eve.
	var wg sync.WaitGroup
	aliceQuantum := make(chan Qubit)
	bobQuantum := (<-chan Qubit)(aliceQuantum)
	eveErr := make(chan error, 1)
	if cfg.Eve {
		eveOut := make(chan Qubit)
		bobQuantum = eveOut
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := eve(ctx, cfg.EveFraction, eveRNG, aliceQuantum, eveOut)
			if err != nil {
				cancel()
			}
			eveErr <- err
		}()
	} else {
		eveErr <- nil
	}

	// Each party reports its outcome on its own buffered channel, so even
	// party results travel over channels rather than shared variables. On
	// error the reporting party cancels the shared context so its
	// counterparts cannot block forever on an exchange that will never
	// happen.
	type partyResult struct {
		out partyOutput
		err error
	}
	aliceOut := make(chan partyResult, 1)
	bobOut := make(chan partyResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		out, err := alice(ctx, cfg.N, cfg.SampleFraction, aliceRNG, aliceQuantum, cc)
		if err != nil {
			cancel()
		}
		aliceOut <- partyResult{out, err}
	}()
	go func() {
		defer wg.Done()
		out, err := bob(ctx, bobRNG, bobQuantum, cc)
		if err != nil {
			cancel()
		}
		bobOut <- partyResult{out, err}
	}()
	wg.Wait()

	a, b := <-aliceOut, <-bobOut
	if err := errors.Join(a.err, b.err, <-eveErr); err != nil {
		return Result{}, err
	}
	return Result{
		N:           cfg.N,
		SiftedLen:   len(a.out.sifted),
		SampleSize:  a.out.sampleSize,
		QBER:        a.out.qber,
		AliceSifted: a.out.sifted,
		BobSifted:   b.out.sifted,
		AliceKey:    a.out.key,
		BobKey:      b.out.key,
	}, nil
}
