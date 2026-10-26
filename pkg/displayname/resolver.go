package displayname

import (
	"sort"
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
	// Locale is the viewer's Grafana language (e.g. "de-DE"), sent by the frontend.
	Locale string
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
	case "description":
		return localizedDescription(ctx.Entry, ctx.Locale)
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
	if entry.Metadata != nil {
		if descMap := entry.Metadata.Description(); descMap != nil {
			if desc, ok := descMap[locale]; ok && desc != "" {
				return desc
			}
		}
	}
	return entry.Name
}

// localizedDescription picks the catalog description in the viewer's language: exact locale, then the same
// language in another region, then English, then the entry name.
func localizedDescription(entry *datacatalog.CatalogEntry, locale string) string {
	if entry.Metadata == nil {
		return entry.Name
	}
	descMap := entry.Metadata.Description()
	if desc := descMap[locale]; desc != "" {
		return desc
	}
	if desc := firstForLanguage(descMap, languageOf(locale)); desc != "" {
		return desc
	}
	if desc := descMap["en-US"]; desc != "" {
		return desc
	}
	if desc := firstForLanguage(descMap, "en"); desc != "" {
		return desc
	}
	return entry.Name
}

func languageOf(locale string) string {
	return strings.ToLower(strings.SplitN(locale, "-", 2)[0])
}

// firstForLanguage returns the first non-empty description whose locale has this language, in locale order.
func firstForLanguage(descMap map[string]string, language string) string {
	if language == "" {
		return ""
	}
	locales := make([]string, 0, len(descMap))
	for locale := range descMap {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		if languageOf(locale) == language && descMap[locale] != "" {
			return descMap[locale]
		}
	}
	return ""
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
		result = strings.ReplaceAll(result, "{unit}", string(ctx.Entry.Metadata.Unit))
		if strings.Contains(result, "{description}") {
			result = strings.ReplaceAll(result, "{description}", localizedDescription(ctx.Entry, ctx.Locale))
		}

		if descMap := ctx.Entry.Metadata.Description(); descMap != nil {
			for locale, desc := range descMap {
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
