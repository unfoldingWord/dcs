// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

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
	Type struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"type"`
	ResourceID string `json:"resource_id"` // for tS package_version < 5
	Resource   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"resource"`
	Project struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
}
