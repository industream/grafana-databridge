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

// ListConnections returns all source connections from the DataCatalog.
func (c *Client) ListConnections(ctx context.Context) ([]SourceConnection, error) {
	var result []SourceConnection
	if err := c.get(ctx, "/source-connections", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
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
func (c *Client) GetEntriesByIds(ctx context.Context, ids []string) ([]CatalogEntry, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	params := url.Values{"ids": {strings.Join(ids, ",")}}

	var resp PaginatedResponse[CatalogEntry]
	if err := c.get(ctx, "/catalog-entries", params, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListAssetDictionaries returns all asset dictionaries.
func (c *Client) ListAssetDictionaries(ctx context.Context) ([]AssetDictionary, error) {
	var result []AssetDictionary
	if err := c.get(ctx, "/asset-dictionaries", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListAssetNodes returns nodes for an asset dictionary, with entry counts.
func (c *Client) ListAssetNodes(ctx context.Context, dictionaryId string) ([]AssetNode, error) {
	var result []AssetNode
	path := fmt.Sprintf("/asset-dictionaries/%s/nodes", dictionaryId)
	if err := c.get(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListNodeEntries returns catalog entries assigned to a node.
func (c *Client) ListNodeEntries(ctx context.Context, nodeId string) ([]CatalogEntry, error) {
	path := fmt.Sprintf("/asset-nodes/%s/entries", nodeId)
	var result []CatalogEntry
	if err := c.get(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListLabels returns all labels from the DataCatalog.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var result []Label
	if err := c.get(ctx, "/labels", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ping checks connectivity to the DataCatalog API.
func (c *Client) Ping(ctx context.Context) error {
	return c.get(ctx, "/labels", nil, &[]Label{})
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
	defer resp.Body.Close()

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
