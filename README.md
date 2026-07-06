# bb84-qkd

A BB84 quantum key distribution simulator in Go — Alice, Bob, and an optional
eavesdropper Eve as concurrent goroutines communicating only over channels.

Work in progress; milestones land via pull requests:

1. **M1** — Qubit type, Alice/Bob goroutines over a quantum channel, sifting.
2. **M2** — Eve (intercept-resend), error estimation, QBER.
3. **M3** — Abort decision, `cmd/bb84` CLI, clean-shutdown guarantees.

License: MIT.
