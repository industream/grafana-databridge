package displayname

import (
	"strings"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
)

// ResolveContext holds all information needed to resolve a display name.
type ResolveContext struct {
	Entry       *datacatalog.CatalogEntry
	Column      string
	Aggregation string
	AssetPath   string
	Connection  string
}

// Resolve returns the display name based on the preset and optional custom pattern.
func Resolve(preset, pattern string, ctx *ResolveContext) string {
	if ctx == nil || ctx.Entry == nil {
		if ctx != nil && ctx.Column != "" {
			return ctx.Column
		}
		return "unknown"
	}

	switch preset {
	case "tagLevel1":
		if ctx.Entry.Metadata != nil && ctx.Entry.Metadata.TagLevel1 != "" {
			return ctx.Entry.Metadata.TagLevel1
		}
		return ctx.Entry.Name
	case "descriptionEn":
		return descriptionOrName(ctx.Entry, "en-US")
	case "descriptionDe":
		return descriptionOrName(ctx.Entry, "de-DE")
	case "assetPath":
		if ctx.AssetPath != "" {
			return ctx.AssetPath
		}
		return ctx.Entry.Name
	case "custom":
		if pattern != "" {
			return resolvePattern(pattern, ctx)
		}
		return ctx.Entry.Name
	default: // "entryName" or empty
		return ctx.Entry.Name
	}
}

func descriptionOrName(entry *datacatalog.CatalogEntry, locale string) string {
	if entry.Metadata != nil && entry.Metadata.Description != nil {
		if desc, ok := entry.Metadata.Description[locale]; ok && desc != "" {
			return desc
		}
	}
	return entry.Name
}

func resolvePattern(pattern string, ctx *ResolveContext) string {
	result := pattern
	result = strings.ReplaceAll(result, "{name}", ctx.Entry.Name)
	result = strings.ReplaceAll(result, "{column}", ctx.Column)
	result = strings.ReplaceAll(result, "{aggregation}", ctx.Aggregation)
	result = strings.ReplaceAll(result, "{connection}", ctx.Connection)
	result = strings.ReplaceAll(result, "{asset.path}", ctx.AssetPath)

	if ctx.Entry.Metadata != nil {
		result = strings.ReplaceAll(result, "{tagLevel1}", ctx.Entry.Metadata.TagLevel1)
		result = strings.ReplaceAll(result, "{unit}", ctx.Entry.Metadata.Unit)

		if ctx.Entry.Metadata.Description != nil {
			for locale, desc := range ctx.Entry.Metadata.Description {
				placeholder := "{description." + locale + "}"
				result = strings.ReplaceAll(result, placeholder, desc)
			}
		}
	}

	if len(ctx.Entry.Labels) > 0 {
		result = strings.ReplaceAll(result, "{label}", ctx.Entry.Labels[0].Name)
	}

	return result
}
