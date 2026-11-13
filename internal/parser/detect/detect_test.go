package detect

import (
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	unifiedLog := `[0.002s][info][gc] Using G1
[0.002s][info][gc,init] Version: 21.0.3+9-LTS
[2026-11-15T18:04:10.000+0300][10.000s][info][gc] GC(0) Pause Young (Normal)`

	legacyFlags := `CommandLine flags: -XX:InitialHeapSize=2147483648 -XX:MaxHeapSize=4294967296 -XX:+UseG1GC
2026-11-15T18:04:12.123+0300: 12.456: [GC pause (G1 Evacuation Pause) (young), 0.0142340 secs]`

	legacyDirect := `2026-11-15T18:04:10.123+0300: 10.456: [GC (Allocation Failure) [PSYoungGen: 524288K->65536K(614400K)] 524288K->65600K(2097152K), 0.0152340 secs]`

	unknownLog := `Server started on port 25565
[Server thread/INFO]: Loading properties
[Server thread/INFO]: Default game type: SURVIVAL`

	tests := []struct {
		name     string
		input    string
		expected Format
	}{
		{"unified", unifiedLog, FormatUnified},
		{"legacy with flags", legacyFlags, FormatLegacy},
		{"legacy direct GC", legacyDirect, FormatLegacy},
		{"unknown log", unknownLog, FormatUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectFormat(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
