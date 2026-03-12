package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// CheckHealth verifies connectivity to the DataCatalog and all DataBridge connections.
// DataCatalog is required; individual DataBridge failures are reported as warnings.
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var criticalErrors []string
	var lines []string

	// Check DataCatalog connectivity (required)
	if d.catalogClient == nil {
		criticalErrors = append(criticalErrors, "DataCatalog API URL is not configured")
	} else if err := d.catalogClient.Ping(ctx); err != nil {
		criticalErrors = append(criticalErrors, fmt.Sprintf("DataCatalog: %v", err))
	} else {
		lines = append(lines, "✅ DataCatalog connected")
	}

	// Check DataBridge connectivity via source connections
	hasWarnings := false
	if d.catalogClient != nil {
		conns, err := d.getConnections(ctx)
		if err != nil {
			lines = append(lines, fmt.Sprintf("❌ Failed to list connections: %v", err))
			hasWarnings = true
		} else if len(conns) == 0 {
			lines = append(lines, "⚠️ No source connections found in DataCatalog")
			hasWarnings = true
		} else {
			for _, c := range conns {
				if c.URL == "" {
					continue
				}
				client := d.dataBridgeClient(c.URL)
				if err := client.Ping(ctx); err != nil {
					lines = append(lines, fmt.Sprintf("❌ %s — %v", c.Name, err))
					hasWarnings = true
				} else {
					lines = append(lines, fmt.Sprintf("✅ %s", c.Name))
				}
			}
		}
	}

	// Clear caches on health check
	d.connectionCache.Clear()
	d.entryCache.Clear()
	d.labelCache.Clear()
	d.assetCache.Clear()
	d.assetPathCache.Clear()

	// DataCatalog down = error
	if len(criticalErrors) > 0 {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: strings.Join(criticalErrors, "\n"),
		}, nil
	}

	message := strings.Join(lines, "\n")

	if hasWarnings {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusOk,
			Message: message,
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: message,
	}, nil
}
