DRAFT — podcast script for bb84-qkd (github.com/Pisush/bb84-qkd)

# Podcast script: "The Eavesdropper Is Just Another Goroutine"

*Two hosts. HOST A is a curious generalist — comfortable with Go, no
cryptography or physics background. HOST B is the builder of bb84-qkd and
knows the codebase cold. Target runtime: ~15-18 minutes.*

---

**HOST A:** So we're doing quantum key distribution today, and I want to
start with the thing that got me — the title you gave this talk. "The
eavesdropper is just another goroutine." That's a wild sentence. Unpack it.

**HOST B:** It's the literal architecture of the simulator. BB84 — that's
the 1984 protocol from Bennett and Brassard, the original quantum key
distribution scheme — has three roles: Alice sending, Bob receiving, and
optionally Eve listening in. In this codebase, Alice is a goroutine, Bob is
a goroutine, and when you enable Eve, she's a goroutine too, sitting between
Alice's output channel and Bob's input channel. She's not special. She's
just another thing reading from and writing to a `chan Qubit`.

**HOST A:** Okay, but before we get to Eve — what is BB84 actually trying to
do? Pretend I've never heard of quantum key distribution.

**HOST B:** Two people, Alice and Bob, want to agree on a secret key over a
channel that might be tapped. The classical answer is math-hard problems —
factoring, discrete log — where breaking it isn't impossible, just
expensive. BB84's answer is different: it makes eavesdropping *physically
detectable*. Not computationally hard to get away with — physically
impossible to get away with cleanly.

**HOST A:** How does a photon make eavesdropping detectable?

**HOST B:** Two rules from quantum mechanics. First: Alice encodes each bit
using one of two "bases" — think of them as two different orientations,
rectilinear or diagonal. If you measure a photon in the same basis it was
prepared in, you get the bit back perfectly. Measure it in the *other*
basis, and you get a coin flip, and — this is the important part — the
original state is destroyed by the act of measuring. Second rule: the
no-cloning theorem. You cannot copy an unknown quantum state. So Eve can't
just siphon off a duplicate and send the original along untouched. She has
to touch the real photon, and touching it risks breaking it.

**HOST A:** So she's forced to gamble.

**HOST B:** Exactly. She has to guess a basis to measure in, same as Bob
does. She's right half the time and wrong half the time, and we'll get to
exactly what that costs her.

**HOST A:** Let's talk about how you represented a qubit in Go, because I
expected complex numbers and I didn't find any.

**HOST B:** That surprised me too when I sat down to design it, and then it
made total sense. BB84 only ever prepares four specific states, and each
photon gets measured exactly once. For that narrow menu, the underlying
quantum probability rule — the Born rule — collapses down to exactly two
cases: same basis, you get the bit for certain; different basis, you get a
fair coin flip. A full complex-amplitude state vector would carry zero
extra information over just storing the pair. So that's what `Qubit` is:

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

**HOST A:** Those fields are unexported. Is that just Go style, or does it
matter?

**HOST B:** It matters a lot, actually — it's load-bearing. Because the
fields are unexported, no party, not Alice, not Bob, not Eve, can read a
qubit's bit or basis without calling `Measure`. The no-peeking rule that
quantum mechanics enforces with physics, this type enforces with the
compiler. Bob can't cheat by reaching into a `Qubit` and reading the bit
directly — there's no way to write that code. It's a small thing, but I
love that the type system is doing security work here, not just organizing
data.

**HOST A:** Walk me through an actual run. What happens end to end?

**HOST B:** Four phases. Phase one, quantum transmission: Alice draws N
random bits and N random bases, encodes each pair as a `Qubit`, and sends
all N down a `chan Qubit`, closing the channel when she's done — the
channel close *is* the end-of-transmission signal. Bob measures each
arriving qubit in a freshly drawn random basis of his own.

**HOST A:** And at that point neither of them knows if they used the same
basis for any given photon.

**HOST B:** Right, that's phase two — sifting, and it happens on a
completely separate classical channel. Bob announces his basis choices
publicly. Alice compares them against her own and announces back the
*positions* where the bases matched — about half of them, on average. Both
sides keep only their bits at those matching positions. And here's the
invariant that matters: bit values never travel on that public channel.
Only bases, and only positions. Nobody broadcasts the actual secret.

**HOST A:** Is that classical channel one generic pipe, or does the type
system do something interesting there too?

**HOST B:** It's not one generic pipe, and I think that's one of the nicer
details in the codebase. It's a struct of four separately typed channels —
one per phase of the public discussion:

```go
type classicalChannel struct {
    bases      chan []Basis            // Bob -> Alice
    matches    chan []int              // Alice -> Bob
    sample     chan sampleAnnouncement // Alice -> Bob
    mismatches chan int                // Bob -> Alice
}
```

Each field carries exactly one kind of message, in exactly one direction.
You can read the entire public protocol off that struct definition — you
don't need a sequence diagram, the types already are the sequence diagram.

**HOST A:** And I'm guessing all of this is reproducible, given how
precisely you quoted those QBER numbers a minute ago?

