package detect

import (
	"bufio"
	"io"
	"strings"
)

type Format string

const (
	FormatUnified Format = "unified" // Java 9/17/21/25 unified logging (-Xlog:gc*)
	FormatLegacy  Format = "legacy"  // Java 8 logging (-XX:+PrintGCDetails)
	FormatUnknown Format = "unknown"
)

func DetectFormat(r io.Reader) (Format, error) {
	scanner := bufio.NewScanner(r)
	linesRead := 0

	for scanner.Scan() && linesRead < 50 {
		line := strings.TrimSpace(scanner.Text())
		linesRead++

		if line == "" {
			continue
		}

		if strings.Contains(line, "][gc") || strings.Contains(line, "[info][gc") || strings.Contains(line, "[gc]") || strings.Contains(line, "[gc,") || strings.Contains(line, "GC(") {
			return FormatUnified, nil
		}

		if strings.HasPrefix(line, "CommandLine flags:") {
			return FormatLegacy, nil
		}
		if strings.Contains(line, "Java HotSpot(TM)") || strings.Contains(line, "OpenJDK 64-Bit Server VM") {
			return FormatLegacy, nil
		}
		if strings.Contains(line, ": [GC") || strings.Contains(line, ": [Full GC") {
			return FormatLegacy, nil
		}
		if strings.HasPrefix(line, "[GC") || strings.HasPrefix(line, "[Full GC") {
			return FormatLegacy, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return FormatUnknown, err
	}

	return FormatUnknown, nil
}
