package service

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// The catalogue is a reviewed, read-only set of starting points. Listing it never
// creates persisted capabilities or enables a provider account.
//
//go:embed media_capability_catalog.json
var mediaCapabilityCatalogJSON []byte

// MediaCapabilityCatalogEntry retains pending models without advertising an
// executable schema when their exact public documentation is incomplete.
type MediaCapabilityCatalogEntry struct {
	Platform      string                     `json:"platform"`
	UpstreamModel string                     `json:"upstream_model"`
	MediaType     string                     `json:"media_type"`
	Enabled       bool                       `json:"enabled"`
	Source        string                     `json:"source"`
	Evidence      string                     `json:"evidence,omitempty"`
	Notes         string                     `json:"notes,omitempty"`
	Schema        json.RawMessage            `json:"schema,omitempty"`
	Example       json.RawMessage            `json:"example,omitempty"`
	Profile       UnifiedMediaAdapterProfile `json:"profile"`
}

var mediaCatalogOnce sync.Once
var mediaCatalogEntries []MediaCapabilityCatalogEntry
var mediaCatalogProfiles map[string]UnifiedMediaAdapterProfile

func loadMediaCapabilityCatalog() {
	mediaCatalogOnce.Do(func() {
		if err := json.Unmarshal(mediaCapabilityCatalogJSON, &mediaCatalogEntries); err != nil {
			panic(fmt.Sprintf("invalid embedded media catalogue: %v", err))
		}
		mediaCatalogProfiles = make(map[string]UnifiedMediaAdapterProfile, len(mediaCatalogEntries))
		for _, entry := range mediaCatalogEntries {
			mediaCatalogProfiles[entry.Platform+":"+entry.MediaType+":"+entry.UpstreamModel] = entry.Profile
		}
	})
}

// MediaCapabilityCatalog returns independent copies for administrative template
// browsing. Provider availability and local pricing still require configuration.
func MediaCapabilityCatalog() []MediaCapabilityCatalogEntry {
	loadMediaCapabilityCatalog()
	entries := make([]MediaCapabilityCatalogEntry, len(mediaCatalogEntries))
	for i, entry := range mediaCatalogEntries {
		entries[i] = entry
		entries[i].Schema = append(json.RawMessage(nil), entry.Schema...)
		entries[i].Example = append(json.RawMessage(nil), entry.Example...)
		entries[i].Profile = copyMediaAdapterProfile(entry.Profile)
	}
	return entries
}

func copyMediaAdapterProfile(profile UnifiedMediaAdapterProfile) UnifiedMediaAdapterProfile {
	profile.TopLevelMetadata = append([]string(nil), profile.TopLevelMetadata...)
	if profile.FieldMap != nil {
		fields := make(map[string]string, len(profile.FieldMap))
		for key, value := range profile.FieldMap {
			fields[key] = value
		}
		profile.FieldMap = fields
	}
	if profile.FixedFields != nil {
		raw, _ := json.Marshal(profile.FixedFields)
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			panic(err)
		}
		profile.FixedFields = fields
	}
	return profile
}
