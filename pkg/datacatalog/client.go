package datacatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client communicates with the DataCatalog REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a DataCatalog client with the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ListConnections returns DataBridge source connections from the DataCatalog.
func (c *Client) ListConnections(ctx context.Context) ([]SourceConnection, error) {
	params := url.Values{}
	params.Set("sourceTypeId", "DataBridge")

	var resp PaginatedResponse[SourceConnection]
	if err := c.get(ctx, "/source-connections", params, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListEntries returns catalog entries with optional filtering.
func (c *Client) ListEntries(ctx context.Context, label, search string) ([]CatalogEntry, error) {
	params := url.Values{}
	if label != "" {
		params.Set("label", label)
	}
	if search != "" {
		params.Set("search", search)
	}
	params.Set("limit", "1000")

	var resp PaginatedResponse[CatalogEntry]
	if err := c.get(ctx, "/catalog-entries", params, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetEntriesByIds fetches catalog entries by their IDs.
// The DataCatalog API does not support filtering by IDs, so we fetch all and filter client-side.
func (c *Client) GetEntriesByIds(ctx context.Context, ids []string) ([]CatalogEntry, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	all, err := c.ListEntries(ctx, "", "")
	if err != nil {
		return nil, err
	}

	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	result := make([]CatalogEntry, 0, len(ids))
	for _, entry := range all {
		if idSet[entry.ID] {
			result = append(result, entry)
		}
	}
	return result, nil
}

// ListAssetDictionaries returns all asset dictionaries.
func (c *Client) ListAssetDictionaries(ctx context.Context) ([]AssetDictionary, error) {
	var resp PaginatedResponse[AssetDictionary]
	if err := c.get(ctx, "/asset-dictionaries", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListAssetNodes returns nodes for an asset dictionary, with entry counts.
func (c *Client) ListAssetNodes(ctx context.Context, dictionaryId string) ([]AssetNode, error) {
	path := fmt.Sprintf("/asset-dictionaries/%s/nodes", dictionaryId)
	var resp PaginatedResponse[AssetNode]
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListNodeEntries returns catalog entries assigned to a node.
func (c *Client) ListNodeEntries(ctx context.Context, nodeId string) ([]CatalogEntry, error) {
	path := fmt.Sprintf("/asset-nodes/%s/entries", nodeId)
	var resp PaginatedResponse[CatalogEntry]
	if err := c.get(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListLabels returns all labels from the DataCatalog.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var resp PaginatedResponse[Label]
	if err := c.get(ctx, "/labels", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// Ping checks connectivity to the DataCatalog API.
func (c *Client) Ping(ctx context.Context) error {
	var resp PaginatedResponse[Label]
	return c.get(ctx, "/labels", nil, &resp)
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("DataCatalog API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
