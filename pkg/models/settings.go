package models

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// PluginSettings holds the datasource configuration from Grafana.
type PluginSettings struct {
	DataBridgeApiUrl     string `json:"dataBridgeApiUrl"`
	DataCatalogApiUrl    string `json:"dataCatalogApiUrl"`
	SourceConnectionId   string `json:"sourceConnectionId"`
	DefaultDisplayName   string `json:"defaultDisplayNamePreset"`
	DefaultAggregation   string `json:"defaultAggregation"`
	MaxRawRows           int    `json:"maxRawRows"`
	HardLimitRows        int    `json:"hardLimitRows"`
	CacheTtlSeconds      int    `json:"cacheTtlSeconds"`
	Secrets              *SecretSettings `json:"-"`
}

// SecretSettings holds encrypted credentials from secureJsonData.
type SecretSettings struct {
	ApiKey string `json:"apiKey"`
}

// Defaults applies default values to unset fields.
func (s *PluginSettings) Defaults() {
	if s.MaxRawRows == 0 {
		s.MaxRawRows = 50_000
	}
	if s.HardLimitRows == 0 {
		s.HardLimitRows = 1_000_000
	}
	if s.CacheTtlSeconds == 0 {
		s.CacheTtlSeconds = 300
	}
	if s.DefaultAggregation == "" {
		s.DefaultAggregation = "avg"
	}
	if s.DefaultDisplayName == "" {
		s.DefaultDisplayName = "entryName"
	}
}

// LoadPluginSettings parses datasource instance settings into PluginSettings.
func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := &PluginSettings{}
	if err := json.Unmarshal(source.JSONData, settings); err != nil {
		return nil, fmt.Errorf("unmarshal plugin settings: %w", err)
	}

	settings.Secrets = &SecretSettings{
		ApiKey: source.DecryptedSecureJSONData["apiKey"],
	}
	settings.Defaults()

	return settings, nil
}
