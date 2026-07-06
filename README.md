# bb84-qkd

A BB84 quantum key distribution simulator in Go — Alice, Bob, and an optional
eavesdropper Eve as concurrent goroutines communicating only over channels.

## The physics, in ~200 words

BB84 (Bennett & Brassard, 1984) lets two parties, Alice and Bob, grow a
shared secret key over an insecure line, with eavesdropping made *physically
detectable* rather than merely computationally hard.

Alice encodes each random bit into a photon using one of two randomly chosen
"bases" — orientations for the encoding, rectilinear `+` or diagonal `×`.
Quantum mechanics imposes two rules. First, measuring a photon in the same
basis it was prepared in reproduces the encoded bit perfectly, but measuring
in the other basis yields a coin flip *and destroys the original state*.
Second, an unknown quantum state cannot be copied (the no-cloning theorem),
so a spy cannot keep a photon and forward a duplicate.

Bob measures each arriving photon in a random basis. Afterwards, over a
public channel, the two compare *bases only* — never bit values — and keep
the ~50% of positions where the bases matched: the sifted key.

An eavesdropper, Eve, must measure to learn anything, guessing the basis.
She guesses wrong half the time, resending a disturbed photon that gives Bob
a wrong bit in a quarter of the sifted positions. By sacrificing a random
sample and comparing it publicly, Alice and Bob estimate this error rate —
roughly 25% under a full intercept-resend attack — and abort when it is too
high.

## Design notes

- **Qubits as (bit, basis).** BB84 only ever uses the four states |0⟩, |1⟩,
  |+⟩, |−⟩, each measured exactly once in one of two bases, so the Born rule
  reduces to "same basis → encoded bit, other basis → fair coin". A full
  2-amplitude state vector carries no extra information; `Qubit` stores the
  `(bit, basis)` pair with unexported fields, so no party can read a bit
  without measuring.
- **Physical separation via channels.** Alice and Bob are goroutines with no
  shared memory; the quantum channel is a `chan Qubit`, and the public
  discussion travels on a separate classical channel that only ever carries
  basis information — with one protocol-sanctioned exception, the sacrificed
  error-check sample. When Eve is enabled the quantum channel routes through
  her goroutine; she never touches anything but what travels on the wire.
- **Deterministic randomness.** Every stochastic step draws from a per-party
  `*rand.Rand` derived from a single seed, so runs are reproducible.
- **Clean shutdown.** `Run` waits for all party goroutines before returning
  and honors context cancellation, so no goroutines leak.

## Usage

### CLI

```sh
$ go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42
BB84 run (seed 42)
  qubits sent        4096
  eavesdropper       off
  sifted key length  2073 (50.6% of sent)
  sacrificed sample  518 bits
  estimated QBER     0.0000 (abort threshold 0.1100)
  decision           ACCEPT — final key length 1555

$ go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42 -eve
BB84 run (seed 42)
  qubits sent        4096
  eavesdropper       intercept-resend, fraction 1.00
  sifted key length  2002 (48.9% of sent)
  sacrificed sample  501 bits
  estimated QBER     0.2315 (abort threshold 0.1100)
  decision           ABORT — channel is not trustworthy, key discarded
```

Flags: `-n` qubits (default 4096), `-eve` enable the eavesdropper,
`-eve-fraction` how much of the stream Eve attacks (default 1.0),
`-seed` for reproducible runs (0 = clock-derived), `-sample-fraction`
(default 0.25), `-threshold` abort threshold (default 0.11). Exit codes:
0 accept, 1 abort, 2 error.

### Library

```go
res, err := bb84.Run(context.Background(), bb84.Config{N: 4096, Seed: 42, Eve: true})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("QBER %.4f, accepted=%v, key length %d\n", res.QBER, res.Accepted, len(res.AliceKey))
```

## Limitations

- Idealized channel: no photon loss, dark counts, detector inefficiency, or
  channel noise — every error is attributable to Eve.
- Intercept-resend is the only attack modeled; no photon-number-splitting,
  entangling, or collective attacks.
- No privacy amplification or information reconciliation (cascade, LDPC);
  the run ends after the abort/accept decision.
- The classical channel is assumed authenticated, as BB84 requires.

License: MIT.
