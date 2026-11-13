package unified

import "testing"

func FuzzParseLine(f *testing.F) {
	// Seed corpus
	f.Add("[0.002s][info][gc] Using G1")
	f.Add("[0.002s][info][gc,init] Version: 21.0.3+9-LTS (release)")
	f.Add("[2026-11-15T18:04:12.123+0300][12.456s][info][gc] GC(0) Pause Young (Normal) (G1 Evacuation Pause) 124M->42M(4096M) 14.234ms")
	f.Add("[2026-11-15T18:05:00.100+0300][60.100s][info][gc] GC(2) Pause Full (System.gc()) 3200M->850M(4096M) 1845.120ms")
	f.Add("[2026-11-15T18:04:10.000+0300][10.000s][info][gc,phases] GC(0) Pause Mark Start 0.050ms")
	f.Add("[2026-11-15T18:04:12.000+0300][12.000s][info][gc] GC(1) Allocation Stall (Server thread) 15.500ms")
	f.Add("random corrupted unbracketed text line")
	f.Add("[9999999s][error][gc] GC(9999999999999999999999) Pause")

	p := NewParser()
	f.Fuzz(func(t *testing.T, line string) {
		p.ParseLine(line)
	})
}
