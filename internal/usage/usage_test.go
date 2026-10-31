package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// zone is UTC+2, so the local day 2026-10-31 runs from 2026-10-30T22:00Z to 2026-10-31T22:00Z.
var (
	zone = time.FixedZone("UTC+2", 2*3600)
	now  = time.Date(2026, 10, 31, 12, 0, 0, 0, zone)
)

func today(t *testing.T, root string) Totals {
	t.Helper()
	got, err := Today(root, now)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTodaySumsTokens(t *testing.T) {
	got := today(t, "testdata/projects")
	want := Totals{InputTokens: 116, OutputTokens: 227, CacheCreationTokens: 1003, CacheReadTokens: 11004}
	got.CostUSD = 0
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func writeLog(t *testing.T, lines string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "s.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestTodayCountsRequestOnce(t *testing.T) {
	l := `{"type":"assistant","timestamp":"2026-10-31T08:00:00Z","requestId":"r","message":{"id":"m","model":"x","usage":{"input_tokens":1,"output_tokens":2,"cache_creation_input_tokens":3,"cache_read_input_tokens":4}}}` + "\n"
	got := today(t, writeLog(t, l+l+l))
	if got != (Totals{InputTokens: 1, OutputTokens: 2, CacheCreationTokens: 3, CacheReadTokens: 4}) {
		t.Fatalf("got %+v", got)
	}
}

func TestTodayCost(t *testing.T) {
	cost := func(in, out, cw, cr, pin, pout float64) float64 {
		return (in*pin + out*pout + cw*pin*1.25 + cr*pin*0.1) / 1e6
	}
	want := cost(100, 200, 1000, 10000, 5, 25) + // opus
		cost(10, 20, 0, 1000, 3, 15) + // sonnet
		cost(1, 2, 3, 4, 1, 5) // haiku; <synthetic> adds 0
	if got := today(t, "testdata/projects").CostUSD; math.Abs(got-want) > 1e-12 {
		t.Fatalf("cost %v, want %v", got, want)
	}
}

func TestPriceFor(t *testing.T) {
	for model, want := range map[string]float64{
		"claude-opus-4-20250514":     15,
		"claude-opus-4-1-20250805":   15,
		"claude-opus-4-5-20251101":   5,
		"claude-sonnet-4-5-20250929": 3,
		"claude-haiku-4-5-20251001":  1,
	} {
		if p, ok := priceFor(model); !ok || p.input != want {
			t.Errorf("%s: got %v %v, want input %v", model, p, ok, want)
		}
	}
	if _, ok := priceFor("<synthetic>"); ok {
		t.Error("unknown model has a price")
	}
}

func TestTodayMissingRoot(t *testing.T) {
	if got := today(t, filepath.Join(t.TempDir(), "nope")); got != (Totals{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestTodaySkipsMalformedLine(t *testing.T) {
	got := today(t, writeLog(t, "{not json\n"+`{"type":"assistant","timestamp":"2026-10-31T08:00:00Z","message":{"id":"m","usage":{"input_tokens":7}}}`+"\n"))
	if got.InputTokens != 7 {
		t.Fatalf("got %+v", got)
	}
}
