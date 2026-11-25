package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amiraliT-88/tickdoctor/internal/analyzer"
	"github.com/amiraliT-88/tickdoctor/internal/correlate"
	"github.com/amiraliT-88/tickdoctor/internal/flags"
	"github.com/amiraliT-88/tickdoctor/internal/model"
	"github.com/amiraliT-88/tickdoctor/internal/parser/detect"
	"github.com/amiraliT-88/tickdoctor/internal/parser/legacy"
	"github.com/amiraliT-88/tickdoctor/internal/parser/serverlog"
	"github.com/amiraliT-88/tickdoctor/internal/parser/unified"
	"github.com/amiraliT-88/tickdoctor/internal/reader"
	"github.com/amiraliT-88/tickdoctor/internal/render/json"
	"github.com/amiraliT-88/tickdoctor/internal/render/markdown"
	"github.com/amiraliT-88/tickdoctor/internal/render/text"
	"github.com/amiraliT-88/tickdoctor/internal/render/tui"
	"github.com/amiraliT-88/tickdoctor/internal/rules"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

func main() {
	rootCmd := &cobra.Command{
		Use:     "tickdoctor",
		Short:   "JVM GC pause and lag diagnosis tool for Minecraft servers",
		Version: version,
	}

	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newLintCmd())
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newWatchCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(3)
	}
}

func newAnalyzeCmd() *cobra.Command {
	var gcLogPath string
	var serverLogPath string
	var cmdPath string
	var format string
	var outputPath string
	var redact bool
	var noTUI bool

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze GC log and report pause metrics and suggested JVM flag fixes",
		Run: func(cmd *cobra.Command, args []string) {
			executeAnalysis(gcLogPath, serverLogPath, cmdPath, format, outputPath, redact, noTUI)
		},
	}

	cmd.Flags().StringVarP(&gcLogPath, "gc-log", "g", "", "Path to gc.log or server directory")
	cmd.Flags().StringVarP(&serverLogPath, "server-log", "s", "", "Path to latest.log to correlate lag spikes with GC pauses")
	cmd.Flags().StringVarP(&cmdPath, "cmd", "c", "", "Path to startup script (start.sh/start.bat) to combine with GC log analysis")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, markdown, json")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (defaults to stdout)")
	cmd.Flags().BoolVar(&redact, "redact", false, "Sanitize player IPs and sensitive identifiers in output")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "Disable interactive TUI and output plain text")
	return cmd
}

func newReportCmd() *cobra.Command {
	var gcLogPath string
	var serverLogPath string
	var cmdPath string
	var format string
	var outputPath string
	var redact bool

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate non-interactive diagnostic report (text, markdown, json)",
		Run: func(cmd *cobra.Command, args []string) {
			executeAnalysis(gcLogPath, serverLogPath, cmdPath, format, outputPath, redact, true)
		},
	}

	cmd.Flags().StringVarP(&gcLogPath, "gc-log", "g", "", "Path to gc.log or server directory")
	cmd.Flags().StringVarP(&serverLogPath, "server-log", "s", "", "Path to latest.log to correlate lag spikes with GC pauses")
	cmd.Flags().StringVarP(&cmdPath, "cmd", "c", "", "Path to startup script (start.sh/start.bat) to combine with GC log analysis")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: text, markdown, json")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (defaults to stdout)")
	cmd.Flags().BoolVar(&redact, "redact", true, "Sanitize player IPs and sensitive identifiers in output")
	return cmd
}

