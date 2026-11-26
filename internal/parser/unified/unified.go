package unified

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/model"
)

var (
	headerRegex = regexp.MustCompile(`\[info\s*\]\[gc,init\s*\]\s*([^:]+):\s*(.+)`)
	usingRegex  = regexp.MustCompile(`\[info\s*\]\[gc\s*\]\s*Using\s+(\w+)`)

	// Matches summary pause line:
	// GC(0) Pause Young (Normal) (G1 Evacuation Pause) 124M->42M(4096M) 14.234ms
	// GC(1) Pause Full (System.gc()) 3200M->850M(4096M) 1845.120ms
	pauseSummaryRegex = regexp.MustCompile(`GC\((\d+)\)\s+Pause\s+([A-Za-z]+)(?:\s+(.*?))?\s+(\d+[A-Za-z]*)->(\d+[A-Za-z]*)\((\d+[A-Za-z]*)\)\s+([0-9.,]+)ms`)

	// ZGC pauses:
	// GC(0) Pause Mark Start 0.050ms
	// GC(0) Y: Pause Mark End 0.080ms
	zgcPauseRegex = regexp.MustCompile(`GC\((\d+)\)\s+(?:[YyOo]:\s+)?Pause\s+([A-Za-z ]+?)\s+([0-9.,]+)ms`)

	// ZGC Allocation Stall:
	// GC(0) Allocation Stall (Server thread) 12.345ms
	zgcStallRegex = regexp.MustCompile(`GC\((\d+)\)\s+(?:[YyOo]:\s+)?Allocation Stall\s+\((.*?)\)\s+([0-9.,]+)ms`)

	// Matches region details:
	// GC(0) Humongous regions: 2->2
	humongousRegex = regexp.MustCompile(`GC\((\d+)\)\s+Humongous regions:\s+(\d+)->(\d+)`)
)

type Parser struct {
	JVM    model.JVMInfo
	events map[uint64]*model.GCEvent
	order  []uint64
}

func NewParser() *Parser {
	return &Parser{
		JVM: model.JVMInfo{
			Collector: model.CollectorUnknown,
		},
		events: make(map[uint64]*model.GCEvent),
	}
}

func (p *Parser) Events() []*model.GCEvent {
	res := make([]*model.GCEvent, 0, len(p.order))
	for _, id := range p.order {
		if ev, ok := p.events[id]; ok && (ev.Type != "" || ev.Duration > 0) {
			res = append(res, ev)
		}
	}
	return res
}

func (p *Parser) EventCount() int {
	count := 0
	for _, id := range p.order {
		if ev, ok := p.events[id]; ok && (ev.Type != "" || ev.Duration > 0) {
			count++
		}
	}
	return count
}

func (p *Parser) LastEvent() *model.GCEvent {
	for i := len(p.order) - 1; i >= 0; i-- {
		if ev, ok := p.events[p.order[i]]; ok && (ev.Type != "" || ev.Duration > 0) {
			return ev
		}
	}
	return nil
}

func (p *Parser) Parse(r io.Reader) ([]*model.GCEvent, error) {
	if r == nil {
		return p.Events(), nil
	}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		p.ParseLine(line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return p.Events(), nil
}

func (p *Parser) ParseLine(line string) {
	if strings.Contains(line, "Using ") {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "THE Z GARBAGE COLLECTOR") || strings.Contains(upper, "USING ZGC") {
			p.JVM.Collector = model.CollectorZGC
		} else if m := usingRegex.FindStringSubmatch(line); len(m) > 1 {
			switch strings.ToUpper(m[1]) {
			case "G1":
				p.JVM.Collector = model.CollectorG1
			case "PARALLEL":
				p.JVM.Collector = model.CollectorParallel
			case "CMS":
				p.JVM.Collector = model.CollectorCMS
			case "ZGC":
				p.JVM.Collector = model.CollectorZGC
			case "SHENANDOAH":
				p.JVM.Collector = model.CollectorShenandoah
			}
		}
	}

	if strings.Contains(line, "[gc,init") {
		p.parseHeader(line)
		return
	}

	if !strings.Contains(line, "GC(") {
		return
	}

	if m := pauseSummaryRegex.FindStringSubmatch(line); len(m) > 0 {
		p.parsePauseSummary(line, m)
		return
	}

	if m := zgcPauseRegex.FindStringSubmatch(line); len(m) > 0 {
		p.parseZGCPause(line, m)
		return
	}

	if m := zgcStallRegex.FindStringSubmatch(line); len(m) > 0 {
		p.parseZGCStall(line, m)
		return
	}

	if m := humongousRegex.FindStringSubmatch(line); len(m) > 0 {
		id, _ := strconv.ParseUint(m[1], 10, 64)
		count, _ := strconv.Atoi(m[3])
		if count > 0 {
			ev := p.getOrCreateEvent(id)
			ev.HumongousCount = count
			ev.RawLines = append(ev.RawLines, line)
		}
	}
}

func (p *Parser) parseZGCPause(line string, m []string) {
	id, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return
	}

	ev := p.getOrCreateEvent(id)
	ev.ID = id
	if ev.Type == "" || ev.Type == model.GCTypeUnknown {
		ev.Type = model.GCTypeYoung
	}
	if ev.Reason == "" {
		prefix := "ZGC Pause"
		if p.JVM.Collector == model.CollectorShenandoah {
			prefix = "Shenandoah Pause"
		}
		ev.Reason = prefix + " (" + strings.TrimSpace(m[2]) + ")"
	}

	durationStr := strings.ReplaceAll(m[3], ",", ".")
	if durMs, err := strconv.ParseFloat(durationStr, 64); err == nil {
		ev.Duration += time.Duration(durMs * float64(time.Millisecond))
	}

	if ev.StartTime.IsZero() {
		ev.StartTime, ev.Uptime = extractDecorations(line)
	}
	ev.RawLines = append(ev.RawLines, line)
}

