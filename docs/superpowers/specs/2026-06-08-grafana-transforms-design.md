# Grafana plugin — query-time transforms (DataBridge v2.3.0)

Date: 2026-06-08
Status: Approved

## Goal

Expose the DataBridge v2.3.0 query-time transforms in the Grafana DataBridge
datasource plugin, so dashboards can request resampling, gap-filling, smoothing,
cumulative sums and rolling statistics directly through the existing
`POST /records/query` endpoint via its `transforms` array.

## Scope (v1)

Five transforms, in pipeline order:

1. `resample` — `every`, `aggregation?`, `createEmpty?`, `offset?`
2. `fill` — `method?`, `value?`, `limit?`
3. `movingAverage` — `window`
4. `cumulativeSum` — (no params)
5. `rollingStats` — `window`, `stats?[]`, `outputSuffix?`

Out of scope: the `/records/stats` endpoint (separate effort, option C), transform
reordering UI, per-tag transforms.

## Resample vs "Optimize display" (time_window)

Both bucket by time. Running both double-aggregates. Rule:

> If a `resample` transform is configured, `buildRecordsQuery` skips the automatic
> `time_window` SELECT + GROUP BY injection. Per-tag aggregations in `select` still
> apply. Without a resample, current behavior is unchanged (no regression).

## Architecture

### Backend (Go)

- `pkg/databridge/types.go` — add `Transform` (wrapper-object, nillable pointers)
  and its param structs; add `Transforms []Transform` to `RecordsQuery`.
- `pkg/models/query.go` — add `Transforms []databridge.Transform` to
  `QueryDefinition` (frontend sends the exact wrapper-object shape; no mapping).
- `pkg/plugin/query.go` `buildRecordsQuery` — copy `qd.Transforms` to
  `rq.Transforms`; if any element has `Resample != nil`, skip the time_window block.

`models` importing `databridge` introduces no cycle (verified: `databridge` does
not import `models`).

### Frontend (React/TS)

- `src/types.ts` — `Transform` union types + `transforms?: Transform[]` on
  `DataBridgeQuery`.
- `src/components/TransformsEditor.tsx` — new collapsible section, fixed-order
  enable toggles + param fields per transform, applied per query.
- `src/components/QueryEditor.tsx` — render `<TransformsEditor>` next to
  `<QueryOptions>`.

### UI

Fixed pipeline order (resample → fill → movingAverage → cumulativeSum →
rollingStats). Each transform: Enable toggle + its parameter inputs. Enabled
transforms are emitted, in order, into `query.transforms`. `rollingStats.stats`
defaults to empty (backend defaults: mean, std, min, max).

## Testing

- Go `buildRecordsQuery`:
  - transforms forwarded onto `rq.Transforms`
  - a resample transform suppresses the auto time_window (SELECT + GROUP BY)
  - without resample, time_window still injected (regression guard)
- TS: enabled toggles serialize to the wrapper-object contract shape.

## Risks

- Double-aggregation if suppression rule regresses → covered by Go tests.
- `fill.value` must serialize only when set (pointer / omitempty) to avoid sending 0.