func executeAnalysis(gcLogPath, serverLogPath, cmdPath, format, outputPath string, redact, noTUI bool) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "", "text", "markdown", "md", "json":
	default:
		fmt.Fprintf(os.Stderr, "Error: unsupported output format %q (valid: text, markdown, json)\n", format)
		os.Exit(3)
	}

	path := findGCLog(gcLogPath)
	if path == "" {
		fmt.Fprintln(os.Stderr, "Error: no GC log found. Specify with --gc-log <path> or run inside a Minecraft server directory.")
		fmt.Fprintln(os.Stderr, "Tip: If GC logging is not enabled yet, run 'tickdoctor setup' for instructions.")
		os.Exit(3)
	}

	files, err := reader.DiscoverRotatedLogs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading logs: %v\n", err)
		os.Exit(3)
	}

	logFormat := detect.FormatUnified
	if firstCloser, firstScanner, err := reader.OpenStream(files[0]); err == nil {
		var headLines strings.Builder
		for i := 0; i < 50 && firstScanner.Scan(); i++ {
			headLines.WriteString(firstScanner.Text())
			headLines.WriteByte('\n')
		}
		firstCloser.Close()
		if detected, err := detect.DetectFormat(strings.NewReader(headLines.String())); err == nil && detected != detect.FormatUnknown {
			logFormat = detected
		}
	}

	var events []*model.GCEvent
	var jvm model.JVMInfo

	if logFormat == detect.FormatLegacy {
		lp := legacy.NewParser()
		for _, f := range files {
			closer, scanner, err := reader.OpenStream(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open %s: %v\n", f, err)
				continue
			}
			for scanner.Scan() {
				lp.ParseLine(scanner.Text())
			}
			closer.Close()
		}
		events = lp.Events()
		jvm = lp.JVM
	} else {
		up := unified.NewParser()
		for _, f := range files {
			closer, scanner, err := reader.OpenStream(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to open %s: %v\n", f, err)
				continue
			}
			for scanner.Scan() {
				up.ParseLine(scanner.Text())
			}
			closer.Close()
		}
		events = up.Events()
		jvm = up.JVM
	}

	if len(events) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no recognizable GC events found in %s\n", path)
		os.Exit(3)
	}

	metrics := analyzer.Analyze(events, 0)

	engine, err := rules.NewEngine()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing rule engine: %v\n", err)
		os.Exit(3)
	}

	findings := engine.Evaluate(metrics, jvm)

	if script := findStartupScriptForLog(cmdPath, path); script != "" {
		if parsed, err := flags.ParseScriptOrString(script); err == nil {
			if len(jvm.RawFlags) == 0 {
				jvm.RawFlags = parsed.AllFlags
			}
			if jvm.HeapInitial == 0 && parsed.HeapMin > 0 {
				jvm.HeapInitial = parsed.HeapMin
			}
			if jvm.HeapMax == 0 && parsed.HeapMax > 0 {
				jvm.HeapMax = parsed.HeapMax
			}
			if jvm.AlwaysPreTouch == nil && parsed.PreTouch != nil {
				jvm.AlwaysPreTouch = parsed.PreTouch
			}

			javaVer := jvm.MajorVersion
			if javaVer == 0 {
				javaVer = 21
			}
			flagFindings := engine.EvaluateFlags(
				parsed.AllFlags,
				parsed.HeapMin,
				parsed.HeapMax,
				parsed.PreTouch,
				parsed.DisableGC,
				jvm.PhysicalMemory,
				javaVer,
			)
			seen := make(map[string]bool, len(findings))
			for _, f := range findings {
				seen[f.Rule.ID] = true
			}
			for _, ff := range flagFindings {
				if ff.Rule.ID == "GC-001" || seen[ff.Rule.ID] {
					continue
				}
				findings = append(findings, ff)
				seen[ff.Rule.ID] = true
			}
		}
	}

	var corrReport *correlate.Report
	sLog := serverLogPath
	if sLog == "" {
		sLog = findServerLog(path)
	}
	if sLog != "" {
		if sCloser, sScanner, err := reader.OpenStream(sLog); err == nil {
			var baseDate time.Time
			for _, ev := range events {
				if !ev.StartTime.IsZero() {
					baseDate = ev.StartTime
					break
				}
			}
			var sLines strings.Builder
			for sScanner.Scan() {
				sLines.WriteString(sScanner.Text())
				sLines.WriteByte('\n')
			}
			sCloser.Close()

			if sRes, err := serverlog.Parse(strings.NewReader(sLines.String()), serverlog.Options{BaseDate: baseDate}); err == nil && len(sRes.Events) > 0 {
				derefEvents := make([]model.GCEvent, len(events))
				for i, e := range events {
					derefEvents[i] = *e
				}
				corrReport = correlate.Correlate(derefEvents, sRes.Events, correlate.Options{})
			}
		}
	}

	var outFile *os.File
	var outWriter = os.Stdout
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file %s: %v\n", outputPath, err)
			os.Exit(3)
		}
		outFile = f
		outWriter = f
	}

	isTTY := false
	if fi, err := os.Stdout.Stat(); err == nil {
		isTTY = (fi.Mode() & os.ModeCharDevice) != 0
	}

	if format == "json" {
		if err := json.Render(outWriter, jvm, metrics, findings, corrReport); err != nil {
			if outFile != nil {
				outFile.Close()
			}
			fmt.Fprintf(os.Stderr, "Error rendering JSON: %v\n", err)
			os.Exit(3)
		}
	} else if format == "markdown" || format == "md" {
		markdown.Render(outWriter, jvm, metrics, findings, corrReport, markdown.Options{Redact: redact})
	} else if !noTUI && outputPath == "" && isTTY {
		if err := tui.RunWithCorrelation(jvm, metrics, findings, events, corrReport); err != nil {
			text.Render(os.Stdout, jvm, metrics, findings)
			if corrReport != nil {
				text.RenderCorrelation(os.Stdout, corrReport)
			}
		}
	} else {
		text.Render(outWriter, jvm, metrics, findings)
		if corrReport != nil {
			text.RenderCorrelation(outWriter, corrReport)
		}
	}

	if outFile != nil {
		if err := outFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing output file %s: %v\n", outputPath, err)
			os.Exit(3)
		}
	}

	for _, f := range findings {
		if f.Rule.Severity == rules.SeverityCritical {
			os.Exit(2)
		}
	}
	if len(findings) > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func newSetupCmd() *cobra.Command {
	var javaVersion int

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Print recommended JVM startup flags to enable GC logging",
		Run: func(cmd *cobra.Command, args []string) {
			if javaVersion <= 8 {
				fmt.Println("Recommended GC logging flags for Java 8 (with rotation & timestamp):")
				fmt.Println()
				fmt.Println("  -XX:+PrintGCDetails -XX:+PrintGCDateStamps -Xloggc:logs/gc.log -XX:+UseGCLogFileRotation -XX:NumberOfGCLogFiles=5 -XX:GCLogFileSize=20M")
				return
			}

			fmt.Printf("Recommended GC logging flags for Java %d+ (with rotation & timestamp):\n", javaVersion)
			fmt.Println()
			fmt.Println("  -Xlog:gc*,gc+phases=debug,gc+age=trace:file=logs/gc.log:time,uptime,pid,level,tags:filecount=5,filesize=20M")
			fmt.Println()
			fmt.Println("Add this line before '-jar' in your start.sh, start.bat, or user_jvm_args.txt file.")
		},
	}

	cmd.Flags().IntVarP(&javaVersion, "java", "j", 21, "Java major version (e.g. 8, 17, 21)")
	return cmd
}

