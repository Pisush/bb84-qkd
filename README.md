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
  basis information (plus, in later milestones, the sacrificed error-check
  sample).
- **Deterministic randomness.** Every stochastic step draws from a per-party
  `*rand.Rand` derived from a single seed, so runs are reproducible.
- **Clean shutdown.** `Run` waits for all party goroutines before returning
  and honors context cancellation, so no goroutines leak.

## Usage

```go
res, err := bb84.Run(context.Background(), bb84.Config{N: 4096, Seed: 42})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("sifted key length: %d\n", len(res.AliceSifted))
```

A `cmd/bb84` CLI (flags `-n`, `-eve`, `-eve-fraction`, `-seed`) arrives in
milestone M3.

## Milestones

1. **M1** — Qubit type, Alice/Bob goroutines over a quantum channel, sifting.
2. **M2** — Eve (intercept-resend), error estimation, QBER.
3. **M3** — Abort decision, `cmd/bb84` CLI, clean-shutdown guarantees.

## Limitations

- Idealized channel: no photon loss, dark counts, detector inefficiency, or
  channel noise — every error is attributable to Eve.
- Intercept-resend is the only attack modeled; no photon-number-splitting,
  entangling, or collective attacks.
- No privacy amplification or information reconciliation (cascade, LDPC);
  the run ends after the abort/accept decision.
- The classical channel is assumed authenticated, as BB84 requires.

License: MIT.
