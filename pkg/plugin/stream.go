package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
	"github.com/industream/industream-data-bridge/pkg/databridge"
	"github.com/industream/industream-data-bridge/pkg/models"
)

const defaultStreamInterval = 5 * time.Second

// streamRequest holds the parsed stream path parameters.
// Supports both legacy single-dataset mode and multi-dataset catalog mode.
type streamRequest struct {
	// Legacy single-dataset fields
	ConnectionId string `json:"connectionId"`
	DatabaseName string `json:"databaseName"`
	DatasetName  string `json:"datasetName"`

	// Multi-dataset catalog mode
	Select []models.SelectDefinition `json:"select,omitempty"`

	Interval int `json:"interval"` // seconds
}

// SubscribeStream validates the stream subscription and returns initial data.
func (d *Datasource) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	var sr streamRequest
	if err := json.Unmarshal(req.Data, &sr); err != nil {
		return &backend.SubscribeStreamResponse{
			Status: backend.SubscribeStreamStatusNotFound,
		}, nil
	}

	// Accept either legacy single-dataset or catalog multi-dataset
	hasLegacy := sr.DatabaseName != "" && sr.DatasetName != ""
	hasCatalog := len(sr.Select) > 0

	if !hasLegacy && !hasCatalog {
		return &backend.SubscribeStreamResponse{
			Status: backend.SubscribeStreamStatusNotFound,
		}, nil
	}

	return &backend.SubscribeStreamResponse{
		Status: backend.SubscribeStreamStatusOK,
	}, nil
}

// PublishStream disallows client-side publishing.
func (d *Datasource) PublishStream(_ context.Context, _ *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}

// RunStream continuously queries the DataBridge and pushes frames to subscribers.
func (d *Datasource) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	var sr streamRequest
	if err := json.Unmarshal(req.Data, &sr); err != nil {
		return fmt.Errorf("parse stream request: %w", err)
	}

	interval := defaultStreamInterval
	if sr.Interval > 0 {
		interval = time.Duration(sr.Interval) * time.Second
	}

	// Multi-dataset catalog mode
	if len(sr.Select) > 0 {
		return d.runMultiDatasetStream(ctx, sender, sr.Select, interval)
	}

	// Legacy single-dataset mode
	bridgeUrl, err := d.resolveConnectionUrl(ctx, sr.ConnectionId)
	if err != nil {
		return fmt.Errorf("resolve connection: %w", err)
	}

	client := d.dataBridgeClient(bridgeUrl)
	return runStreamLoop(ctx, sender, interval, func() ([]*data.Frame, error) {
		frame, err := fetchLatestFrame(ctx, client, sr.DatabaseName, sr.DatasetName, interval)
		if err != nil {
			return nil, err
		}
		if frame == nil {
			return nil, nil
		}
		return []*data.Frame{frame}, nil
	})
}

// resolveStreamTargets resolves catalog entries into DataBridge query targets,
// applying the same id-first / column-fallback resolution as the query path so
// cross-instance dashboards stream without edits.
func (d *Datasource) resolveStreamTargets(ctx context.Context, selectItems []models.SelectDefinition) ([]queryTarget, error) {
	if d.catalogClient == nil {
		return nil, fmt.Errorf("DataCatalog URL is not configured")
	}

	// Collect catalog entry IDs. A select is resolvable via either its id or its
	// stable column (cross-instance fallback).
	ids := make([]string, 0, len(selectItems))
	resolvable := 0
	for _, s := range selectItems {
		if s.CatalogEntryId != "" {
			ids = append(ids, s.CatalogEntryId)
		}
		if s.CatalogEntryId != "" || s.Column != "" {
			resolvable++
		}
	}
	if resolvable == 0 {
		return nil, fmt.Errorf("no catalog entries in stream request")
	}

	// Fetch entries and resolve targets, serving from the cached all-entries
	// listing when possible so streams don't refetch on every resolve.
	entries, err := d.getEntriesByIds(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("fetch entries: %w", err)
	}

	entryMap := make(map[string]*datacatalog.CatalogEntry, len(entries))
	for i := range entries {
		entryMap[entries[i].ID] = &entries[i]
	}

	// Build the column-fallback index only when an id fails to resolve.
	var byColumn map[string]*datacatalog.CatalogEntry
	if selectsNeedColumnFallback(selectItems, entryMap) {
		byColumn = d.dataBridgeEntriesByColumn(ctx)
	}

	targets, err := d.groupByTarget(ctx, selectItems, entryMap, byColumn)
	if err != nil {
		return nil, fmt.Errorf("resolve targets: %w", err)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no valid stream targets found")
	}
	return targets, nil
}

