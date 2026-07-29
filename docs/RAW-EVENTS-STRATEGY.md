# `raw` query strategy — status: PROOF OF CONCEPT, not reviewed

> **Read this before evaluating, extending, or merging this branch.**
> Branch: `feature/grafana-raw-events-mode`. Built 2026-07-28. Never merged, never
> reviewed. Backend and frontend both build clean and unit tests pass — but it has
> **never been seen running in a real Grafana instance**. See
> [What is NOT proven](#what-is-not-proven).

## ⚠️ Branch base — read first

This branch is **not** based on `master`. It sits on top of `fix/stats-send-datasetname`,
which is itself an unmerged branch already pushed to the remote:

```
origin/master
     └── 9500c93  fix(stats): send datasetName to /records/stats   ┐ from
     └── b05a156  fix(query): select a time column for raw ts      ┘ fix/stats-send-datasetname
          └── 41008e5  feat(query): add raw-events strategy        ← the only commit that is mine
```

**Merging this branch into `master` would also bring in those two commits.** If that's not
wanted, rebase the single `41008e5` commit onto `master` first — it is self-contained and
should rebase cleanly (it touches `query.go`'s strategy dispatch, `types.ts`,
`QueryEditor.tsx`, `RawColumnSelector.tsx`, plus new tests; the two base commits touch
`stats`/time-column selection).

## Why this exists

For the **VERITAS** RFCS proposal (BFI-led) and its Dillinger material-genealogy use case:
we need to display **discrete process events** (`event_id`, `event_type`, `occurred_at`,
`equipment`, `payload`) in Grafana — a table of individual event rows.

The plugin's existing strategies can't do that:

| Existing strategy | Why it doesn't fit |
|---|---|
| `timeseries` | Buckets + aggregates for line panels |
| `table` | **Reduces over the whole range** — returns totals, not per-event rows |
| `stats` | Scalar statistics per signal (mean/min/max/percentiles) |

So `raw` was added: a plain, unaggregated select returning one frame row per record.

**Good news that made this small:** `pkg/databridge/frame.go::ToDataFrame` already infers
per-column types generically (`float64`/`bool`/`datetime`/`string`) — the type plumbing for
string/JSON event columns needed **no change at all**. Only the query-building path did.

## What changed

1. **`src/types.ts`** — added `'raw'` to `QueryStrategy`. Note the deliberate naming
   collision documented in the code: this is distinct from the existing
   `QueryMode: 'raw'` (which selects connection/database/dataset instead of DataCatalog
   signals). The two compose freely — `QueryMode` is *where columns come from*,
   `QueryStrategy` is *how they're aggregated*.
2. **`pkg/plugin/query.go` (`buildRecordsQuery`)** — short-circuit branch when
   `Strategy == "raw"`: every select item becomes a plain `SelectClause{Column: col}` with
   no `Function`, skipping aggregation/type-compatibility logic entirely. The automatic
   `time_window` GROUP BY is now gated on `Strategy != "raw"` as well as `!= "table"`.
   The time column *is* still auto-prepended (unlike `table`) since raw events need a
   per-row timestamp. `Limit`/`Offset`/`OrderBy` pass through unchanged.
   `applySafetyLimits` always applies the row-count check for `raw`, even if a stale saved
   query still carries `OptimizeDisplay=true`.
3. **`src/components/QueryEditor.tsx`** — "Raw Events" added to the strategy radio group;
   `optimizeDisplay` forced `false` in this strategy; newly-picked columns/tags default to
   `aggregation: 'none'`; every select-mutation handler recomputes `hasAggregation`
   excluding the raw strategy, so the persisted `optimizeDisplay` stays consistent with
   what's actually sent to the backend.
4. **`src/components/RawColumnSelector.tsx`** — new optional `strategy` prop; when `'raw'`,
   the per-column aggregation `Combobox` is replaced by a static "Raw (no aggregation)"
   badge, since aggregation is meaningless there.
5. **`pkg/plugin/query_test.go`** — two new tests:
   `TestBuildRecordsQuery_RawStrategy_NoAggregation` (no GROUP BY, plain columns including
   a `string` column whose aggregation must be ignored, time column prepended, `Limit`
   respected) and `TestBuildRecordsQuery_RawStrategy_IgnoresStaleOptimizeDisplayFlag`
   (defense-in-depth: a stale `OptimizeDisplay=true` must not resurrect aggregation).

## What IS proven

All commands run, real output, all green:

- `go build ./...`, `go vet ./...` — clean
- `go test ./...` — all packages pass, including both new raw-strategy tests
- `npx tsc --noEmit` — clean
- `npm run build` (webpack production) — compiled successfully
- `npm run lint` — 0 errors (1 pre-existing warning in `src/module.ts`, unrelated:
  `setVariableQueryEditor` deprecation)
- `npm run test:ci` (jest) — 6/6 existing tests pass

## What is NOT proven

- **Never run in a live Grafana instance.** No panel has ever rendered a `raw` query.
- **The Playwright e2e suite was not run** (`tests/queryEditor.spec.ts`) — it needs a live
  Grafana + provisioned datasource via `npm run server` (docker compose). The UI changes
  (`QueryEditor.tsx`, `RawColumnSelector.tsx`) have **no jest coverage** — they're only
  covered by that unrun e2e spec. Treat the UI half as unverified.
- **Never queried a live DataBridge.** Correctness of the generated `RecordsQuery` JSON
  against the real API is verified only by unit-testing the query builder.
- No row-limit UI control was added — the existing `applySafetyLimits`/`MaxRawRows` setting
  covers it, and no other strategy has a manual limit field either.

## Annotations (event markers on timeseries panels) — NOT implemented

The natural companion feature — overlaying event markers on existing signal panels — is
**absent**. The plugin has **zero annotation support** today: no `AnnotationQuery` handler,
and no `"annotations": true` in `plugin.json` (both verified by grep, 0 hits).

There is a design-note comment block at the end of `pkg/plugin/query.go` sketching what it
would take. It is deliberately **documentation only, not code** — the exact annotation-frame
field contract Grafana's renderer expects couldn't be verified without a live Grafana, and
shipping a guess that silently renders nothing would be worse than shipping nothing.

## Related material

- `feature/iceberg-event-lake-poc` branch in the `industream-databridge` repo — the
  DataBridge Iceberg store provider these event rows would come from, with its own
  handover README.
- `iceberg-poc` repo — the Iceberg lakehouse + PyIceberg sidecar, and `RUNBOOK-E2E.md`
  with the full bring-up sequence and honest proven-vs-unverified breakdown.
- `EAF-EVENT-LAKE-ICEBERG.md` — design rationale for the event-lake as a whole.
