---
marp: true
theme: default
paginate: true
---

# bb84-qkd
### The eavesdropper is just another goroutine

github.com/Pisush/bb84-qkd

<!-- notes: Cold open with a live demo if possible: clean run -> ACCEPT,
then add -eve -> ABORT at ~23% QBER. Say almost nothing else yet; let the
demo raise the question the rest of the talk answers. -->

---

# What BB84 is trying to do

- Alice and Bob want a shared secret key over a channel that might be
  tapped.
- Classical crypto: eavesdropping is computationally hard to get away
  with.
- BB84 (Bennett & Brassard, 1984): eavesdropping is **physically
  detectable**.

<!-- notes: This distinction -- hard vs detectable -- is the whole reason
QKD is interesting. Land it before any Go content. -->

---

# Two rules from quantum mechanics

- **Rule 1:** measure a photon in the basis it was prepared in -> correct
  bit. Measure in the *other* basis -> coin flip, and the original state
  is destroyed.
- **Rule 2 (no-cloning):** you cannot copy an unknown quantum state.
- Consequence: anyone who reads the stream must disturb it.

<!-- notes: No complex numbers needed for this talk -- say so explicitly,
it disarms physics anxiety early. -->

---

# The cast, as Go concurrency primitives

- Alice = goroutine. Bob = goroutine. Eve (optional) = goroutine.
- Quantum channel = `chan Qubit`.
- Public classical channel = a struct of four typed channels.
- No shared memory between any of them, ever.

<!-- notes: "Share memory by communicating" stops being a style preference
here and becomes the actual security architecture. -->

---

# A qubit without complex numbers

```go
type Qubit struct {
    bit   Bit
    basis Basis
}

func (q Qubit) Measure(b Basis, rng *rand.Rand) Bit {
    if b == q.basis {
        return q.bit
    }
    return Bit(rng.IntN(2))
}
```

- BB84 only prepares 4 states, each measured once — Born rule collapses
  to 2 cases.

<!-- notes: Explain why this is exact, not a simplification, for this
specific protocol -- a full complex amplitude vector would carry zero
extra information here. -->

---

# Unexported fields enforce the physics

- `bit` and `basis` are unexported.
- No party can read a value without calling `Measure`.
- The no-peeking rule is enforced by the **compiler**, not code review.

<!-- notes: Small detail, big payoff -- Bob physically cannot cheat because
there's no code path that lets him read the field directly. -->

---

# Phase 1: quantum transmission

- Alice draws N random bits and N random bases, sends N `Qubit`s.
- `close(quantum)` after the last one — channel-close *is* the
  end-of-transmission signal.
- Bob measures each arrival in a freshly drawn random basis.

<!-- notes: Point out close() doing double duty as a protocol signal, not
just cleanup -- a nice small Go idiom doing real work. -->

---

# Phase 2: sifting

- Bob announces his basis choices publicly.
- Alice replies with the *positions* where bases matched (~50%).
- Both keep only their bits at those positions.
- **Invariant: bit values never cross the classical channel here.**

<!-- notes: Emphasize the invariant -- only bases and positions are public
in this phase. One exception is coming in phase 3. -->

---

# The classical channel, typed

```go
type classicalChannel struct {
    bases      chan []Basis            // Bob -> Alice
    matches    chan []int              // Alice -> Bob
    sample     chan sampleAnnouncement // Alice -> Bob
    mismatches chan int                // Bob -> Alice
}
```

- The struct definition **is** the protocol specification.
- Every field: one phase, one direction, one message shape.

<!-- notes: You can audit the entire public discussion by reading four
field declarations. That's the payoff of modeling it this explicitly. -->

---

# Phase 3+4: error estimation and decision

- Alice sacrifices a random sample of the sifted key (25% by default) —
  announces positions **and** bit values. The one sanctioned exception.
- Bob replies with the mismatch count.
- Both compute the same QBER from two shared public integers.
- QBER > 11% (default threshold) -> both abort, independently.

<!-- notes: The sacrificed bits are safe to disclose precisely because
they're immediately discarded from the key afterward. -->

---

# Enter Eve

```go
if rng.Float64() < fraction {
    basis := randomBasis(rng)
    bit := q.Measure(basis, rng)
    q = NewQubit(bit, basis) // can't clone; must resend her guess
}
out <- q
```

- Intercept-resend: measure in a guessed basis, re-encode the guess,
  forward.
- No-cloning means this is the best she can do.

<!-- notes: She can't peek and forward the original -- physics forbids
cloning an unknown state, so measuring is her only option, and measuring
disturbs it. -->

---

# Wiring her in: one line

- Eve enabled -> Bob's receive end points at Eve's output channel instead
  of Alice's.
- Alice's code: unchanged. Bob's code: unchanged.
- Eve has exactly the powers the wire gives her — a channel end, nothing
  more.

<!-- notes: This is the Go payoff of the whole talk -- the attacker is a
pipeline stage you insert, not a special case threaded through everyone
else's logic. -->

---

# Why ~25%

- Eve guesses a basis: wrong half the time.
- Wrong guess -> re-encodes in the wrong basis -> even where Bob's basis
  matches Alice's original, his measurement is now a coin flip.
- 1/2 (wrong guess) x 1/2 (coin flip) = **1/4 of sifted positions wrong.**
- Partial interception, fraction f -> QBER scales to roughly f/4.

<!-- notes: Walk this arithmetic slowly -- it's the "aha" moment of the
whole talk. The 11% threshold catches full interception with enormous
margin. -->

---

# Let's run it

```text
$ go run ./cmd/bb84 -n 4096 -seed 42
  estimated QBER     0.0000 (abort threshold 0.1100)
  decision           ACCEPT — final key length 1555

$ go run ./cmd/bb84 -n 4096 -seed 42 -eve
  estimated QBER     0.2315 (abort threshold 0.1100)
  decision           ABORT — channel is not trustworthy, key discarded
```

<!-- notes: Same seed both runs -- deterministic RNG per party means these
exact numbers reproduce live, not cherry-picked screenshots. -->

---

# Determinism and typed messages, together

- Every stochastic step draws from a per-party `*rand.Rand` derived from
  one master seed.
- Same seed -> same run, bit for bit -> statistical tests get tight
  tolerances *and* zero flakes.
- No Eve: QBER ~ 0. Full Eve: QBER ~ 25% (tested to 5 sigma).

<!-- notes: Determinism is what makes "QBER approx 25%" a testable claim
instead of a hand-wave. -->

---

# No goroutine leaks, by construction

- Every channel send/recv is wrapped in a `select` against a cancellable
  `ctx`.
- Any party's error cancels a run-scoped context — nobody blocks forever.
- `Run` waits for every goroutine before returning.
- Leak test: baseline goroutine count, run sessions (including one
  cancelled mid-flight), confirm return to baseline.

<!-- notes: Anecdote: a deliberately injected leak was caught by this test
during review -- good proof the test earns its keep. -->

---

# Honest limitations

- Idealized channel: no photon loss, dark counts, or detector noise —
  every error is attributable to Eve.
- Only intercept-resend modeled — no photon-number-splitting or
  collective attacks.
- No privacy amplification or information reconciliation after the
  decision.

<!-- notes: This is a pedagogical simulator, not production QKD -- say so
plainly, then pivot to why that's still the right scope for seeing the
protocol work. -->

---

# Try it yourself

```sh
go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42
go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42 -eve
go test -race ./...
```

- Repo: github.com/Pisush/bb84-qkd
- "Share photons by communicating."

<!-- notes: Closing line, repo link, open for Q&A. -->