**HOST B:** Fully. Every stochastic step — Alice's bits, everyone's basis
choices, Eve's decision to attack a given photon, which positions get
sacrificed for the sample — draws from a per-party `*rand.Rand` that's
itself derived from one master seed passed into `Config`. Same seed, same
run, bit for bit, every time. That's what let me paste those exact numbers
into this script without hand-waving — `-seed 42` always gives 0.2315.

**HOST A:** So how do they know if someone was listening?

**HOST B:** Phase three, error estimation. Alice sacrifices a random slice
of the sifted key — a quarter of it by default — and announces those
positions *and* her bit values publicly. That's the one sanctioned
exception to "bits never go on the classical channel," and it's sanctioned
precisely because those bits get thrown away afterward; they're spent, not
secret anymore. Bob compares his own bits at those same positions and
reports back the mismatch count. Both sides independently compute the same
error rate from those two public integers — that's the quantum bit error
rate, QBER.

**HOST A:** And phase four is just "is that number too high."

**HOST B:** Phase four, decision: if the QBER exceeds a threshold — 11% by
default, which is the standard theoretical security bound for BB84 — both
parties abort and throw away everything. Otherwise, the surviving
unsacrificed bits become the shared key.

**HOST A:** Okay, now Eve. Walk me through what she actually does when
she's turned on.

**HOST B:** She sits on the wire between Alice's channel and Bob's. For
every incoming photon, with some configurable probability, she measures it
in a randomly guessed basis — because measuring is the only way she learns
anything — and then, since she can't clone the original, she has to forward
*something*, so she re-encodes her own measured bit in her own guessed
basis and sends that along.

```go
if rng.Float64() < fraction {
    basis := randomBasis(rng)
    bit := q.Measure(basis, rng)
    q = NewQubit(bit, basis) // she can't clone; she must resend her guess
}
out <- q
```

**HOST A:** And wiring her into the pipeline is really just redirecting a
channel?

**HOST B:** One line in the orchestrator. If Eve's enabled, Bob's receive
end points at Eve's output channel instead of Alice's send channel directly.
Alice's code doesn't change. Bob's code doesn't change. Eve doesn't get
elevated permissions or a peek at anyone's private state — she gets a
channel end, exactly like everyone else in this system, and that's the only
power she has.

**HOST A:** Now give me the arithmetic. Why 25%?

**HOST B:** Eve guesses a basis, and she's wrong half the time. When she's
wrong, she measures the photon in the wrong basis, gets a coin-flip result,
and re-encodes it — in the wrong basis. Now here's the part that catches
people: even on the positions where Bob's basis matches *Alice's original*
basis — which is exactly the subset that survives sifting — if Eve
re-encoded in the wrong basis, Bob's measurement against that re-encoded
photon is *also* a coin flip, because his basis doesn't match what's
actually sitting in the photon anymore. Wrong guess, half the time; coin
flip on top of that, half the time. A quarter of the sifted positions come
back wrong. That's your 25%.

**HOST A:** Let's do the "let me run the demo" bit. Show me.

**HOST B:** Clean run first:

```text
$ go run ./cmd/bb84 -n 4096 -seed 42
  sifted key length  2073 (50.6% of sent)
  estimated QBER     0.0000 (abort threshold 0.1100)
  decision           ACCEPT — final key length 1555
```

No Eve, QBER is dead zero, key gets accepted. Now the same run with `-eve`:

```text
$ go run ./cmd/bb84 -n 4096 -seed 42 -eve
  sifted key length  2002 (48.9% of sent)
  estimated QBER     0.2315 (abort threshold 0.1100)
  decision           ABORT — channel is not trustworthy, key discarded
```

**HOST A:** Twenty-three percent, against an eleven percent threshold.
That's not subtle.

**HOST B:** That's the whole point — the security margin is enormous by
design. And you don't need full interception to see the signature scale:
there's an `-eve-fraction` flag, and if Eve only attacks a quarter of the
qubits instead of all of them, the QBER comes out around a quarter of 25%,
roughly 6%. The tests pin down both the full-interception number and the
partial-fraction scaling, with statistical tolerances tight enough to be
meaningful but loose enough not to flake.

**HOST A:** Last thing — you mentioned goroutine leaks earlier off-mic.
What was that about?

**HOST B:** Every blocking channel operation in this codebase — every send,
every receive — is wrapped in a select against a shared, cancellable
context. If any party hits an error, it cancels that context, which
unblocks everyone else immediately instead of leaving them waiting forever
on a message that's never coming. `Run` waits for every goroutine before it
returns, so nothing outlives the call. There's a dedicated leak test that
runs sessions, including one deliberately cancelled mid-flight at a few
million qubits, and checks the goroutine count returns to baseline
afterward. During review, I actually injected a deliberate leak to make
sure the test would catch it — and it did.

**HOST A:** That's a good note to end on. Where should people go to try
this themselves?

**HOST B:** github.com/Pisush/bb84-qkd — clone it, run the CLI with and
without `-eve`, and then go read `alice.go`, `bob.go`, and `eve.go` back to
back. They're short, and once you see that Eve's file is barely different
in shape from Bob's, the "she's just another goroutine" framing stops being
a slogan and starts being obviously, structurally true.

**HOST A:** Love it. Thanks for building this.

**HOST B:** Thanks for having me.

---

*Approx. 2,050 words.*
