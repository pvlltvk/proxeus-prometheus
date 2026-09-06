# proxeus-prometheus

A patched fork of [prometheus/prometheus](https://github.com/prometheus/prometheus), maintained solely as a dependency of
[proxeus](https://github.com/pvlltvk/proxeus).

**Do not depend on this directly.** It exists because proxeus needs PromQL engine internals that upstream Prometheus does
not expose. It tracks upstream releases and carries a single patch on top of them.

## Why this fork exists

proxeus federates several Prometheus-compatible backends (Thanos, VictoriaMetrics, …) behind one PromQL endpoint. To
avoid pulling every raw sample across the network, it rewrites the query AST and *pushes aggregations down* to each
backend, then re-combines the per-backend partials locally.

That requires a hook inside the PromQL engine to substitute nodes during evaluation, plus a few supporting changes.
Upstream has no such extension point.

## What is patched

A short patch series on top of the upstream release tag, touching 7 files:

| file | change |
|---|---|
| `promql/engine.go` | adds the `NodeReplacer` field to `Engine`; `FindMinMaxTime` becomes the method `(*Engine).findMinMaxTime`; `newQuery`/`populateSeries` propagate errors; range tracking made safe for a parallel tree walk |
| `promql/parser/ast.go` | `parser.Inspect` gains a `context.Context`, takes an `*EvalStmt`, returns an error, and accepts a `NodeReplacer`; adds the `NodeReplacer` type; `Walk` may visit children in parallel, `Inspect` never does |
| `promql/engine_extra.go` | new file: per-selector `LookbackDelta` support |
| `promql/info.go` | call-site updates for the new `Inspect` signature |
| `promql/promqltest/test.go` | call-site updates |
| `rules/group.go` | call-site updates |
| `web/web.go` | adds `(*Handler).HTTPHandler`, the mux `Run` serves, so the handler can be mounted in another server instead of being reverse-proxied |

## Why it is not upstreamable

The patch changes the signature of `parser.Inspect`, a widely-used exported function, and converts the exported
`FindMinMaxTime` into an unexported method. Both are breaking API changes across the whole Prometheus tree, made to
serve a use case (federating proxy with aggregation pushdown) that upstream does not have. Upstream would reasonably
reject it. Carrying it as a fork is the same approach Grafana Mimir takes with
[`grafana/mimir-prometheus`](https://github.com/grafana/mimir-prometheus), and that promxy took with
`jacksontj/prometheus`.

## How proxeus consumes it

In proxeus's `go.mod`:

```
replace github.com/prometheus/prometheus => github.com/pvlltvk/proxeus-prometheus v0.305.0-proxeus.3
```

This fork's own `go.mod` deliberately keeps `module github.com/prometheus/prometheus`. That is required — every internal
import path in the tree depends on it, and rewriting them would force two incompatible copies of every Prometheus type
into the dependency graph. Go permits a `replace` target whose declared module path differs from the replacement path.

Note that Go only honours `replace` directives in the **main** module. Anything importing proxeus as a *library* must
copy the directive above into its own `go.mod`.

## Versioning

Prometheus ships **two** tag series from the same commits: a product tag (`v3.5.0`) and a Go module tag (`v0.305.0`).
Both point at the same object. The Go module path `github.com/prometheus/prometheus` has no `/v3` suffix, so Go only
accepts `v0.x`/`v1.x` versions — `v3.5.0` is not a usable module version, which is why upstream publishes the `v0.3NN.M`
series (product `3.13.1` → module `v0.313.1`).

This fork therefore tags **`v0.3NN.M-proxeus.<n>`**, mirroring the upstream *module* tag it sits on, with a counter for
revisions of the patch against that same upstream release. Current: `v0.305.0-proxeus.3`, sitting on upstream
`v0.305.0` (= product `v3.5.0`, commit `8be3a95`).

## Rebasing onto a new Prometheus release

```sh
git remote add upstream https://github.com/prometheus/prometheus.git   # once
git fetch upstream --tags

# use the MODULE tag series (v0.3NN.M), not the product tag (v3.N.M)
git checkout -b rebase-v0.306.0 main
git rebase --onto v0.306.0 v0.305.0 main
# resolve conflicts — they cluster in promql/engine.go and promql/parser/ast.go,
# since the patch changes function signatures that upstream also edits

go build ./... && go test ./promql/...
git tag v0.306.0-proxeus.1
git push origin main --tags
```

Then in proxeus: bump the `replace` directive, `go mod tidy`, and run its full test suite — the pushdown tests in
`pkg/proxystorage` are what actually exercise this patch.

Dependabot and Renovate cannot see Prometheus releases through a `replace` directive. Upgrades are manual; watch
upstream releases directly.