func (p *Parser) parseZGCStall(line string, m []string) {
	id, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return
	}

	ev := p.getOrCreateEvent(id)
	ev.ID = id
	ev.Reason = "ZGC Allocation Stall (" + strings.TrimSpace(m[2]) + ")"

	durationStr := strings.ReplaceAll(m[3], ",", ".")
	if durMs, err := strconv.ParseFloat(durationStr, 64); err == nil {
		ev.Duration += time.Duration(durMs * float64(time.Millisecond))
	}

	if ev.StartTime.IsZero() {
		ev.StartTime, ev.Uptime = extractDecorations(line)
	}
	ev.RawLines = append(ev.RawLines, line)
}

func (p *Parser) parseHeader(line string) {
	m := headerRegex.FindStringSubmatch(line)
	if len(m) < 3 {
		return
	}
	key := strings.TrimSpace(m[1])
	val := strings.TrimSpace(m[2])

	switch key {
	case "Version":
		p.JVM.FullVersion = val
		parts := strings.FieldsFunc(val, func(r rune) bool {
			return r == '.' || r == '+' || r == '-' || r == ' '
		})
		if len(parts) > 0 {
			if major, err := strconv.Atoi(parts[0]); err == nil {
				p.JVM.MajorVersion = major
			}
		}
	case "CPUs":
		fmt.Sscanf(val, "%d", &p.JVM.CPUs)
	case "Memory":
		p.JVM.PhysicalMemory = parseBytes(val)
	case "Heap Min Capacity":
		p.JVM.HeapMin = parseBytes(val)
	case "Heap Initial Capacity":
		p.JVM.HeapInitial = parseBytes(val)
	case "Heap Max Capacity":
		p.JVM.HeapMax = parseBytes(val)
	case "Heap Region Size":
		p.JVM.RegionSize = parseBytes(val)
	case "Pre-touch":
		enabled := strings.EqualFold(val, "Enabled")
		p.JVM.AlwaysPreTouch = &enabled
	}
}

func (p *Parser) parsePauseSummary(line string, m []string) {
	id, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return
	}

	ev := p.getOrCreateEvent(id)
	ev.ID = id
	ev.Type = parseGCType(m[2])
	ev.Reason = strings.TrimSpace(m[3])
	if strings.EqualFold(m[2], "Degenerated") {
		ev.Type = model.GCTypeFull
		if ev.Reason == "" {
			ev.Reason = "Degenerated GC"
		} else {
			ev.Reason = "Degenerated " + ev.Reason
		}
	}

	ev.HeapBefore = parseBytes(m[4])
	ev.HeapAfter = parseBytes(m[5])
	ev.HeapTotal = parseBytes(m[6])

	durationStr := strings.ReplaceAll(m[7], ",", ".")
	if durMs, err := strconv.ParseFloat(durationStr, 64); err == nil {
		ev.Duration = time.Duration(durMs * float64(time.Millisecond))
	}

	ev.StartTime, ev.Uptime = extractDecorations(line)
	ev.RawLines = append(ev.RawLines, line)
}

func (p *Parser) getOrCreateEvent(id uint64) *model.GCEvent {
	if ev, ok := p.events[id]; ok {
		return ev
	}
	ev := &model.GCEvent{ID: id}
	p.events[id] = ev
	p.order = append(p.order, id)
	return ev
}

func parseGCType(s string) model.GCType {
	switch strings.ToLower(s) {
	case "young":
		return model.GCTypeYoung
	case "mixed":
		return model.GCTypeMixed
	case "full":
		return model.GCTypeFull
	case "remark":
		return model.GCTypeRemark
	case "cleanup":
		return model.GCTypeCleanup
	default:
		return model.GCTypeUnknown
	}
}

func extractDecorations(line string) (time.Time, time.Duration) {
	var t time.Time
	var upt time.Duration

	tokens := extractBracketTokens(line)
	for _, tok := range tokens {
		if strings.HasSuffix(tok, "s") {
			sVal := strings.TrimSuffix(tok, "s")
			sVal = strings.ReplaceAll(sVal, ",", ".")
			if sec, err := strconv.ParseFloat(sVal, 64); err == nil {
				upt = time.Duration(sec * float64(time.Second))
				continue
			}
		}

		// Try parsing ISO8601 timestamp e.g. 2026-11-15T18:04:12.123+0330 or 2026-11-15T18:04:12.123Z
		layouts := []string{
			"2006-01-02T15:04:05.000-0700",
			"2006-01-02T15:04:05.000Z07:00",
			"2006-01-02T15:04:05.000",
			time.RFC3339Nano,
			time.RFC3339,
		}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, tok); err == nil {
				t = parsed
				break
			}
		}
	}

	return t, upt
}

func extractBracketTokens(line string) []string {
	var tokens []string
	inBracket := false
	start := 0
	for i := 0; i < len(line); i++ {
		if !inBracket {
			if line[i] == '[' {
				inBracket = true
				start = i + 1
			} else if line[i] != ' ' && line[i] != '\t' {
				break
			}
			continue
		}
		if line[i] == ']' {
			inBracket = false
			tokens = append(tokens, strings.TrimSpace(line[start:i]))
		}
	}
	return tokens
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
