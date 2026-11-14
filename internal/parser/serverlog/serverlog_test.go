package serverlog

import (
	"strings"
	"testing"
	"time"
)

func TestParseServerLog(t *testing.T) {
	sample := `
[18:04:00] [Server thread/INFO]: Starting minecraft server version 1.20.4
[18:04:00] [Server thread/INFO]: Loading Paper 1.20.4-R0.1-SNAPSHOT
[18:04:05] [Server thread/INFO]: Done (5.123s)! For help, type "help"
[18:05:30] [Server thread/WARN]: Can't keep up! Is the server overloaded? Running 1250ms or 25 ticks behind
[18:06:00] [Server thread/WARN]: Can't keep up! Running 3400ms or 68 ticks behind
[18:07:00] [Spigot Watchdog Thread/FATAL]: A single server tick took 60.00 seconds (should be at most 0.05)
`

	baseDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	res, err := Parse(strings.NewReader(sample), Options{
		BaseDate: baseDate,
		Timezone: time.UTC,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ServerBrand != "Paper" {
		t.Errorf("expected ServerBrand Paper, got %s", res.ServerBrand)
	}
	if res.ServerVersion != "1.20.4" {
		t.Errorf("expected ServerVersion 1.20.4, got %s", res.ServerVersion)
	}

	if len(res.Events) != 3 {
		t.Fatalf("expected 3 lag events, got %d", len(res.Events))
	}

	ev0 := res.Events[0]
	if ev0.Duration != 1250*time.Millisecond {
		t.Errorf("expected 1250ms duration, got %v", ev0.Duration)
	}
	if ev0.TicksBehind != 25 {
		t.Errorf("expected 25 ticks, got %d", ev0.TicksBehind)
	}
	expectedTime0 := time.Date(2026, 11, 15, 18, 5, 30, 0, time.UTC)
	if !ev0.Time.Equal(expectedTime0) {
		t.Errorf("expected time %v, got %v", expectedTime0, ev0.Time)
	}

	ev2 := res.Events[2]
	if ev2.Duration != 60*time.Second {
		t.Errorf("expected 60s duration, got %v", ev2.Duration)
	}
	if ev2.Source != "Watchdog" {
		t.Errorf("expected Watchdog source, got %s", ev2.Source)
	}
}

func TestMidnightRollover(t *testing.T) {
	sample := `
[23:59:50] [Server thread/INFO]: Server tick loop normal
[00:00:10] [Server thread/WARN]: Can't keep up! Running 1500ms or 30 ticks behind
`
	baseDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	res, err := Parse(strings.NewReader(sample), Options{
		BaseDate: baseDate,
		Timezone: time.UTC,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Events) != 1 {
		t.Fatalf("expected 1 lag event, got %d", len(res.Events))
	}

	ev := res.Events[0]
	expectedDay := 16 // Rolled over to next day
	if ev.Time.Day() != expectedDay {
		t.Errorf("expected day %d after rollover, got %d (full time: %v)", expectedDay, ev.Time.Day(), ev.Time)
	}
}

func TestForgeTimestampAndWatchdogDedup(t *testing.T) {
	sample := `
[15Nov2026 18:04:00.000] [main/INFO]: Loading Minecraft 1.20.4 with Fabric Loader 0.15.7
[15Nov2026 18:05:10.500] [Server thread/WARN]: Can't keep up! Is the server overloaded? Running 2200ms or 44 ticks behind
[15Nov2026 18:06:00.000] [Spigot Watchdog Thread/ERROR]: The server has stopped responding!
[15Nov2026 18:06:01.000] [Spigot Watchdog Thread/ERROR]: A single server tick took 60.00 seconds (should be at most 0.05)
`
	baseDate := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	res, err := Parse(strings.NewReader(sample), Options{BaseDate: baseDate, Timezone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}

	if res.ServerBrand != "Fabric" || res.ServerVersion != "1.20.4" {
		t.Errorf("unexpected brand/version: %s / %s", res.ServerBrand, res.ServerVersion)
	}
	if len(res.Events) != 2 {
		t.Fatalf("expected 2 deduplicated events, got %d", len(res.Events))
	}
	if res.Events[1].Duration != 60*time.Second || res.Events[1].Source != "Watchdog" {
		t.Errorf("expected deduplicated 60s Watchdog event, got %+v", res.Events[1])
	}
}