// runMultiDatasetStream resolves catalog entries into targets and streams all of them.
func (d *Datasource) runMultiDatasetStream(ctx context.Context, sender *backend.StreamSender, selectItems []models.SelectDefinition, interval time.Duration) error {
	targets, err := d.resolveStreamTargets(ctx, selectItems)
	if err != nil {
		return err
	}

	d.logger.Info("Multi-dataset stream started", "targets", len(targets))

	// Build a fetcher that queries all targets in parallel on each tick
	return runStreamLoop(ctx, sender, interval, func() ([]*data.Frame, error) {
		return d.fetchAllTargets(ctx, targets, interval)
	})
}

// fetchAllTargets queries all targets in parallel and returns their frames.
func (d *Datasource) fetchAllTargets(ctx context.Context, targets []queryTarget, interval time.Duration) ([]*data.Frame, error) {
	type result struct {
		frames []*data.Frame
		err    error
	}

	results := make([]result, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target queryTarget) {
			defer wg.Done()
			client := d.dataBridgeClient(target.bridgeUrl)

			// Build SELECT with columns from target's select items
			var selectClauses []databridge.SelectClause
			for _, s := range target.selectItems {
				if s.Column == "" {
					continue
				}
				selectClauses = append(selectClauses, databridge.SelectClause{Column: s.Column})
			}

			now := time.Now().UTC()
			from := now.Add(-interval)

			rq := &databridge.RecordsQuery{
				Select: selectClauses,
				Where: &databridge.WhereExpression{
					Operator: "and",
					Conditions: []databridge.WhereExpression{
						{
							Operator: "greaterOrEqual",
							Left:     &databridge.WhereOperand{Column: "time"},
							Right:    &databridge.WhereOperand{Constant: from.Format(time.RFC3339)},
						},
						{
							Operator: "less",
							Left:     &databridge.WhereOperand{Column: "time"},
							Right:    &databridge.WhereOperand{Constant: now.Format(time.RFC3339)},
						},
					},
				},
				OrderBy: []databridge.OrderClause{{Column: "time", Direction: "asc"}},
			}

			resp, err := client.QueryRecords(ctx, target.databaseName, target.datasetName, rq)
			if err != nil {
				results[idx] = result{err: err}
				return
			}

			frame, err := databridge.ToDataFrame("stream", resp)
			if err != nil {
				results[idx] = result{err: err}
				return
			}

			if frame != nil {
				results[idx] = result{frames: []*data.Frame{frame}}
			}
		}(i, t)
	}
	wg.Wait()

	var allFrames []*data.Frame
	for _, r := range results {
		if r.err != nil {
			d.logger.Warn("Stream target fetch error", "error", r.err)
			continue
		}
		allFrames = append(allFrames, r.frames...)
	}

	return allFrames, nil
}

// runStreamLoop runs a ticker loop, calling fetchFn on each tick and sending frames.
func runStreamLoop(ctx context.Context, sender *backend.StreamSender, interval time.Duration, fetchFn func() ([]*data.Frame, error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frames, err := fetchFn()
			if err != nil {
				continue
			}
			for _, frame := range frames {
				if err := sender.SendFrame(frame, data.IncludeAll); err != nil {
					return err
				}
			}
		}
	}
}

// fetchLatestFrame queries the last interval of data for streaming.
func fetchLatestFrame(ctx context.Context, client *databridge.Client, databaseName, datasetName string, interval time.Duration) (*data.Frame, error) {
	now := time.Now().UTC()
	from := now.Add(-interval)

	rq := &databridge.RecordsQuery{
		Where: &databridge.WhereExpression{
			Operator: "and",
			Conditions: []databridge.WhereExpression{
				{
					Operator: "greaterOrEqual",
					Left:     &databridge.WhereOperand{Column: "time"},
					Right:    &databridge.WhereOperand{Constant: from.Format(time.RFC3339)},
				},
				{
					Operator: "less",
					Left:     &databridge.WhereOperand{Column: "time"},
					Right:    &databridge.WhereOperand{Constant: now.Format(time.RFC3339)},
				},
			},
		},
		OrderBy: []databridge.OrderClause{{Column: "time", Direction: "asc"}},
	}

	resp, err := client.QueryRecords(ctx, databaseName, datasetName, rq)
	if err != nil {
		return nil, err
	}

	return databridge.ToDataFrame("stream", resp)
}
