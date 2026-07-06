DRAFT

# bb84-qkd: the eavesdropper is just another goroutine

bb84-qkd is a Go simulation of BB84, the original 1984 quantum key
distribution protocol. Alice and Bob grow a shared secret key over an
insecure quantum channel, and — the whole point of QKD — any eavesdropper
listening in leaves a measurable fingerprint instead of a silent copy. The
simulator models Alice, Bob, and an optional eavesdropper Eve as goroutines
that share no memory at all, talking only over channels: a `chan Qubit` for
the quantum transmission, and a struct of four typed channels for the public
classical discussion that follows.

The one clever idea is treating "no shared memory between physically
separated parties" as literally, mechanically true, not just a description.
Alice and Bob each own a private `*rand.Rand` and their own bits and bases;
nothing about either party is reachable from the other's code. A `Qubit`
itself is a two-field struct — `bit` and `basis` — with unexported fields,
so no party can read a value without calling `Measure`, which is exactly the
no-peeking constraint the physics demands, now enforced by the compiler
instead of a code review comment. And Eve slots in as a pipeline stage: when
she's enabled, Bob's receive end simply points at her output channel instead
of Alice's, and nothing else in the code changes. She has exactly the powers
the wire gives her — nothing more, nothing less.

Why it's cool: the payoff is a number you can watch move. Eve must measure
every photon she intercepts to learn anything, and she has to guess which of
two bases it was encoded in — she's wrong half the time, and a wrong guess
means she re-encodes the photon into the wrong state before forwarding it
(she can't clone the original; no-cloning forbids it). That wrong re-encoding
shows up as a coin-flip error even on the positions where Bob's basis
happened to match Alice's. Half wrong guesses times half wrong outcomes on
those positions works out to a **~25% error rate** in the sifted key for a
full intercept-resend attack — comfortably above the 11% abort threshold.
Run `go run ./cmd/bb84 -n 4096 -seed 42` for a clean ACCEPT, then add `-eve`
and watch the estimated QBER jump straight to ~23%, decision flipping to
ABORT.

It's a deliberately honest toy: no photon loss or detector noise, only
intercept-resend is modeled, and there's no privacy amplification after the
accept/abort call. But every physics claim it makes is backed by a test —
including a goroutine-leak check that caught a real injected leak during
review — which makes it a solid, small, correct place to see why quantum key
distribution actually works.
