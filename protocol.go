package bb84

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
)

// DefaultN is the default number of qubits Alice transmits in a run.
const DefaultN = 4096

// Config configures a protocol run. The zero value plus a Seed is valid:
// unset fields fall back to the documented defaults.
type Config struct {
	// N is the number of qubits Alice transmits. If 0, DefaultN is used.
	// Negative values are rejected by Run.
	N int
	// Seed seeds the deterministic random number generators. Each party
	// derives its own independent generator from it, so a run is fully
	// reproducible given (Config, Seed).
	Seed uint64
}

// withDefaults returns cfg with unset fields replaced by defaults.
func (cfg Config) withDefaults() Config {
	if cfg.N == 0 {
		cfg.N = DefaultN
	}
	return cfg
}

// Result summarizes a protocol run from the test bench's omniscient point
// of view: it aggregates both parties' private outputs so callers can
// verify the protocol. In a real deployment each party would know only its
// own key.
type Result struct {
	// N is the number of qubits transmitted.
	N int
	// AliceSifted is Alice's sifted key: her encoded bits at the
	// positions where Bob's measurement basis matched hers.
	AliceSifted []Bit
	// BobSifted is Bob's sifted key: his measurement outcomes at the
	// same positions. With no eavesdropper and no channel noise it is
	// identical to AliceSifted.
	BobSifted []Bit
}

// Run executes one BB84 session: quantum transmission of cfg.N qubits from
// Alice to Bob followed by public basis reconciliation (sifting). It
// returns both parties' sifted keys.
//
// Alice and Bob run as goroutines communicating only over channels. Run
// waits for both to finish before returning — cancel ctx to shut a session
// down early — so no goroutines outlive the call.
func Run(ctx context.Context, cfg Config) (Result, error) {
	cfg = cfg.withDefaults()
	if cfg.N < 0 {
		return Result{}, fmt.Errorf("bb84: config N must be non-negative, got %d", cfg.N)
	}

	// Independent, deterministic randomness per party. The second PCG
	// word is a fixed per-party tag so the streams never coincide.
	aliceRNG := rand.New(rand.NewPCG(cfg.Seed, 0xA11CE))
	bobRNG := rand.New(rand.NewPCG(cfg.Seed, 0xB0B))

	// ctx is fanned out so that one party failing (or the caller
	// cancelling) unblocks everyone; otherwise the counterpart could wait
	// forever on a classical message that will never come.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	quantum := make(chan Qubit)
	cc := newClassicalChannel()

	// Each party reports its outcome on its own buffered channel, so even
	// party results travel over channels rather than shared variables. On
	// error the reporting party cancels the shared context so its
	// counterpart cannot block forever on an exchange that will never
	// happen.
	type partyResult struct {
		key []Bit
		err error
	}
	aliceOut := make(chan partyResult, 1)
	bobOut := make(chan partyResult, 1)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		key, err := alice(ctx, cfg.N, aliceRNG, quantum, cc)
		if err != nil {
			cancel()
		}
		aliceOut <- partyResult{key, err}
	}()
	go func() {
		defer wg.Done()
		key, err := bob(ctx, bobRNG, quantum, cc)
		if err != nil {
			cancel()
		}
		bobOut <- partyResult{key, err}
	}()
	wg.Wait()

	a, b := <-aliceOut, <-bobOut
	if err := errors.Join(a.err, b.err); err != nil {
		return Result{}, err
	}
	return Result{
		N:           cfg.N,
		AliceSifted: a.key,
		BobSifted:   b.key,
	}, nil
}
