package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// CheckHealth verifies connectivity to the DataCatalog and all DataBridge connections.
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var errors []string
	var details []string

	// Check DataCatalog connectivity (required)
	if d.catalogClient == nil {
		errors = append(errors, "DataCatalog API URL is not configured")
	} else if err := d.catalogClient.Ping(ctx); err != nil {
		errors = append(errors, fmt.Sprintf("DataCatalog: %v", err))
	} else {
		details = append(details, "DataCatalog connected")
	}

	// Check DataBridge connectivity via source connections
	if d.catalogClient != nil {
		conns, err := d.getConnections(ctx)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to list connections: %v", err))
		} else if len(conns) == 0 {
			errors = append(errors, "No source connections found in DataCatalog")
		} else {
			for _, c := range conns {
				if c.URL == "" {
					continue
				}
				client := d.dataBridgeClient(c.URL)
				if err := client.Ping(ctx); err != nil {
					errors = append(errors, fmt.Sprintf("DataBridge %s (%s): %v", c.Name, c.URL, err))
				} else {
					details = append(details, fmt.Sprintf("DataBridge %s OK", c.Name))
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

	if len(errors) > 0 {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: strings.Join(errors, "; "),
		}, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: strings.Join(details, ", "),
	}, nil
}
