# Talk Outline: "The Eavesdropper Is Just Another Goroutine — Simulating Quantum Key Distribution with Go Channels"

*Conference-talk outline (25–30 min slot + Q&A) for the bb84-qkd project —
https://github.com/Pisush/bb84-qkd. Fits a Go conference (GopherCon-style)
audience: no physics background assumed, working Go knowledge assumed.*

## Abstract (for the CFP, ~120 words)

Quantum key distribution sounds like it needs a physics degree and a
dilution refrigerator. It needs neither — BB84, the original 1984 protocol,
can be understood in one slide and simulated in ~300 lines of Go. Better:
Go's concurrency model is a natural fit, because BB84's security *is* an
isolation property. Alice, Bob, and the eavesdropper Eve become goroutines;
the quantum channel becomes a `chan Qubit`; "no shared memory" stops being a
style rule and becomes the laws of physics. We'll build the simulator live
on slides, wire an intercept-resend attacker into the pipeline with one
line, and watch her stamp a detectable 25% error rate into the key. You'll
leave knowing how QKD works — and with a new appreciation for channel-close
semantics.

## Audience takeaways

1. How BB84 works, end to end, with zero linear algebra.
2. Why "share memory by communicating" is a *security architecture*, not
   just a style preference — and how to enforce isolation with unexported
   fields, channel direction types, and per-goroutine state.
3. Concrete patterns: pipeline stage injection (Eve), typed per-phase
   channels as protocol documentation, ctx-guarded send/recv helpers,
   deterministic seeded concurrency, goroutine-leak testing.

## Outline

### 1. Cold open (2 min)
- Live demo, nothing explained yet: `bb84 -n 4096 -seed 42` → ACCEPT;
  add `-eve` → QBER 23%, ABORT.
- "We just caught a wiretapper. With a channel of structs. Let me show you."

### 2. The one slide of physics (4 min)
- A photon can encode a bit in one of two "orientations" (bases): + or ×.
- Rule 1: measure in the right basis → correct bit; wrong basis → coin
  flip, and the original state is destroyed.
- Rule 2 (no-cloning): you cannot copy an unknown quantum state.
- Consequence: anyone who reads the stream *must* damage it. Security by
  physics, not by hard math problems.
- Speaker note: explicitly say "no complex numbers today; the four BB84
  states make them redundant" — this disarms the physics anxiety early.

### 3. Modeling the qubit in Go (4 min)
- `type Qubit struct { bit Bit; basis Basis }` — and *why* this is exact,
  not a cheat, for BB84's four states (Born rule reduces to two cases).
- Unexported fields = no party reads a bit without `Measure`. The compiler
  enforces the no-peeking rule. Type system as physics engine.
- `Measure(b Basis, rng *rand.Rand) Bit` — injectable randomness; every
  "quantum" coin flip is reproducible with a seed. Demo: same seed, same
  universe.

### 4. The protocol as channel choreography (6 min)
- Cast: Alice, Bob = goroutines; quantum channel = `chan Qubit`; classical
  channel = struct of four typed channels (bases, matches, sample,
  mismatch count). Slide: the struct definition *is* the protocol spec.
- Phase 1 — transmission: N random (bit, basis) pairs; `close(ch)` as
  end-of-transmission. Bob measures in random bases as qubits arrive.
- Phase 2 — sifting: bases are compared publicly, ~50% of positions
  survive. Emphasize the invariant: *bit values never travel on the
  classical channel* (one exception coming).
- Phases 3+4 — error estimation and decision: sacrifice 25% of the sifted
  key publicly, compute QBER from two shared integers, abort above 11%.
- Concurrency pattern slide: ctx-guarded `send`/`recv` generics; any
  party's failure cancels a run-scoped context so nobody blocks forever;
  `Run` waits for every goroutine — no leaks by construction, verified by
  a mutation-tested leak test.

### 5. Enter Eve (6 min)
- Intercept-resend: measure in a random basis, re-encode *your guess*,
  forward. No-cloning means she can't do better without new physics.
- The Go payoff: Eve is a pipeline stage. One line rewires Bob's receive
  end to Eve's output; Alice's and Bob's code is untouched. She holds
  channel ends, nothing more — the attacker has exactly the powers the
  wire gives her.
- The arithmetic on one slide: P(wrong basis) × P(flip anyway) = ½ × ½ →
  25% QBER on sifted positions; partial interception fraction f → f/4.
- Live demo: `-eve-fraction` sweep 0 → 0.25 → 0.5 → 1.0; watch QBER climb
  0% → ~6% → ~12.5% → ~25% and the decision flip to ABORT past 11%.

### 6. What the tests pin down (3 min)
- "Every physics claim has a test": no Eve → QBER ≈ 0; full Eve → 25%
  within 5σ; partial Eve scales; decision boundary (spec says abort
  strictly above threshold — test the exact-equality edge).
- Seeded statistical tests: meaningful tolerances *and* zero flakes.
- Goroutine-leak test design: baseline + polling; caught an injected leak
  during code review (real anecdote).

### 7. Honest limitations + close (3 min)
- Noiseless channel (all errors are Eve's), one attack modeled, no privacy
  amplification. Real QKD engineering budgets QBER for hardware noise.
- Closing beat: BB84 turns "someone might be listening" into a number you
  can test in CI. Go turns the protocol diagram into a program whose
  every information flow is a channel you can point at.
- Repo + slides QR code.

### Q&A seeds (backup slides)
- Why not model amplitudes with complex128? (When you WOULD need them:
  other protocols, e.g. B92, six-state, entanglement-based E91.)
- What if Eve measures in a smarter basis (Breidbart)? → still ≥15% QBER;
  nice extension exercise.
- Photon loss / noise: how the threshold changes when not all error is
  attributable to Eve.
- Performance: how large an N before unbuffered channels dominate; why
  correctness-over-throughput is the right call for a simulator.

## Demo plan (all terminal, no slides-only claims)

1. Clean run, seed 42 → ACCEPT (30 s).
2. `-eve` → ABORT at ~23–25% QBER (30 s).
3. Fraction sweep loop in shell → QBER staircase (90 s).
4. `go test -race ./...` live as the "trust me" moment (30 s).
5. Backup: recorded asciinema in case of demo gods.

## Logistics

- Format: 25–30 min talk; can compress to 20 by cutting section 6 into
  section 5 and dropping demo 3.
- Also works as a 45-min workshop seed: attendees implement Eve themselves
  against the provided Alice/Bob (repo has the interfaces ready).
- Prior art check: several BB84 explainers exist; the goroutine/channel
  isolation framing and the "attacker as pipeline stage" angle are the
  novel hook for a Go audience.
