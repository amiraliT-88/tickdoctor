package serverlog

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
	// Matches [HH:MM:SS], [HH:MM:SS.mmm], [HH:MM:SS WARN], or Forge [15Nov2026 18:05:30.123]
	timePrefixRegex = regexp.MustCompile(`^\[(?:\d{1,2}[A-Za-z]{3}\d{4}\s+)?(\d{2}):(\d{2}):(\d{2})(?:\.(\d{3}))?(?:\s+[A-Z]+)?\]`)

	// Can't keep up! Is the server overloaded? Running 2341ms or 46 ticks behind
	cantKeepUpRegex = regexp.MustCompile(`Can't keep up!\s*(?:Is the server overloaded\?\s*)?Running\s+(\d+)ms\s+or\s+(\d+)\s+ticks\s+behind`)

	// Paper/Spigot Watchdog: A single server tick took 60.00 seconds (should be at most 0.05)
	watchdogSingleTickRegex = regexp.MustCompile(`A single server tick took\s+([0-9.]+)\s+seconds`)

	// Paper/Spigot Watchdog: The server has stopped responding!
	watchdogStoppedRegex = regexp.MustCompile(`The server has stopped responding!`)

	// Version matching
	serverVersionRegex = regexp.MustCompile(`(?:Starting minecraft server version|Loading Minecraft|Loading Paper|Loading Purpur|This server is running \w+ version)\s+([0-9a-zA-Z._-]+)`)
	serverBrandRegex   = regexp.MustCompile(`(?:Loading\s+(Paper|Purpur|Spigot|Folia)|This server is running\s+(\w+)|with\s+(Fabric|Forge|NeoForge)\s+Loader|ModLauncher running.*--(forge|neoforge))`)
)

type Options struct {
	BaseDate time.Time
	Timezone *time.Location
}

type Result struct {
	ServerBrand   string
	ServerVersion string
	StartTime     time.Time
	Events        []model.LagEvent
	LinesParsed   int
}

func Parse(r io.Reader, opts Options) (*Result, error) {
	loc := opts.Timezone
	if loc == nil {
		if !opts.BaseDate.IsZero() {
			loc = opts.BaseDate.Location()
		} else {
			loc = time.Local
		}
	}

	base := opts.BaseDate
	if base.IsZero() {
		now := time.Now().In(loc)
		base = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	} else {
		base = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, loc)
	}

	res := &Result{
		ServerBrand:   "Minecraft",
		ServerVersion: "Unknown",
	}

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024) // up to 1MB line buffer for thread dumps

	var lastTime time.Time
	currentDate := base

	for scanner.Scan() {
		line := scanner.Text()
		res.LinesParsed++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if vMatch := serverVersionRegex.FindStringSubmatch(trimmed); vMatch != nil && res.ServerVersion == "Unknown" {
			res.ServerVersion = vMatch[1]
		}
		if bMatch := serverBrandRegex.FindStringSubmatch(trimmed); bMatch != nil && res.ServerBrand == "Minecraft" {
			for i := 1; i < len(bMatch); i++ {
				if bMatch[i] != "" {
					res.ServerBrand = bMatch[i]
					break
				}
			}
		}

		timeMatches := timePrefixRegex.FindStringSubmatch(trimmed)
		if timeMatches == nil {
			continue
		}

		hour, _ := strconv.Atoi(timeMatches[1])
		min, _ := strconv.Atoi(timeMatches[2])
		sec, _ := strconv.Atoi(timeMatches[3])
		msec := 0
		if timeMatches[4] != "" {
			msec, _ = strconv.Atoi(timeMatches[4])
		}

		lineTime := time.Date(
			currentDate.Year(),
			currentDate.Month(),
			currentDate.Day(),
			hour,
			min,
			sec,
			msec*int(time.Millisecond),
			loc,
		)

		if !lastTime.IsZero() && lineTime.Before(lastTime) && lastTime.Hour() >= 23 && lineTime.Hour() <= 1 {
			currentDate = currentDate.AddDate(0, 0, 1)
			lineTime = lineTime.AddDate(0, 0, 1)
		}
		lastTime = lineTime

		if res.StartTime.IsZero() {
			res.StartTime = lineTime
		}

		if match := cantKeepUpRegex.FindStringSubmatch(trimmed); match != nil {
			ms, _ := strconv.ParseInt(match[1], 10, 64)
			ticks, _ := strconv.Atoi(match[2])

			res.Events = append(res.Events, model.LagEvent{
				Time:        lineTime,
				Duration:    time.Duration(ms) * time.Millisecond,
				TicksBehind: ticks,
				Source:      "TickLoop",
				RawLine:     trimmed,
			})
			continue
		}

		if match := watchdogSingleTickRegex.FindStringSubmatch(trimmed); match != nil {
			secs, _ := strconv.ParseFloat(match[1], 64)
			dur := time.Duration(secs * float64(time.Second))
			ticks := int(secs / 0.05)

			ev := model.LagEvent{
				Time:        lineTime,
				Duration:    dur,
				TicksBehind: ticks,
				Source:      "Watchdog",
				RawLine:     trimmed,
			}
			if n := len(res.Events); n > 0 && res.Events[n-1].Source == "WatchdogFreeze" && lineTime.Sub(res.Events[n-1].Time) <= 5*time.Second {
				res.Events[n-1] = ev
			} else {
				res.Events = append(res.Events, ev)
			}
			continue
		}

		if watchdogStoppedRegex.MatchString(trimmed) {
			if n := len(res.Events); n > 0 && res.Events[n-1].Source == "Watchdog" && lineTime.Sub(res.Events[n-1].Time) <= 5*time.Second {
				continue
			}
			res.Events = append(res.Events, model.LagEvent{
				Time:        lineTime,
				Duration:    10 * time.Second, // default minimum freeze duration before watchdog kicks in
				TicksBehind: 200,
				Source:      "WatchdogFreeze",
				RawLine:     trimmed,
			})
			continue
		}
	}

	return res, scanner.Err()
}
