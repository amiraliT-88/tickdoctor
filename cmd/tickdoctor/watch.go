package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/flags"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/parser/detect"
	"github.com/amiraliT-88/tickdoctor/internal/parser/legacy"
	"github.com/amiraliT-88/tickdoctor/internal/parser/unified"
	"github.com/amiraliT-88/tickdoctor/internal/reader"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
	"github.com/spf13/cobra"
)

type streamParser interface {
	ParseLine(string)
	EventCount() int
	LastEvent() *model.GCEvent
}

func newWatchCmd() *cobra.Command {
	var gcLogPath string
	var pid int
	var pollIntervalMs int

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Continuously stream and monitor JVM GC events and server health in real-time",
		Run: func(cmd *cobra.Command, args []string) {
			engine, err := rules.NewEngine()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing rule engine: %v\n", err)
				os.Exit(3)
			}

			if pid > 0 {
				fmt.Printf("Attaching to Java process PID %d...\n", pid)
				cmdline, err := flags.ReadProcessCommandLine(pid)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to read process %d flags: %v\n", pid, err)
				} else {
					parsed := flags.ParseCommandLine(cmdline)
					fmt.Printf("Detected %d JVM flags from process command line.\n", len(parsed.AllFlags))

					inferredJava := 21
					for _, flg := range parsed.AllFlags {
						if strings.HasPrefix(flg, "-Xlog:gc") {
							if gcLogPath == "" {
								for _, part := range strings.Split(flg, ":") {
									if strings.HasPrefix(part, "file=") {
										gcLogPath = strings.TrimPrefix(part, "file=")
										break
									}
								}
							}
						} else if strings.HasPrefix(flg, "-Xloggc:") {
							inferredJava = 8
							if gcLogPath == "" {
								gcLogPath = strings.TrimPrefix(flg, "-Xloggc:")
							}
						} else if flg == "-XX:+PrintGCDetails" || flg == "-XX:+PrintGCDateStamps" {
							inferredJava = 8
						}
					}

					lintFindings := engine.EvaluateFlags(
						parsed.AllFlags,
						parsed.HeapMin,
						parsed.HeapMax,
						parsed.PreTouch,
						parsed.DisableGC,
						0,
						inferredJava,
					)
					if len(lintFindings) > 0 {
						fmt.Printf("\nPre-flight JVM Flag Audit (%d warnings):\n", len(lintFindings))
						for _, lf := range lintFindings {
							fmt.Printf("  [%s] %s: %s\n", lf.Rule.Severity, lf.Rule.Title, lf.Recommendation)
						}
						fmt.Println()
					}
				}
			}

			logPath := findGCLog(gcLogPath)
			if logPath == "" {
				fmt.Fprintln(os.Stderr, "Error: no GC log found to watch. Specify with --gc-log <path>.")
				os.Exit(3)
			}

			absPath, _ := filepath.Abs(logPath)
			fmt.Printf("Watching JVM GC stream: %s\n", absPath)
			fmt.Println("Press Ctrl+C to stop.")

			ctx, cancel := context.WithCancel(context.Background())
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigChan
				fmt.Println("\nStopping GC watch stream...")
				cancel()
			}()

			poll := time.Duration(pollIntervalMs) * time.Millisecond
			if poll <= 0 {
				poll = 150 * time.Millisecond
			}

			var parser streamParser = unified.NewParser()
			if c, sc, err := reader.OpenStream(absPath); err == nil {
				var head strings.Builder
				for i := 0; i < 50 && sc.Scan(); i++ {
					head.WriteString(sc.Text())
					head.WriteByte('\n')
				}
				c.Close()
				if fmtDetected, err := detect.DetectFormat(strings.NewReader(head.String())); err == nil && fmtDetected == detect.FormatLegacy {
					parser = legacy.NewParser()
				}
			}

			lines, errs := reader.Tail(ctx, absPath, reader.TailOptions{
				PollInterval:  poll,
				FromBeginning: false,
			})

			var totalEvents []*model.GCEvent

			for {
				select {
				case <-ctx.Done():
					if len(totalEvents) > 0 {
						metrics := analyzer.Analyze(totalEvents, 0)
						fmt.Printf("\n--- Watch Session Summary (%d events) ---\n", len(totalEvents))
						fmt.Printf("Avg Pause: %v | Max Pause: %v | Total Time: %v\n",
							formatWatchDuration(metrics.PauseAvg),
							formatWatchDuration(metrics.PauseMax),
							formatWatchDuration(metrics.PauseSum),
						)
						fmt.Printf("Tick Stutters (50-150ms): %d | Severe Freezes (>150ms): %d\n",
							metrics.From50To150, metrics.Above150ms)
					}
					return

				case err, ok := <-errs:
					if ok && err != nil {
						fmt.Fprintf(os.Stderr, "Stream error: %v\n", err)
					}
					return

				case line, ok := <-lines:
					if !ok {
						return
					}

					prevCount := parser.EventCount()
					parser.ParseLine(line)

					if parser.EventCount() > prevCount {
						if latest := parser.LastEvent(); latest != nil {
							totalEvents = append(totalEvents, latest)
							renderWatchEvent(latest)
						}
					}
				}
			}
		},
	}

	cmd.Flags().StringVarP(&gcLogPath, "gc-log", "g", "", "Path to gc.log")
	cmd.Flags().IntVarP(&pid, "pid", "p", 0, "Attach to running Java Minecraft server PID")
	cmd.Flags().IntVarP(&pollIntervalMs, "interval", "i", 150, "Polling interval in milliseconds")
	return cmd
}

func renderWatchEvent(ev *model.GCEvent) {
	timeStr := time.Now().Format("15:04:05")
	if !ev.StartTime.IsZero() {
		timeStr = ev.StartTime.Format("15:04:05")
	}

	var statusTag string
	dur := ev.Duration

	switch {
	case ev.Type == model.GCTypeFull:
		statusTag = "[FULL GC FREEZE]"
	case dur > 150*time.Millisecond:
		statusTag = "[CRITICAL STALL]"
	case dur > 50*time.Millisecond:
		statusTag = "[TICK DROP]"
	default:
		statusTag = "[OK]"
	}

	heapDelta := "-"
	if ev.HeapTotal > 0 || ev.HeapBefore > 0 {
		heapDelta = fmt.Sprintf("%dM->%dM", ev.HeapBefore/(1024*1024), ev.HeapAfter/(1024*1024))
	}

	fmt.Printf("[%s] %-17s GC(%d) %-7s %-8s | Heap: %-13s | %s\n",
		timeStr,
		statusTag,
		ev.ID,
		ev.Type,
		formatWatchDuration(dur),
		heapDelta,
		ev.Reason,
	)
}

func formatWatchDuration(d time.Duration) string {
	if d == 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
