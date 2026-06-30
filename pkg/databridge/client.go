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

// GetInfo returns the DataBridge instance info, including the active provider
// and its capabilities. Capabilities is nil on older images that predate the
// capability contract — callers must degrade open in that case.
func (c *Client) GetInfo(ctx context.Context) (*InfoResponse, error) {
	var result InfoResponse
	if err := c.get(ctx, "/info", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
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

// problemDetails is the subset of an RFC 7807 problem-details body that
// DataBridge returns for validation errors (422). The "code" field carries the
// typed error name, e.g. "QueryRecords.AggregationNotSupported".
type problemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
	Code   string `json:"code"`
}

// APIError is a non-2xx response from the DataBridge API. It preserves the HTTP
// status and, when the body is an RFC 7807 problem-details document, the typed
// error code and human-readable detail so callers can surface a clean message
// instead of a raw JSON blob.
type APIError struct {
	StatusCode int
	Code       string
	Detail     string
	Body       string
}

// newAPIError builds an APIError, parsing the body as problem-details when
// possible and falling back to the raw body otherwise.
func newAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{StatusCode: statusCode, Body: string(body)}
	var pd problemDetails
	if err := json.Unmarshal(body, &pd); err == nil {
		apiErr.Code = pd.Code
		// Prefer the most specific human-readable message available.
		if pd.Detail != "" {
			apiErr.Detail = pd.Detail
		} else if pd.Title != "" {
			apiErr.Detail = pd.Title
		}
	}
	return apiErr
}

// Error implements the error interface with a concise, user-readable message.
func (e *APIError) Error() string {
	if e.Detail != "" {
		if e.Code != "" {
			return fmt.Sprintf("%s: %s", e.Code, e.Detail)
		}
		return e.Detail
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// IsNotSupported reports whether this error is a DataBridge capability
// rejection (a 422 carrying a *NotSupported code), which means the active
// provider does not support the requested aggregation or stat.
func (e *APIError) IsNotSupported() bool {
	return e.StatusCode == http.StatusUnprocessableEntity && strings.HasSuffix(e.Code, "NotSupported")
}