func newLintCmd() *cobra.Command {
	var cmdInput string
	var ramInput string
	var javaVersion int

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Audit startup script or JVM command flags without log files",
		Run: func(cmd *cobra.Command, args []string) {
			target := findStartupScript(cmdInput)
			if target == "" {
				fmt.Fprintln(os.Stderr, "Error: no startup command or script specified.")
				fmt.Fprintln(os.Stderr, "Usage: tickdoctor lint --cmd <path-or-string> [--ram 16G] [--java 21]")
				os.Exit(3)
			}

			parsed, err := flags.ParseScriptOrString(target)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading startup script: %v\n", err)
				os.Exit(3)
			}

			systemRAM := flags.ParseSize(ramInput)

			engine, err := rules.NewEngine()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error initializing rule engine: %v\n", err)
				os.Exit(3)
			}

			findings := engine.EvaluateFlags(
				parsed.AllFlags,
				parsed.HeapMin,
				parsed.HeapMax,
				parsed.PreTouch,
				parsed.DisableGC,
				systemRAM,
				javaVersion,
			)

			fmt.Println("=== TickDoctor Startup Flags Audit ===")
			fmt.Println()
			fmt.Printf("Target Script / Command: %s\n", target)
			if parsed.Collector != "" {
				fmt.Printf("  Collector:  %s\n", parsed.Collector)
			}
			if parsed.HeapMax > 0 {
				fmt.Printf("  Max Heap:   %d MB\n", parsed.HeapMax/(1024*1024))
			}
			if parsed.HeapMin > 0 {
				fmt.Printf("  Init Heap:  %d MB\n", parsed.HeapMin/(1024*1024))
			}
			if systemRAM > 0 {
				fmt.Printf("  Host RAM:   %d MB\n", systemRAM/(1024*1024))
			}
			fmt.Println()

			if len(findings) == 0 {
				fmt.Println("Result: [HEALTHY] No flag conflicts or anti-patterns detected.")
				os.Exit(0)
			}

			fmt.Printf("Detected Issues (%d):\n", len(findings))
			var actionPlan []string
			hasCritical := false

			for _, f := range findings {
				fmt.Printf("  [%s] %s (%s)\n", f.Rule.Severity, f.Rule.Title, f.Rule.ID)
				if f.Evidence != "" {
					fmt.Printf("    Evidence:       %s\n", f.Evidence)
				}
				if f.Recommendation != "" {
					fmt.Printf("    Recommendation: %s\n", f.Recommendation)
					actionPlan = append(actionPlan, f.Recommendation)
				}
				if f.Rule.Explanation != "" {
					fmt.Printf("    Why:            %s\n", f.Rule.Explanation)
				}
				fmt.Println()
				if f.Rule.Severity == rules.SeverityCritical {
					hasCritical = true
				}
			}

			if len(actionPlan) > 0 {
				fmt.Println("Suggested Action Plan:")
				for _, a := range actionPlan {
					fmt.Printf("  * %s\n", a)
				}
				fmt.Println()
			}

			if hasCritical {
				os.Exit(2)
			}
			os.Exit(1)
		},
	}

	cmd.Flags().StringVarP(&cmdInput, "cmd", "c", "", "Path to startup script (start.sh/start.bat) or raw java command line")
	cmd.Flags().StringVarP(&ramInput, "ram", "r", "", "Host machine total RAM size (e.g. 16G, 32G)")
	cmd.Flags().IntVarP(&javaVersion, "java", "j", 21, "Target Java major version (defaults to 21)")
	return cmd
}

