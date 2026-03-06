package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/industream/industream-data-bridge/pkg/databridge"
)

const defaultStreamInterval = 5 * time.Second

// streamRequest holds the parsed stream path parameters.
type streamRequest struct {
	ConnectionId string `json:"connectionId"`
	DatabaseName string `json:"databaseName"`
	DatasetName  string `json:"datasetName"`
	Interval     int    `json:"interval"` // seconds
}

// SubscribeStream validates the stream subscription and returns initial data.
func (d *Datasource) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	var sr streamRequest
	if err := json.Unmarshal(req.Data, &sr); err != nil {
		return &backend.SubscribeStreamResponse{
			Status: backend.SubscribeStreamStatusNotFound,
		}, nil
	}

	if sr.DatabaseName == "" || sr.DatasetName == "" {
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

	bridgeUrl, err := d.resolveConnectionUrl(ctx, sr.ConnectionId)
	if err != nil {
		return fmt.Errorf("resolve connection: %w", err)
	}

	client := d.dataBridgeClient(bridgeUrl)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frame, err := d.fetchLatestFrame(ctx, client, sr.DatabaseName, sr.DatasetName, interval)
			if err != nil {
				d.logger.Warn("Stream fetch error", "error", err)
				continue
			}
			if frame != nil {
				if err := sender.SendFrame(frame, data.IncludeAll); err != nil {
					return err
				}
			}
		}
	}
}

// fetchLatestFrame queries the last interval of data for streaming.
func (d *Datasource) fetchLatestFrame(ctx context.Context, client *databridge.Client, databaseName, datasetName string, interval time.Duration) (*data.Frame, error) {
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
