# TickDoctor

[![CI](https://github.com/amiraliT-88/tickdoctor/actions/workflows/ci.yml/badge.svg)](https://github.com/amiraliT-88/tickdoctor/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/amiraliT-88/tickdoctor)](https://github.com/amiraliT-88/tickdoctor/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/amiraliT-88/tickdoctor)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A local CLI and TUI tool for diagnosing JVM garbage collection pauses and lag spikes on Minecraft servers.

Written in Go. It reads your `gc.log`, `latest.log`, and startup flags to answer whether server freezes are actually caused by GC, show the exact log evidence, and give you a flag diff to fix them.

## When this helps (and when it doesn't)

Minecraft lag usually falls into two buckets:

- **Sudden freezes and micro-stutters** (500ms+ tick spikes, periodic freezes, Watchdog crashes): often caused by GC pauses or misconfigured heap flags. **TickDoctor helps here.**
- **Steady low TPS** (e.g. a constant 13 TPS): usually caused by entities, redstone, chunk generation, or a heavy plugin on the main thread. **This is [spark](https://spark.lucko.me/)'s job, not TickDoctor's.**

TickDoctor compares timestamps between `gc.log` and `latest.log`. If GC pauses don't line up with your `Can't keep up!` warnings, it will tell you straight up that your GC looks fine and point you toward profiling with spark instead of guessing new JVM flags.

## Features

- **Java 8 and Java 17+ support:** Handles both legacy Java 8 logs (`Parallel`, `G1`, `CMS`) and Java 17+ unified logging (`G1`, Generational `ZGC`).
- **Missing & broken flag detection:** Combines your `start.sh` / `start.bat` / `user_jvm_args.txt` with what's recorded in the GC log header to catch missing flags, outdated Java 8 flags left on Java 21, and unsafe heap sizes.
- **Zero-log `lint` mode:** Checks startup scripts for deprecated flags, conflicting collectors, or dangerous `-Xmx` values in seconds without needing a GC log first.
- **Lag correlation:** Lines up `Can't keep up!` and Spigot/Paper Watchdog entries in `latest.log` against GC stop-the-world pauses.
- **Read-only & local:** Never modifies your startup scripts automatically and never uploads your logs anywhere. Use `--redact` to strip player IPs when exporting reports to Discord.

## Commands

| Command | Description |
| :--- | :--- |
| `setup` | Prints the exact GC logging flags for your Java version. |
| `lint` | Audits a startup command or script (`start.sh`, `start.bat`, `user_jvm_args.txt`) without any log files. |
| `analyze` | Parses `gc.log` (including rotated and `.gz` files) and opens the interactive TUI. |
| `watch` | Tails an active server's GC log (or inspects a running `--pid`) live in the terminal. |
| `report` | Outputs a non-interactive report in `text`, `markdown`, or `json`. |

## Example output

```text
Verdict: GC is responsible for 78% of recorded lag spikes (11/14 events matched)

[CRITICAL] 14 Full GC pauses (avg 1.8s) triggered by System.gc()
  evidence: gc.log lines 2041, 2378, 2901 ...
  fix:      add -XX:+DisableExplicitGC

[WARNING]  312 humongous allocations with 4M region size
  evidence: gc.log (24.3% of Young/Mixed pauses included humongous objects)
  fix:      change -XX:G1HeapRegionSize=4M -> -XX:G1HeapRegionSize=16M

Suggested startup diff:
  + -XX:+DisableExplicitGC
  ~ -XX:G1HeapRegionSize=16M
```

## Usage

Check your startup script right away without a GC log:

```sh
tickdoctor lint --cmd ./start.sh --ram 16G
```

Generate the right GC logging flags if you haven't enabled them yet:

```sh
tickdoctor setup --java 21
```

Run inside a server directory (auto-detects `./logs/gc.log`, `./logs/latest.log`, and `./start.sh` / `./start.bat`):

```sh
tickdoctor analyze
```

Pass paths explicitly or export a redacted Markdown report for Discord:

```sh
tickdoctor report --gc-log ./logs/gc.log --server-log ./logs/latest.log --format markdown --redact
```

## Documentation

- [Diagnostic Rules Reference](docs/rules.md) — Complete list of all built-in GC and flag rules with explanations and documentation links.
- [How It Works](docs/how-it-works.md) — Technical deep-dive into the streaming reader, multi-generation parsers, and lag correlation algorithm.
- [Contributing Rules](docs/contributing-rules.md) — Guide for adding new diagnostic YAML heuristics.

## Supported targets

- **Java 17, 21, 25 (`-Xlog:gc*`):** G1GC, Generational ZGC
- **Java 8 (`-XX:+PrintGCDetails`):** Parallel GC, G1GC, CMS
- **Server logs:** Vanilla, Paper, Purpur, Spigot, Folia, Fabric, Forge

## Build from source

Requires Go 1.24 or newer:

```sh
git clone https://github.com/amiraliT-88/tickdoctor.git
cd tickdoctor
go build ./cmd/tickdoctor
```

## License

[MIT](LICENSE)
