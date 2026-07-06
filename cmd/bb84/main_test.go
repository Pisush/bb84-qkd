package main

import (
	"strings"
	"testing"
)

func TestRunExitCodesAndSummary(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantContains []string
	}{
		{
			name:         "clean run accepts",
			args:         []string{"-n", "4096", "-seed", "1"},
			wantCode:     0,
			wantContains: []string{"ACCEPT", "eavesdropper       off", "seed 1"},
		},
		{
			name:         "full Eve aborts",
			args:         []string{"-n", "4096", "-seed", "1", "-eve"},
			wantCode:     1,
			wantContains: []string{"ABORT", "intercept-resend, fraction 1.00"},
		},
		{
			name:         "eve with zero fraction is no eve",
			args:         []string{"-n", "2048", "-seed", "2", "-eve", "-eve-fraction", "0"},
			wantCode:     0,
			wantContains: []string{"eavesdropper       off"},
		},
		{
			// f=0.75 → QBER ≈ 18.75%, ~7σ above the 11% threshold at
			// this sample size — decisive regardless of seed.
			name:         "three-quarter Eve aborts",
			args:         []string{"-n", "8192", "-seed", "3", "-eve", "-eve-fraction", "0.75"},
			wantCode:     1,
			wantContains: []string{"ABORT"},
		},
		{
			name:     "bad flag is a usage error",
			args:     []string{"-definitely-not-a-flag"},
			wantCode: 2,
		},
		{
			name:     "invalid config is a runtime error",
			args:     []string{"-eve", "-eve-fraction", "1.5"},
			wantCode: 2,
		},
		{
			name:         "zero threshold aborts on any error",
			args:         []string{"-n", "4096", "-seed", "1", "-eve", "-eve-fraction", "0.1", "-threshold", "0"},
			wantCode:     1,
			wantContains: []string{"ABORT"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut strings.Builder
			code := run(tt.args, &out, &errOut)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s",
					code, tt.wantCode, out.String(), errOut.String())
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(out.String(), want) {
					t.Errorf("summary missing %q:\n%s", want, out.String())
				}
			}
		})
	}
}
