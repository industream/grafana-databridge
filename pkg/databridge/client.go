package databridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client communicates with the DataBridge REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a DataBridge client with the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListDatabases returns available databases from a connection.
func (c *Client) ListDatabases(ctx context.Context) ([]DatabaseInfo, error) {
	var result PaginatedResponse[DatabaseInfo]
	if err := c.get(ctx, "/databases", nil, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ListDatasets returns datasets for a given database.
func (c *Client) ListDatasets(ctx context.Context, databaseName string) ([]DatasetInfo, error) {
	params := url.Values{"databaseName": {databaseName}}
	var result PaginatedResponse[DatasetInfo]
	if err := c.get(ctx, "/datasets", params, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// GetSchema returns the column schema for a dataset by looking it up in the datasets list.
func (c *Client) GetSchema(ctx context.Context, databaseName, datasetName string) (*DatasetSchema, error) {
	datasets, err := c.ListDatasets(ctx, databaseName)
	if err != nil {
		return nil, err
	}
	for _, ds := range datasets {
		if ds.Name == datasetName {
			return &DatasetSchema{Columns: ds.Columns}, nil
		}
	}
	return nil, fmt.Errorf("dataset %q not found", datasetName)
}

// QueryRecords executes a records query and returns the raw JSON response body.
func (c *Client) QueryRecords(ctx context.Context, databaseName, datasetName string, query *RecordsQuery) (*RecordsResponse, error) {
	params := url.Values{
		"databaseName": {databaseName},
		"datasetName":  {datasetName},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	var result RecordsResponse
	if err := c.post(ctx, "/records/query", params, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ComputeStats computes descriptive statistics (mean, min, max, percentiles, ...) per
// signal over a time range via POST /records/stats. Entries are signal columns/_fields.
// datasetName scopes the stats to a single dataset (measurement) so a field name shared
// across measurements is not pooled; pass "" to keep the legacy whole-database behavior.
func (c *Client) ComputeStats(ctx context.Context, databaseName, datasetName string, query *StatsQuery) (StatsResponse, error) {
	params := url.Values{"databaseName": {databaseName}}
	if datasetName != "" {
		params.Set("datasetName", datasetName)
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal stats query: %w", err)
	}

	var result StatsResponse
	if err := c.post(ctx, "/records/stats", params, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ping checks connectivity to the DataBridge API.
func (c *Client) Ping(ctx context.Context) error {
	u, err := url.Parse(c.baseURL + "/databases")
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping DataBridge: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DataBridge returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	return c.doAndDecode(req, result)
}

func (c *Client) post(ctx context.Context, path string, params url.Values, body []byte, result interface{}) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.doAndDecode(req, result)
}

func (c *Client) doAndDecode(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// APIError is a structured DataBridge API error. NoData is true when the failure is a
// 422 "column does not exist" — i.e. DataBridge simply has no data for the requested
// column(s) (typically because the source never wrote any value), not a real fault.
// Detail carries the human-readable reason(s) extracted from the RFC problem+json body.
type APIError struct {
	StatusCode int
	Body       string
	NoData     bool
	Detail     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// newAPIError parses a non-2xx DataBridge response body (RFC 7807 problem+json) and
// flags the "column does not exist" case so callers can surface a clearer message.
func newAPIError(status int, body []byte) *APIError {
	e := &APIError{StatusCode: status, Body: string(body)}
	var p struct {
		Detail string `json:"detail"`
		Errors []struct {
			Detail string `json:"detail"`
			Code   string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &p) == nil {
		var details []string
		for _, er := range p.Errors {
			if strings.Contains(er.Code, "ColumnDoesNotExist") {
				e.NoData = true
			}
			if er.Detail != "" {
				details = append(details, er.Detail)
			}
		}
		if len(details) > 0 {
			e.Detail = strings.Join(details, "; ")
		} else {
			e.Detail = p.Detail
		}
	}
	return e
}
