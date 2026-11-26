package diff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
)

func TestCompareVerdicts(t *testing.T) {
	before := analyzer.Metrics{
		TotalEvents:   20,
		PauseAvg:      60 * time.Millisecond,
		PauseP50:      30 * time.Millisecond,
		PauseP95:      140 * time.Millisecond,
		PauseP99:      250 * time.Millisecond,
		PauseMax:      900 * time.Millisecond,
		Above150ms:    4,
		FullCount:     2,
		GCOverheadPct: 3.5,
	}
	afterBetter := analyzer.Metrics{
		TotalEvents:   20,
		PauseAvg:      12 * time.Millisecond,
		PauseP50:      10 * time.Millisecond,
		PauseP95:      20 * time.Millisecond,
		PauseP99:      25 * time.Millisecond,
		PauseMax:      40 * time.Millisecond,
		Above150ms:    0,
		FullCount:     0,
		GCOverheadPct: 0.6,
	}

	rep := Compare(before, afterBetter)
	if rep.Verdict != VerdictImproved {
		t.Fatalf("expected VerdictImproved, got %s", rep.Verdict)
	}
	if rep.PauseP99Ms.DeltaPct > -89.0 {
		t.Errorf("expected ~-90%% p99 improvement, got %.1f%%", rep.PauseP99Ms.DeltaPct)
	}
	if rep.Above150ms.Delta != -4 {
		t.Errorf("expected -4 severe freeze delta, got %d", rep.Above150ms.Delta)
	}

	reg := Compare(afterBetter, before)
	if reg.Verdict != VerdictRegressed {
		t.Fatalf("expected VerdictRegressed, got %s", reg.Verdict)
	}

	same := Compare(before, before)
	if same.Verdict != VerdictUnchanged {
		t.Fatalf("expected VerdictUnchanged, got %s", same.Verdict)
	}
}

func TestRenderDiffTextAndJSON(t *testing.T) {
	before := analyzer.Metrics{TotalEvents: 5, PauseP99: 200 * time.Millisecond, PauseMax: 500 * time.Millisecond, Above150ms: 1}
	after := analyzer.Metrics{TotalEvents: 5, PauseP99: 20 * time.Millisecond, PauseMax: 30 * time.Millisecond, Above150ms: 0}
	rep := Compare(before, after)

	var txt bytes.Buffer
	RenderText(&txt, "old.log", "new.log", rep)
	if !strings.Contains(txt.String(), "IMPROVED") || !strings.Contains(txt.String(), "p99 Pause") {
		t.Errorf("unexpected text diff output:\n%s", txt.String())
	}

	var js bytes.Buffer
	if err := RenderJSON(&js, rep); err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(js.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid diff JSON: %v", err)
	}
	if decoded.Verdict != VerdictImproved {
		t.Errorf("expected IMPROVED in decoded JSON, got %s", decoded.Verdict)
	}
}
