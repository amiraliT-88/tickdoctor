# TickDoctor Diagnostic Rules Reference

TickDoctor uses an embedded, YAML-driven diagnostic rule engine to identify performance anti-patterns, misconfigurations, and root causes of JVM Stop-The-World (STW) pauses.

---

## 1. Evidence-Based GC Rules (Log Analysis)

These rules trigger when concrete evidence of a performance degradation pattern is discovered in the GC log stream.

### `GC-001`: Explicit System.gc() Pause Detected
- **Severity:** `CRITICAL`
- **Collector:** All (G1, ZGC, Parallel, CMS)
- **Target Java:** All
- **Condition:** Occurrences of `System.gc()` Full STW pauses $\ge 1$.
- **Why It Matters:** A plugin or library is invoking `System.gc()`, forcing a synchronous Stop-The-World cycle that freezes Minecraft server ticking for hundreds or thousands of milliseconds.
- **Recommended Action:** Add `-XX:+DisableExplicitGC` to JVM arguments.
- **Reference:** [PaperMC Aikar's Flags](https://docs.papermc.io/paper/aikars-flags)

---

### `GC-002`: G1 Evacuation Failure (To-Space Exhausted)
- **Severity:** `CRITICAL`
- **Collector:** G1
- **Target Java:** Java 8, 11, 17, 21+
- **Condition:** Evacuation failure or to-space exhaustion events $\ge 1$.
- **Why It Matters:** The G1 collector ran out of survivor regions during an evacuation phase. When this occurs, G1 performs an in-place fallback compaction, causing extreme tick stalls (often 300ms–2000ms).
- **Recommended Action:** Increase `-Xmx` or tune reserve survivor space: `-XX:G1ReservePercent=15`.
- **Reference:** [Oracle G1 GC Tuning Guide](https://docs.oracle.com/en/java/javase/21/gctuning/garbage-first-g1-garbage-collector1.html)

---

### `GC-003`: Frequent Humongous Object Allocations
- **Severity:** `WARNING`
- **Collector:** G1
- **Target Java:** Java 8, 11, 17, 21+
- **Condition:** Humongous region allocation count $\ge 20$.
- **Why It Matters:** Any object larger than 50% of the G1 region size is allocated directly in the Old Generation as contiguous humongous regions. This bypasses Eden, triggers premature mixed collection cycles, and fragments heap memory.
- **Recommended Action:** Increase region size with `-XX:G1HeapRegionSize=16M` (or `32M`).
- **Reference:** [Oracle G1 Region Sizing](https://docs.oracle.com/en/java/javase/21/gctuning/garbage-first-g1-garbage-collector1.html#GUID-0CD10D08-2A27-4632-B833-D66D8E42FA54)

---

### `GC-004`: Initial and Max Heap Mismatch
- **Severity:** `WARNING`
- **Collector:** All
- **Target Java:** All
- **Condition:** `-Xms` is configured with a different value than `-Xmx`.
- **Why It Matters:** When `-Xms` is smaller than `-Xmx`, the JVM dynamically requests and releases memory pages from the operating system during player spikes and chunk loading. Heap resizing introduces latency spikes.
- **Recommended Action:** Set `-Xms` equal to `-Xmx` (e.g. `-Xms8G -Xmx8G`).
- **Reference:** [PaperMC Aikar's Flags](https://docs.papermc.io/paper/aikars-flags)

---

### `GC-005`: AlwaysPreTouch Flag Disabled
- **Severity:** `INFO`
- **Collector:** All
- **Target Java:** All
- **Condition:** Pre-touch is reported as `Disabled` in the JVM configuration header.
- **Why It Matters:** Without pre-touching, the OS allocates virtual memory lazily and commits physical RAM pages only upon first write. This introduces micro-stutters during gameplay as new chunks and entities are instantiated.
- **Recommended Action:** Add `-XX:+AlwaysPreTouch`.
- **Reference:** [PaperMC Aikar's Flags](https://docs.papermc.io/paper/aikars-flags)

---

### `JAVA8-001`: Java 8 Parallel Collector Active
- **Severity:** `WARNING`
- **Collector:** Parallel (or unset default on Java 8)
- **Target Java:** Java 8
- **Condition:** Major version is 8 and Parallel GC causes Full collections or pauses $> 150\text{ms}$.
- **Why It Matters:** Java 8 defaults to the Parallel collector (throughput-focused), which stops the world on all application threads for full heap compaction.
- **Recommended Action:** Migrate to G1 GC: `-XX:+UseG1GC`.
- **Reference:** [PaperMC Optimization Guide](https://docs.papermc.io/paper/aikars-flags)

---

### `JAVA8-002`: CMS Concurrent Mode Failure
- **Severity:** `CRITICAL`
- **Collector:** CMS
- **Target Java:** Java 8
- **Condition:** CMS Concurrent Mode Failure occurrences $\ge 1$.
- **Why It Matters:** The CMS collector was unable to reclaim memory before the Old Generation filled up, causing a catastrophic single-threaded fallback Full GC that freezes the server for multiple seconds.
- **Recommended Action:** Migrate to G1 GC (`-XX:+UseG1GC`) or initiate concurrent cycles earlier with `-XX:CMSInitiatingOccupancyFraction=70`.
- **Reference:** [Oracle CMS Tuning Guide](https://docs.oracle.com/javase/8/docs/technotes/guides/vm/gctuning/cms.html)

---

### `ZGC-001`: ZGC Allocation Stall Detected
- **Severity:** `WARNING`
- **Collector:** ZGC
- **Target Java:** Java 21+
- **Condition:** ZGC allocation stall events $\ge 1$.
- **Why It Matters:** Mutator threads (e.g., Minecraft Server thread) allocated memory faster than concurrent ZGC background threads could reclaim pages, forcing application threads into a stall.
- **Recommended Action:** Increase `-Xmx` or allocate more concurrent workers with `-XX:ConcGCThreads`.
- **Reference:** [Oracle Z Garbage Collector Guide](https://docs.oracle.com/en/java/javase/21/gctuning/z-garbage-collector.html)

---

## 2. Zero-Log Lint Rules (Static Flag Audit)

These rules run via `tickdoctor lint` to audit startup scripts (`start.sh`, `start.bat`, `user_jvm_args.txt`) without requiring any log files.

### `LINT-001`: Conflicting Garbage Collectors
- **Severity:** `CRITICAL`
- **Condition:** Multiple garbage collector activation flags specified simultaneously (e.g., `-XX:+UseG1GC` and `-XX:+UseParallelGC`).
- **Why It Matters:** The JVM will pick whichever collector is parsed last, creating confusion or unpredictable collector behavior across server restarts.
- **Recommended Action:** Specify only one primary collector flag (e.g., `-XX:+UseG1GC`).

---

### `LINT-002`: Removed or Deprecated JVM Flags
- **Severity:** `WARNING`
- **Condition:** Outdated flags like `-XX:+UseConcMarkSweepGC`, `-Xincgc`, or `-XX:MaxPermSize` present when running on Java 17+.
- **Why It Matters:** CMS was removed in Java 14; PermGen was removed in Java 8. Passing these flags either causes startup failure or generates noisy warning messages on modern runtimes.
- **Recommended Action:** Remove obsolete flags and adopt modern Aikar's flags for G1.

---

### `LINT-003`: Unsafe Heap Allocation Relative to Host RAM
- **Severity:** `CRITICAL`
- **Condition:** `-Xmx` exceeds physical host RAM, or leaves $< 1536\text{ MB}$ of headroom for OS and off-heap memory.
- **Why It Matters:** Minecraft uses significant off-heap memory for Netty network buffers, native decompression, JVM thread stacks, and Metaspace. Starving the host OS triggers OS page-swapping or Linux OOM-killer kernel crashes.
- **Recommended Action:** Reduce `-Xmx` to leave at least 1.5GB to 2GB free for host OS and off-heap allocations.
