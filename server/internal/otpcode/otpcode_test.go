package otpcode

import (
	"regexp"
	"testing"
)

var sixDigit = regexp.MustCompile(`^\d{6}$`)

func TestGenerate_Format(t *testing.T) {
	for i := 0; i < 1000; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}
		if !sixDigit.MatchString(code) {
			t.Fatalf("code %q is not 6 digits", code)
		}
	}
}

func TestGenerate_NoLeadingZeroStrip(t *testing.T) {
	// Codes must be zero-padded to exactly 6 chars. A naive Sprintf
	// of n%1_000_000 with no formatting would give e.g. "1234" for 1234.
	foundShort := false
	for i := 0; i < 5000; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			foundShort = true
			t.Fatalf("code %q is not 6 chars", code)
		}
	}
	if foundShort {
		t.Fatal("expected 6-digit zero-padded codes")
	}
}

func TestGenerate_DistributionIsApproximatelyUniform(t *testing.T) {
	// Statistical smoke test. With 100k samples over 1M buckets, the
	// expected count per bucket is 0.1 and any single bucket's count
	// should be roughly 0 ± 3 (3σ ≈ 0.3). To keep test runtime fast we
	// aggregate into 1000 "deciles" of 1000 codes each and require no
	// decile to be empty.
	const samples = 100_000
	const deciles = 1000
	decileSize := 1_000_000 / deciles
	counts := make([]int, deciles)
	for i := 0; i < samples; i++ {
		code, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		var n int
		for _, c := range code {
			n = n*10 + int(c-'0')
		}
		counts[n/decileSize]++
	}
	empty := 0
	for _, c := range counts {
		if c == 0 {
			empty++
		}
	}
	// With 100k samples uniformly over 1000 deciles (expected 100 each),
	// a tolerance of 30 empty deciles is a generous ceiling. A biased
	// generator (modulo without rejection) would leave many more deciles
	// empty.
	if empty > 30 {
		t.Fatalf("too many empty deciles (%d / %d) — distribution looks biased", empty, deciles)
	}
}
