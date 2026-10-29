// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"bytes"

	"gitea.dev/modules/json"
)

// Type represents the structure "type" as an object.
type Type struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON inspects the raw bytes to determine if it's a string or object.
func (t *Type) UnmarshalJSON(data []byte) error {
	// Trim leading/trailing whitespace
	data = bytes.TrimSpace(data)

	// If it starts with '{', it's a JSON object -> unmarshal all fields normally
	if bytes.HasPrefix(data, []byte("{")) {
		// Using an alias avoids infinite recursion loop during unmarshalling
		type Alias Type
		var aux Alias
		if err := json.Unmarshal(data, &aux); err != nil {
			return err
		}
		*t = Type(aux)
		return nil
	}

	// Otherwise, treat it as a string and assign it directly to the ID field
	return json.Unmarshal(data, &t.ID)
}

// tc v7: https://git.door43.org/qa99/en_ult_rom_book/raw/branch/master/manifest.json
// tc v8: https://git.door43.org/pjoakes/en_ust_2co_book/src/branch/master/manifest.json
// ts v3: https://git.door43.org/test2/uw-obs-aas/src/branch/master/manifest.json
// ts v5: https://git.door43.org/69c530493aab80e7/uw-mrk-lol/raw/branch/master/manifest.json
// ts v6: https://git.door43.org/Sitorabi/def_obs_text_obs/src/branch/master/manifest.json
type TcTsManifest struct {
	TcVersion       int `json:"tc_version"`      // for tC
	TsVersion       int `json:"package_version"` // for tS
	MetadataVersion string
	MetadataType    string
	Format          string `json:"format"`
	Subject         string
	FlavorType      string
	Flavor          string
	Title           string
	TargetLanguage  struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Direction string `json:"direction"`
	} `json:"target_language"`
	Type       Type   `json:"type"`
	ResourceID string `json:"resource_id"` // for tS package_version < 5
	Resource   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"resource"`
	ProjectID string `json:"project_id"`
	Project   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"project"`
	Slug string `json:"slug"`
}
