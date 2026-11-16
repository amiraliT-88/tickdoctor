# How TickDoctor Works

TickDoctor is an offline and live diagnostic CLI and TUI tool engineered specifically for Minecraft server administrators. This document details the architectural pipeline, parsing engines, statistical analysis, and correlation algorithms.

---

## Architecture Overview

```
                      ┌──────────────────────┐
                      │    Input Sources     │
                      └──────────┬───────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
          ▼                      ▼                      ▼
    ┌───────────┐          ┌───────────┐          ┌───────────┐
    │  gc.log   │          │latest.log │          │ start.sh  │
    └─────┬─────┘          └─────┬─────┘          └─────┬─────┘
          │                      │                      │
          ▼                      ▼                      ▼
    ┌───────────┐          ┌───────────┐          ┌───────────┐
    │ GC Reader │          │Server Log │          │Flag Parser│
    │  & Tail   │          │  Parser   │          │& Normaliz.│
    └─────┬─────┘          └─────┬─────┘          └─────┬─────┘
          │                      │                      │
          ▼                      │                      │
   Format Detector               │                      │
    ┌─────┴─────┐                │                      │
    │           │                │                      │
    ▼           ▼                │                      │
 Unified     Legacy              │                      │
 (JDK 17+)   (JDK 8)             │                      │
    │           │                │                      │
    └─────┬─────┘                │                      │
          ▼                      ▼                      │
  [GCEvent Stream]        [LagEvent Stream]             │
          │                      │                      │
          ├──────────────────────┘                      │
          ▼                                             │
    ┌───────────┐                                       │
    │Correlator │                                       │
    └─────┬─────┘                                       │
          │                                             │
          ▼                                             ▼
    ┌───────────┐                                 ┌───────────┐
    │ Analyzer  │                                 │   Lint    │
    └─────┬─────┘                                 └─────┬─────┘
          │                                             │
          └──────────────────────┬──────────────────────┘
                                 ▼
                         ┌───────────────┐
                         │  Rule Engine  │
                         └───────┬───────┘
                                 │
                                 ▼
                     ┌──────────────────────┐
                     │ Output Presentations │
                     ├──────────────────────┤
                     │ • Interactive TUI    │
                     │ • Clean Text Report  │
                     │ • Markdown / Discord │
                     │ • Structured JSON    │
                     └──────────────────────┘
```

---

## 1. Streaming Reader (`internal/reader`)

- **Constant Memory Footprint:** Log files from long-running production servers can easily grow to several hundred megabytes. TickDoctor avoids loading entire logs into RAM, streaming token buffers (up to 2MB) through `bufio.Scanner`.
- **Gzip Ingestion:** Compressed rotated archives (`.log.gz`, `.log.0.gz`) are decompressed transparently on-the-fly.
- **Rotation Sorting:** File ordering accounts for generation counters (`gc.log.0` vs `gc.log.current`).
- **Live Following:** In `watch` mode, `reader.Tail` mimics `tail -f`, polling for appended lines while actively tracking truncation and file swaps.

---

## 2. Multi-Generation GC Parsers (`internal/parser`)

TickDoctor normalizes GC telemetry across two radically different Java generations:

### Unified Logging (Java 9, 17, 21+)
- Parses `-Xlog:gc*` tags, extracting ISO8601 wall timestamps, JVM uptimes, GC IDs (`GC(42)`), heap generation transformations (`Before -> After (Total)`), and pause durations in milliseconds.
- Supports G1, ZGC (Generational and Single-Generation), and Parallel.

### Legacy Logging (Java 8)
- Parses classic `-XX:+PrintGCDetails` and `-XX:+PrintGCDateStamps`.
- Extracts startup parameters from the `CommandLine flags:` header block embedded directly in Java 8 logs.
- Supports Parallel GC (`[PSYoungGen: ...]`), G1, and CMS.

---

## 3. Minecraft Tick Budget Analysis (`internal/analyzer`)

Minecraft's tick loop runs at a fixed target of 20 ticks per second (TPS). This leaves exactly **50ms** per tick. While standard enterprise web services consider 100ms–200ms GC pauses completely normal, any Stop-The-World pause in Minecraft directly blocks the server thread.

TickDoctor buckets every pause into specific tick impact brackets:
- **$< 25\text{ms}$ (Healthy):** Safely fits inside the remaining tick budget without dropping TPS.
- **$25\text{ms} - 50\text{ms}$ (Borderline):** Leaves minimal CPU headroom for entity ticking and physics.
- **$50\text{ms} - 150\text{ms}$ (Micro-stutter):** Causes noticeable micro-freezes and drops 1–3 ticks.
- **$> 150\text{ms}$ (Severe freeze):** Causes rubber-banding, input delay, and risks triggering Spigot/Paper Watchdog timeouts.

---

## 4. Lag Correlation Engine (`internal/correlate`)

The most critical capability of TickDoctor is distinguishing between **GC Stop-The-World lag** and **Main-Thread Plugin / Entity lag**.

1. **Log Alignment:** Ingests tick overload warnings (`Can't keep up! Running Xms or Y ticks behind`) and Watchdog freezes from `latest.log`.
2. **Temporal Windowing:** Anchors server log local time (`HH:MM:SS`) to the GC log date. For each lag spike, it checks whether an active Stop-The-World pause occurred during the tick calculation window.
3. **Definitive Verdict:**
   - **`GC_PRIMARY`:** $> 50\%$ of server lag duration coincides with GC pauses. Tuning JVM flags will directly resolve server lag.
   - **`GC_PARTIAL`:** $20\% - 50\%$ coincidence. Both JVM tuning and plugin profiling are advised.
   - **`GC_HEALTHY`:** $< 20\%$ coincidence. The JVM is performing normally; lag originates from heavy plugins, redstone, or entity counts on the main thread. TickDoctor advises profiling with `spark` (`/spark sampler`).