func findStartupScript(explicit string) string {
	return findStartupScriptForLog(explicit, "")
}

func findStartupScriptForLog(explicit, gcLogPath string) string {
	if explicit != "" {
		return explicit
	}

	names := []string{
		"start.sh",
		"start.bat",
		"run.sh",
		"run.bat",
		"user_jvm_args.txt",
	}

	var dirs []string
	if gcLogPath != "" {
		dir := filepath.Dir(gcLogPath)
		if fi, err := os.Stat(gcLogPath); err == nil && fi.IsDir() {
			dir = gcLogPath
		}
		dirs = append(dirs, dir, filepath.Dir(dir))
	}
	dirs = append(dirs, ".")

	for _, d := range dirs {
		for _, n := range names {
			p := filepath.Join(d, n)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
	}
	return ""
}

func findGCLog(explicit string) string {
	if explicit != "" {
		return explicit
	}

	candidates := []string{
		"logs/gc.log",
		"gc.log",
		"logs/gc.log.0",
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

func findServerLog(gcLogPath string) string {
	candidates := []string{
		"latest.log",
		"logs/latest.log",
	}
	if gcLogPath != "" {
		dir := filepath.Dir(gcLogPath)
		if fi, err := os.Stat(gcLogPath); err == nil && fi.IsDir() {
			dir = gcLogPath
		}
		candidates = append([]string{
			filepath.Join(dir, "latest.log"),
			filepath.Join(filepath.Dir(dir), "logs", "latest.log"),
			filepath.Join(filepath.Dir(dir), "latest.log"),
		}, candidates...)
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
