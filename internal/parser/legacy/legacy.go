package legacy

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

var (
	// Matches Java 8 standard GC line:
	// 2026-11-15T18:04:10.123+0300: 10.456: [GC (Allocation Failure) [PSYoungGen: ...] 524288K->65600K(2097152K), 0.0152340 secs]
	// 2026-11-15T18:05:00.000+0300: 60.123: [Full GC (System.gc()) [PSYoungGen: ...] [ParOldGen: ...] 565536K->350000K(2097152K), [Metaspace: ...], 0.8523100 secs]
	gcRegex = regexp.MustCompile(`\[(Full GC|GC)(?:\s+\((.*?)\))?\s+\[.*?\s+(\d+[A-Za-z.]*)->(\d+[A-Za-z.]*)\((\d+[A-Za-z.]*)\)(?:,\s*\[Metaspace:[^\]]+\])?,\s*([0-9.,]+)\s*secs\]`)

	// Matches G1 pause line:
	// 2026-11-15T18:04:12.123+0300: 12.456: [GC pause (G1 Evacuation Pause) (young), 0.0142340 secs]
	g1PauseRegex = regexp.MustCompile(`\[GC pause\s*(?:\(([^)]+)\))?\s*(?:\(([^)]+)\))?,\s*([0-9.,]+)\s*secs\]`)
	g1HeapRegex  = regexp.MustCompile(`Heap:\s*([0-9A-Za-z.]+)\(([0-9A-Za-z.]+)\)->([0-9A-Za-z.]+)\(([0-9A-Za-z.]+)\)`)
)

type Parser struct {
	JVM    model.JVMInfo
	events []*model.GCEvent
	nextID uint64
}

func NewParser() *Parser {
	return &Parser{
		JVM: model.JVMInfo{
			MajorVersion: 8,
			Collector:    model.CollectorUnknown,
		},
	}
}

func (p *Parser) Parse(r io.Reader) ([]*model.GCEvent, error) {
	if r == nil {
		return p.events, nil
	}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		p.ParseLine(scanner.Text())
	}

	return p.events, scanner.Err()
}

func (p *Parser) Events() []*model.GCEvent {
	return p.events
}

func (p *Parser) EventCount() int {
	return len(p.events)
}

func (p *Parser) LastEvent() *model.GCEvent {
	if len(p.events) == 0 {
		return nil
	}
	return p.events[len(p.events)-1]
}

func (p *Parser) ParseLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, "CommandLine flags:") {
		p.parseCommandLineFlags(line)
		return
	}

	if strings.Contains(line, "[GC pause") {
		p.parseG1Pause(line)
		return
	}

	if m := gcRegex.FindStringSubmatch(line); len(m) > 0 {
		p.parseStandardGC(line, m)
		return
	}

	if len(p.events) > 0 && strings.Contains(line, "Heap:") {
		last := p.events[len(p.events)-1]
		if hm := g1HeapRegex.FindStringSubmatch(line); len(hm) > 0 {
			last.HeapBefore = parseBytes(hm[1])
			last.HeapTotal = parseBytes(hm[2])
			last.HeapAfter = parseBytes(hm[3])
			last.RawLines = append(last.RawLines, line)
		}
	}
}

func (p *Parser) parseCommandLineFlags(line string) {
	flags := strings.Fields(line)
	p.JVM.RawFlags = flags

	for _, f := range flags {
		if strings.HasPrefix(f, "-XX:InitialHeapSize=") {
			val := strings.TrimPrefix(f, "-XX:InitialHeapSize=")
			if num, err := strconv.ParseInt(val, 10, 64); err == nil {
				p.JVM.HeapInitial = num
			}
		}
		if strings.HasPrefix(f, "-XX:MaxHeapSize=") {
			val := strings.TrimPrefix(f, "-XX:MaxHeapSize=")
			if num, err := strconv.ParseInt(val, 10, 64); err == nil {
				p.JVM.HeapMax = num
			}
		}
		if strings.HasPrefix(f, "-XX:G1HeapRegionSize=") {
			val := strings.TrimPrefix(f, "-XX:G1HeapRegionSize=")
			p.JVM.RegionSize = parseBytes(val)
		}
		if f == "-XX:+UseParallelGC" || f == "-XX:+UseParallelOldGC" {
			p.JVM.Collector = model.CollectorParallel
		}
		if f == "-XX:+UseG1GC" {
			p.JVM.Collector = model.CollectorG1
		}
		if f == "-XX:+UseConcMarkSweepGC" {
			p.JVM.Collector = model.CollectorCMS
		}
		if f == "-XX:+AlwaysPreTouch" {
			t := true
			p.JVM.AlwaysPreTouch = &t
		} else if f == "-XX:-AlwaysPreTouch" {
			fVal := false
			p.JVM.AlwaysPreTouch = &fVal
		}
	}
}

