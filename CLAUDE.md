## Project knowledge

This repository contains a **Grafana plugin**. You must Read @./.config/AGENTS/instructions.md before doing changes.

### Plugin Details

- **Type**: Backend datasource plugin (Go + React)
- **Plugin ID**: `industream-databridge-datasource`
- **Purpose**: Query time series data from Industream DataBridge with DataCatalog asset navigation
- **PRD**: See `/home/cdm/Projets/industream-grafana-plugins/docs/PRD-QUERY-EDITOR-V2.md`

### Architecture

- **Frontend** (`src/`): React + `@grafana/ui` (Combobox, not deprecated Select)
- **Backend** (`pkg/`): Go with `grafana-plugin-sdk-go`
  - `pkg/plugin/` — Datasource handlers (QueryData, CallResource, CheckHealth)
  - `pkg/databridge/` — DataBridge REST API client
  - `pkg/datacatalog/` — DataCatalog REST API client
  - `pkg/cache/` — Generic TTL-based in-memory cache
  - `pkg/displayname/` — Display name pattern resolver
  - `pkg/models/` — Shared types (settings, query definition)

## RISK CLASSIFICATION
- **LOW** (proceed): Read files, go test, grep, lint.
- **MEDIUM** (log and proceed): Modify Go/React source, install dependencies.
- **HIGH** (confirm first): mage build (plugin binary), modify plugin.json, push to Grafana marketplace.

## NEVER
- Never modify anything inside `.config/` — managed by Grafana plugin tools.
- Never change `pluginId` or `type` in `plugin.json`.
- Never use deprecated `Select` — use `Combobox` from `@grafana/ui` v12.4+.
- Never store credentials in `jsonData` — use `secureJsonData`.
- Never skip `mage` for Go builds or `webpack` for frontend builds.

### Key Rules

- Do NOT modify `.config/` folder (managed by Grafana plugin tools)
- Use `secureJsonData` for credentials (API key)
- Use `mage` for Go builds, `webpack` for frontend builds
- `Combobox` replaces deprecated `Select` in `@grafana/ui` v12.4+
