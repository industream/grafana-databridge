package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// CheckHealth verifies connectivity to both DataBridge and DataCatalog APIs.
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	var errors []string

	// Check DataBridge connectivity
	if d.settings.DataBridgeApiUrl != "" {
		client := d.dataBridgeClient("")
		if err := client.Ping(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("DataBridge: %v", err))
		}
	} else {
		errors = append(errors, "DataBridge API URL is not configured")
	}

	// Check DataCatalog connectivity
	if d.catalogClient != nil {
		if err := d.catalogClient.Ping(ctx); err != nil {
			errors = append(errors, fmt.Sprintf("DataCatalog: %v", err))
		}
	}
	// DataCatalog is optional — no error if not configured

	// Clear caches on health check
	d.connectionCache.Clear()
	d.entryCache.Clear()
	d.labelCache.Clear()
	d.assetCache.Clear()

	if len(errors) > 0 {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: strings.Join(errors, "; "),
		}, nil
	}

	message := "DataBridge connected"
	if d.catalogClient != nil {
		message += ", DataCatalog connected"
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: message,
	}, nil
}
