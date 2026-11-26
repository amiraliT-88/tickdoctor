package flags

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

type ParsedFlags struct {
	RawCmd     string
	JavaBin    string
	Collector  string
	HeapMin    int64
	HeapMax    int64
	PreTouch   *bool
	DisableGC  *bool
	RegionSize int64
	AllFlags   []string
	RawTokens  []string
}

var (
	sizeRegex = regexp.MustCompile(`^(\d+)([KkMmGgTtBb]?)$`)
)

func ParseScriptOrString(input string) (*ParsedFlags, error) {
	content := input

	if fi, err := os.Stat(input); err == nil && !fi.IsDir() {
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, err
		}
		content = string(data)
	}

	return ParseCommandLine(content), nil
}

func ParseCommandLine(cmdLine string) *ParsedFlags {
	res := &ParsedFlags{
		RawCmd: cmdLine,
	}

	expanded := expandScriptVars(cmdLine)
	tokens := flattenEmbeddedCommands(tokenize(expanded), 0)
	res.RawTokens = tokens

	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if !strings.HasPrefix(tok, "-") {
			lowerTok := strings.ToLower(tok)
			if strings.HasSuffix(lowerTok, "java") || strings.HasSuffix(lowerTok, "java.exe") {
				res.JavaBin = tok
			}
			continue
		}

		res.AllFlags = append(res.AllFlags, tok)

		if strings.HasPrefix(tok, "-Xms") {
			val := strings.TrimPrefix(tok, "-Xms")
			res.HeapMin = ParseSize(val)
			continue
		}

		if strings.HasPrefix(tok, "-Xmx") {
			val := strings.TrimPrefix(tok, "-Xmx")
			res.HeapMax = ParseSize(val)
			continue
		}

		switch tok {
		case "-XX:+UseG1GC":
			res.Collector = "G1"
		case "-XX:+UseParallelGC":
			res.Collector = "Parallel"
		case "-XX:+UseConcMarkSweepGC":
			res.Collector = "CMS"
		case "-XX:+UseZGC":
			res.Collector = "ZGC"
		case "-XX:+UseShenandoahGC":
			res.Collector = "Shenandoah"
		case "-XX:+UseSerialGC":
			res.Collector = "Serial"
		}

		if tok == "-XX:+AlwaysPreTouch" {
			t := true
			res.PreTouch = &t
		} else if tok == "-XX:-AlwaysPreTouch" {
			f := false
			res.PreTouch = &f
		}

		if tok == "-XX:+DisableExplicitGC" {
			t := true
			res.DisableGC = &t
		} else if tok == "-XX:-DisableExplicitGC" {
			f := false
			res.DisableGC = &f
		}

		if strings.HasPrefix(tok, "-XX:G1HeapRegionSize=") {
			val := strings.TrimPrefix(tok, "-XX:G1HeapRegionSize=")
			res.RegionSize = ParseSize(val)
		}
	}

	return res
}

func ParseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	m := sizeRegex.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}

	num, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(m[2])
	switch unit {
	case "G":
		return num * 1024 * 1024 * 1024
	case "M":
		return num * 1024 * 1024
	case "K":
		return num * 1024
	case "T":
		return num * 1024 * 1024 * 1024 * 1024
	default:
		return num
	}
}

func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	lineStart := true

	skipToEOL := func(idx int) int {
		for idx < len(line) && line[idx] != '\n' {
			idx++
		}
		return idx
	}

	for i := 0; i < len(line); i++ {
		c := line[i]

		if !inQuotes && current.Len() == 0 {
			if c == '#' {
				i = skipToEOL(i)
				lineStart = true
				continue
			}
			if lineStart {
				if strings.HasPrefix(line[i:], "::") {
					i = skipToEOL(i)
					lineStart = true
					continue
				}
				if len(line)-i >= 3 && strings.EqualFold(line[i:i+3], "REM") &&
					(len(line)-i == 3 || line[i+3] == ' ' || line[i+3] == '\t' || line[i+3] == '\r' || line[i+3] == '\n') {
					i = skipToEOL(i)
					lineStart = true
					continue
				}
			}
		}

		if !inQuotes && (c == '\\' || c == '^') && i+1 < len(line) {
			if line[i+1] == '\r' && i+2 < len(line) && line[i+2] == '\n' {
				i += 2
				continue
			}
			if line[i+1] == '\r' || line[i+1] == '\n' {
				i++
				continue
			}
		}

		if (c == '"' || c == '\'') && !inQuotes {
			inQuotes = true
			quoteChar = c
			lineStart = false
			continue
		} else if inQuotes && c == quoteChar {
			inQuotes = false
			continue
		}

		if (c == ' ' || c == '\t' || c == '\r' || c == '\n') && !inQuotes {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			if c == '\n' {
				lineStart = true
			}
			continue
		}

		lineStart = false
		current.WriteByte(c)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

var (
	batchVarRegex = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)
	shellVarRegex = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?`)
)

func expandScriptVars(content string) string {
	batchVars := make(map[string]string)
	shellVars := make(map[string]string)

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "::") {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "rem ") || lower == "rem" {
			continue
		}

		if strings.HasPrefix(lower, "set ") {
			rest := strings.TrimSpace(line[4:])
			if len(rest) >= 2 && (strings.EqualFold(rest[:2], "/a") || strings.EqualFold(rest[:2], "/p")) {
				rest = strings.TrimSpace(rest[2:])
			}
			rest = strings.Trim(rest, `"`)
			if eq := strings.IndexByte(rest, '='); eq > 0 {
				k := strings.TrimSpace(rest[:eq])
				v := strings.Trim(strings.TrimSpace(rest[eq+1:]), `"'`)
				if k != "" {
					batchVars[strings.ToLower(k)] = v
				}
			}
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[7:])
		}
		if eq := strings.IndexByte(line, '='); eq > 0 && !strings.ContainsAny(line[:eq], " \t-\"'") {
			k := line[:eq]
			v := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
			shellVars[k] = v
		}
	}

	if len(batchVars) > 0 {
		content = batchVarRegex.ReplaceAllStringFunc(content, func(m string) string {
			name := strings.ToLower(m[1 : len(m)-1])
			if val, ok := batchVars[name]; ok {
				return val
			}
			return m
		})
	}
	if len(shellVars) > 0 {
		content = shellVarRegex.ReplaceAllStringFunc(content, func(m string) string {
			name := strings.TrimPrefix(m, "$")
			name = strings.TrimPrefix(name, "{")
			name = strings.TrimSuffix(name, "}")
			if val, ok := shellVars[name]; ok {
				return val
			}
			return m
		})
	}

	return content
}

func flattenEmbeddedCommands(tokens []string, depth int) []string {
	if depth >= 2 {
		return tokens
	}
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if strings.ContainsAny(tok, " \t") && (strings.Contains(tok, " -X") || strings.HasPrefix(strings.TrimSpace(tok), "-X")) {
			inner := tokenize(tok)
			out = append(out, flattenEmbeddedCommands(inner, depth+1)...)
			continue
		}
		out = append(out, tok)
	}
	return out
}
