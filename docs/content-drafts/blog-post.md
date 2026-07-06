# Draft: Catching a Quantum Eavesdropper with Go Channels

*Blog-post draft for the bb84-qkd project — https://github.com/Pisush/bb84-qkd*

---

In 1984, Bennett and Brassard described a protocol that turns eavesdropping
from a silent threat into a measurable event. Forty-odd years later, BB84 is
still the "hello world" of quantum cryptography — and it turns out Go's
concurrency model is an almost suspiciously good fit for simulating it.

This post walks through building a BB84 simulator where Alice, Bob, and the
eavesdropper Eve are goroutines, the quantum channel is a `chan Qubit`, and
the laws of physics are enforced by the type system.

## Why BB84 maps onto goroutines

BB84's security rests on physical separation: Alice and Bob are distinct
systems that share *nothing* except what travels over two links — a quantum
channel carrying photons and a public classical channel carrying discussion.
Eve can touch the wire, but she cannot peek inside Alice's lab.

That is exactly the discipline Go asks of us anyway: *don't communicate by
sharing memory; share memory by communicating.* So the simulator takes the
slogan literally. Each party is a goroutine that owns its state and its own
seeded RNG. There is no struct both Alice and Bob can reach. If information
moves, it moves through a channel — and you can audit every way information
can flow by reading four channel declarations.

## A qubit without complex numbers

The first design decision is what a "qubit" is. BB84 only ever prepares four
states — |0⟩, |1⟩ in the rectilinear basis, |+⟩, |−⟩ in the diagonal one —
and each photon is measured exactly once. For that specific menu, the Born
rule collapses to two sentences: measure in the basis it was prepared in and
you get the encoded bit with certainty; measure in the other basis and you
get a fair coin flip. A two-amplitude complex state vector would carry no
information beyond the pair (bit, basis), so that pair is the whole type:

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

The fields are unexported, and that is load-bearing: *no party can read a bit
without measuring.* Bob physically cannot cheat, because the compiler won't
let him. It's a small thing, but it means the "no cloning, no peeking" rule
of quantum mechanics is enforced at compile time rather than by code review.

## The protocol as channel choreography

A run is four phases, and each phase is one or two channel operations:

1. **Quantum transmission.** Alice draws N random bits and N random bases,
   encodes each pair, and sends N qubits down `chan Qubit`. She closes the
   channel after the last one — channel-close *is* the end-of-transmission
   marker. Bob measures each arrival in a freshly drawn random basis.
2. **Sifting.** Bob announces his basis choices on the classical channel;
   Alice replies with the positions where his basis matched hers. Both keep
   just those bits — about half. Note what never crossed the wire: bit
   values. Only bases and positions are public.
3. **Error estimation.** Alice sacrifices a random 25% of the sifted key,
   announcing positions *and bit values* — the protocol's single sanctioned
   disclosure. Bob replies with the number of mismatches. Both sides now
   hold the same estimate of the quantum bit error rate (QBER), computed
   from identical public integers.
4. **Decision.** If QBER exceeds a threshold (11% by default), both parties
   independently abort and discard everything. Otherwise the unsacrificed
   bits become the shared key.

The classical channel isn't one `chan interface{}` — it's a struct of four
typed channels, one per phase message. The types document the protocol: you
can read the whole public discussion off the struct definition.

```go
type classicalChannel struct {
    bases      chan []Basis            // Bob → Alice
    matches    chan []int              // Alice → Bob
    sample     chan sampleAnnouncement // Alice → Bob
    mismatches chan int                // Bob → Alice
}
```

## Eve is just another goroutine

The intercept-resend attack is beautifully simple to express: Eve sits
between Alice's channel and Bob's, and for each qubit she either passes it
through or measures-and-re-encodes:

```go
if rng.Float64() < fraction {
    basis := randomBasis(rng)
    bit := q.Measure(basis, rng)
    q = NewQubit(bit, basis) // she can't clone; she must resend her guess
}
out <- q
```

Wiring her in is one line in the orchestrator: when Eve is enabled, Bob's
receive end points at Eve's output instead of Alice's channel. Nobody else's
code changes. Eve doesn't get special powers — she gets a channel end, same
as everyone.

And here's the physics payoff. Eve must guess a basis; she's wrong half the
time. A wrong guess re-encodes the photon in the wrong basis, so even when
Bob's basis matches Alice's — the only positions that survive sifting — his
measurement is a coin flip. Half wrong guesses times half wrong outcomes:
**a full intercept-resend attack stamps a ~25% error rate into the sifted
key.** The 11% threshold catches it with absurd margin. Intercepting only a
fraction *f* scales the signature to *f*/4, and the tests verify both.

```text
$ bb84 -n 4096 -seed 42 -eve
  estimated QBER     0.2315 (abort threshold 0.1100)
  decision           ABORT — channel is not trustworthy, key discarded
```

Eavesdropping detected, in about 300 lines of Go.

## The unglamorous parts that make it hold up

**Determinism.** Every stochastic step — Alice's bits, everyone's bases,
Eve's coin, the sample selection — draws from a per-party `rand.Rand` seeded
from one master seed. Same seed, same run, bit for bit. Statistical tests
("QBER ≈ 25%") get fixed seeds and 5σ tolerances, so they are simultaneously
meaningful and non-flaky.

**Shutdown.** Any party can fail (in practice: context cancellation), and a
failed party must not strand its counterpart waiting on a message that will
never come. Every blocking channel op is wrapped in a select against
`ctx.Done()`, any party's error cancels a run-scoped context, and `Run`
waits for every goroutine before returning. A leak test runs sessions —
including one cancelled mid-flight at N=2²² — and checks the goroutine count
returns to baseline. (During review, the test was mutation-tested by
injecting a deliberately leaked goroutine; it caught it.)

**What travels where.** The invariant worth stating in every QKD codebase:
sifted-key bits never appear on the classical channel, except the sacrificed
sample, which is discarded *because* it was disclosed. With typed per-phase
channels, that claim is checkable by grep.

## Limitations, honestly

This is a pedagogical simulator. The channel is noiseless and lossless, so
every error is Eve's fault; real systems must budget QBER for detector noise
and photon loss. Only intercept-resend is modeled — no photon-number
splitting or collective attacks. And the run stops after the accept/abort
decision; real deployments continue into information reconciliation and
privacy amplification. All of which is to say: don't ship it to a bank, but
do use it to *see* why quantum key distribution works.

## Try it

```sh
go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42        # ACCEPT
go run github.com/Pisush/bb84-qkd/cmd/bb84@latest -n 4096 -seed 42 -eve  # ABORT
```

The repo — including the tests that pin the 25% signature — is at
[github.com/Pisush/bb84-qkd](https://github.com/Pisush/bb84-qkd). PRs and
nitpicks welcome, especially from physicists.

---

*~1,200 words. Suggested tags: golang, concurrency, quantum computing,
cryptography. Possible alternative titles: "The Eavesdropper Is Just Another
Goroutine", "BB84 in 300 Lines of Go", "Share Photons by Communicating".*
