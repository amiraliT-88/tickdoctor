# Contributing Diagnostic Rules

TickDoctor is designed so that new JVM rules, Minecraft-specific checks, and collector heuristics can be contributed with **zero Go code changes** by adding a YAML file to `internal/rules/builtin/`.

---

## Rule YAML Structure

Every rule lives in its own file in `internal/rules/builtin/<name>.yaml`.

```yaml
id: GC-006
title: Young Generation Too Small
severity: WARNING
collector: G1
condition_type: young_frequency
threshold: 5
recommendation: increase -XX:G1NewSizePercent=30
explanation: Young collections are firing more than 5 times per second, spending excessive CPU on survivor copies.
doc_url: https://docs.papermc.io/paper/aikars-flags
```

### Fields

| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique identifier (e.g., `GC-xxx`, `JAVA8-xxx`, `LINT-xxx`, `ZGC-xxx`). |
| `title` | `string` | Human-readable title summarizing the issue. |
| `severity` | `string` | `CRITICAL`, `WARNING`, or `INFO`. |
| `collector` | `string` | `G1`, `Parallel`, `CMS`, `ZGC`, or `All`. |
| `condition_type` | `string` | Evaluator condition key registered in `internal/rules/engine.go`. |
| `threshold` | `float` | Metric threshold required to trigger the rule. |
| `recommendation` | `string` | Specific JVM flag action (e.g. `add -XX:...`, `increase -Xmx`). |
| `explanation` | `string` | Clear, jargon-free explanation for server owners. |
| `doc_url` | `string` | Authoritative documentation reference link. |

---

## Adding a New Condition Type

If your rule requires a new metric not currently exposed by `analyzer.Metrics`, you can add a field to `internal/analyzer/analyzer.go` and implement the condition evaluation in `internal/rules/engine.go`:

```go
case "your_condition_type":
    if m.YourMetric >= int(r.Threshold) {
        triggered = true
        evidence = fmt.Sprintf("Your metric value: %d", m.YourMetric)
    }
```

---

## Submitting a Pull Request

1. Add your rule file in `internal/rules/builtin/<id>_<name>.yaml`.
2. Add a unit test verifying rule activation in `internal/rules/engine_test.go`.
3. Document the rule in `docs/rules.md`.
4. Run `go test ./...` to ensure 100% test passage.
5. Submit a pull request with an example log snippet demonstrating the issue.
