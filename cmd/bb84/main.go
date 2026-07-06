// Command bb84 runs one BB84 quantum key distribution session and prints
// a summary: sifted key length, estimated QBER, and the accept/abort
// decision.
//
// Usage:
//
//	bb84 [-n qubits] [-eve] [-eve-fraction f] [-seed s]
//	     [-sample-fraction f] [-threshold t]
//
// The process exits 0 when the key was accepted, 1 when the run aborted,
// and 2 on usage or runtime errors.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	bb84 "github.com/Pisush/bb84-qkd"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses args, executes one session, and writes the summary to out.
// It returns the process exit code: 0 accept, 1 abort, 2 error.
func run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("bb84", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var (
		n              = fs.Int("n", bb84.DefaultN, "number of qubits Alice transmits")
		eve            = fs.Bool("eve", false, "enable the intercept-resend eavesdropper")
		eveFraction    = fs.Float64("eve-fraction", bb84.DefaultEveFraction, "fraction of qubits Eve intercepts (0 disables her)")
		seed           = fs.Uint64("seed", 0, "RNG seed for a reproducible run (0 = derive from the clock)")
		sampleFraction = fs.Float64("sample-fraction", bb84.DefaultSampleFraction, "fraction of sifted bits sacrificed for error estimation")
		threshold      = fs.Float64("threshold", bb84.DefaultQBERThreshold, "abort when the estimated QBER exceeds this")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	effectiveSeed := *seed
	if effectiveSeed == 0 {
		effectiveSeed = uint64(time.Now().UnixNano())
	}
	cfg := bb84.Config{
		N:              *n,
		Seed:           effectiveSeed,
		Eve:            *eve,
		EveFraction:    *eveFraction,
		SampleFraction: *sampleFraction,
		QBERThreshold:  *threshold,
	}
	// The library treats zero-valued fields as "unset, use the default",
	// so explicit zeros from the command line are mapped to their intent
	// here: an Eve who intercepts nothing is no Eve at all, a zero sample
	// skips error estimation, and a zero threshold aborts on any error.
	// The tiny stand-in fractions are far below 1/N, so they are exact.
	if cfg.Eve && cfg.EveFraction <= 0 {
		cfg.Eve = false
		cfg.EveFraction = 0
	}
	if *sampleFraction == 0 {
		cfg.SampleFraction = 1e-12
	}
	if *threshold == 0 {
		cfg.QBERThreshold = 1e-12
	}

	res, err := bb84.Run(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(errOut, "bb84: %v\n", err)
		return 2
	}

	summarize(out, cfg, res)
	if !res.Accepted {
		return 1
	}
	return 0
}

// summarize prints the run summary for one Result.
func summarize(w io.Writer, cfg bb84.Config, res bb84.Result) {
	eavesdropper := "off"
	if cfg.Eve {
		eavesdropper = fmt.Sprintf("intercept-resend, fraction %.2f", cfg.EveFraction)
	}
	fmt.Fprintf(w, "BB84 run (seed %d)\n", cfg.Seed)
	fmt.Fprintf(w, "  qubits sent        %d\n", res.N)
	fmt.Fprintf(w, "  eavesdropper       %s\n", eavesdropper)
	fmt.Fprintf(w, "  sifted key length  %d (%.1f%% of sent)\n",
		res.SiftedLen, 100*float64(res.SiftedLen)/float64(max(res.N, 1)))
	fmt.Fprintf(w, "  sacrificed sample  %d bits\n", res.SampleSize)
	fmt.Fprintf(w, "  estimated QBER     %.4f (abort threshold %.4f)\n", res.QBER, cfg.QBERThreshold)
	if res.Accepted {
		fmt.Fprintf(w, "  decision           ACCEPT — final key length %d\n", len(res.AliceKey))
	} else {
		fmt.Fprintf(w, "  decision           ABORT — channel is not trustworthy, key discarded\n")
	}
}
