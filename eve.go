package bb84

import (
	"context"
	"fmt"
	"math/rand/v2"
)

// eve runs an intercept-resend eavesdropper on the quantum channel.
//
// For each qubit arriving on in she decides — with probability fraction —
// whether to attack it. An attacked qubit is measured in a random basis
// (the only way to learn anything, and Eve cannot clone the state first),
// and a fresh qubit encoding her outcome in her basis is forwarded;
// unattacked qubits pass through untouched. Because her basis guess is
// wrong half the time, and a wrong guess randomizes Bob's matching-basis
// measurement, a fully intercepted stream shows up as a ~25% error rate in
// the sifted key.
//
// eve closes out when in is closed. Like the honest parties she owns her
// rng and shares no memory with anyone; she only touches what travels on
// the channels.
func eve(ctx context.Context, fraction float64, rng *rand.Rand, in <-chan Qubit, out chan<- Qubit) error {
	defer close(out)
	for {
		select {
		case q, ok := <-in:
			if !ok {
				return nil
			}
			if rng.Float64() < fraction {
				basis := randomBasis(rng)
				bit := q.Measure(basis, rng)
				q = NewQubit(bit, basis)
			}
			if err := send(ctx, out, q); err != nil {
				return fmt.Errorf("eve: forwarding: %w", err)
			}
		case <-ctx.Done():
			return fmt.Errorf("eve: interception: %w", ctx.Err())
		}
	}
}