func (p *Parser) parseStandardGC(line string, m []string) {
	p.nextID++
	ev := &model.GCEvent{
		ID:       p.nextID,
		RawLines: []string{line},
	}

	ev.StartTime, ev.Uptime = extractLegacyDecorations(line)

	if m[1] == "Full GC" {
		ev.Type = model.GCTypeFull
	} else {
		ev.Type = model.GCTypeYoung
	}

	ev.Reason = m[2]
	if strings.Contains(line, "Concurrent Mode Failure") {
		if ev.Reason == "" {
			ev.Reason = "Concurrent Mode Failure"
		} else {
			ev.Reason += " (Concurrent Mode Failure)"
		}
	}

	ev.HeapBefore = parseBytes(m[3])
	ev.HeapAfter = parseBytes(m[4])
	ev.HeapTotal = parseBytes(m[5])

	secStr := strings.ReplaceAll(m[6], ",", ".")
	if sec, err := strconv.ParseFloat(secStr, 64); err == nil {
		ev.Duration = time.Duration(sec * float64(time.Second))
	}

	if p.JVM.Collector == model.CollectorUnknown {
		if strings.Contains(line, "PSYoungGen") || strings.Contains(line, "ParOldGen") {
			p.JVM.Collector = model.CollectorParallel
		} else if strings.Contains(line, "ParNew") || strings.Contains(line, "CMS") {
			p.JVM.Collector = model.CollectorCMS
		}
	}

	p.events = append(p.events, ev)
}

func (p *Parser) parseG1Pause(line string) {
	m := g1PauseRegex.FindStringSubmatch(line)
	if len(m) == 0 {
		return
	}

	p.nextID++
	ev := &model.GCEvent{
		ID:       p.nextID,
		RawLines: []string{line},
	}

	ev.StartTime, ev.Uptime = extractLegacyDecorations(line)

	reason := m[1]
	subtype := m[2]
	if subtype != "" {
		if reason == "" {
			reason = subtype
		} else {
			reason += " (" + subtype + ")"
		}
	}
	ev.Reason = reason

	if strings.Contains(strings.ToLower(subtype), "young") {
		ev.Type = model.GCTypeYoung
	} else if strings.Contains(strings.ToLower(subtype), "mixed") {
		ev.Type = model.GCTypeMixed
	} else {
		ev.Type = model.GCTypeYoung
	}

	secStr := strings.ReplaceAll(m[3], ",", ".")
	if sec, err := strconv.ParseFloat(secStr, 64); err == nil {
		ev.Duration = time.Duration(sec * float64(time.Second))
	}

	p.JVM.Collector = model.CollectorG1
	p.events = append(p.events, ev)
}

func extractLegacyDecorations(line string) (time.Time, time.Duration) {
	var t time.Time
	var upt time.Duration

	// Java 8 format prefix: 2026-11-15T18:04:10.123+0300: 10.456: [GC ...
	parts := strings.SplitN(line, ": [", 2)
	if len(parts) > 1 {
		prefix := parts[0]
		tokens := strings.Fields(prefix)
		for _, tok := range tokens {
			tok = strings.TrimSuffix(tok, ":")
			if strings.Contains(tok, "T") {
				t = parseTime(tok)
			} else {
				upt = parseUptime(tok)
			}
		}
	}
	return t, upt
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z07:00",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseUptime(s string) time.Duration {
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", ".")
	if sec, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(sec * float64(time.Second))
	}
	return 0
}

func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := int64(1)
	unit := strings.ToUpper(s[len(s)-1:])
	numStr := s
	switch unit {
	case "G":
		mult = 1024 * 1024 * 1024
		numStr = s[:len(s)-1]
	case "M":
		mult = 1024 * 1024
		numStr = s[:len(s)-1]
	case "K":
		mult = 1024
		numStr = s[:len(s)-1]
	case "B":
		numStr = s[:len(s)-1]
	}

	numStr = strings.ReplaceAll(numStr, ",", ".")
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(mult))
}
