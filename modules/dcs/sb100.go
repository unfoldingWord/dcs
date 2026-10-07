// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// sb100Schema is the Scripture Burrito 1.0.0 schema bundled in options/schema/sb100.
// The files keep their upstream https://burrito.bible/schema/ $ids, which are mapped
// onto that directory (or a server's custom/options copy of it) rather than fetched.
var sb100Schema = &localSchema{
	dir:      "sb100",
	idPrefix: "https://burrito.bible/schema/",
	rootFile: "metadata.schema.json",
}

// GetSBDataFromBlob reads a blob of text and unmarshals it into an SBMetadata100 object
func GetSBDataFromBlob(blob *git.Blob) (*SBMetadata100, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}
	return ParseSBMetadata(buf)
}

// ParseSBMetadata unmarshals the content of a metadata.json into an SBMetadata100 object,
// keeping the generic map of the whole document in its Metadata field.
func ParseSBMetadata(buf []byte) (*SBMetadata100, error) {
	sb100 := &SBMetadata100{}
	if err := json.Unmarshal(buf, sb100); err != nil {
		log.Debug("SBMetadata100{} Unmarshal: %v", err)
		return nil, err
	}

	// Now make a generic map of the buffer to store in the database table
	sb100.Metadata = map[string]any{}
	if err := json.Unmarshal(buf, &sb100.Metadata); err != nil {
		log.Debug("sb100 map[string]interface{}{} Unmarshal: %v", err)
		return nil, err
	}

	return sb100, nil
}

// GetSB100Schema returns the schema for SB v1.0.0, compiled from options/schema/sb100
func GetSB100Schema(reload bool) (*jsonschema.Schema, error) {
	return sb100Schema.Get(reload)
}

// ValidateMapBySB100Schema validates a map structure by the SB v1.0.0 schema and returns the result
func ValidateMapBySB100Schema(data map[string]any) (*jsonschema.ValidationError, error) {
	return sb100Schema.Validate(data)
}

type SBMetadata100 struct {
	Format         string                         `json:"format"`
	Meta           *SB100Meta                     `json:"meta"`
	Identification *SB100Identification           `json:"identification"`
	Languages      []*SB100Language               `json:"languages"`
	Type           *SB100Type                     `json:"type"`
	LocalizedNames map[string]*SB100LocalizedName `json:"localizedNames"`
	Ingredients    map[string]*SB100Ingredient    `json:"ingredients"`
	Metadata       map[string]any
}

type LocalizedText map[string]string

// DetermineLocalizedTextToUse returns the value if there is an English "en" value, otherwise the first value
func (nm LocalizedText) DetermineLocalizedTextToUse() string {
	if value, ok := nm["en"]; ok {
		return value
	}
	for k := range nm {
		return nm[k]
	}
	return ""
}

type ScopeMap map[string][]any

// GetBookID returns the first key of the scope to be used at the book name
func (sm ScopeMap) GetBookID() string {
	for k := range sm {
		return k
	}
	return ""
}

type SB100Meta struct {
	Version       string `json:"version"`
	DefaultLocal  string `json:"defaultLocale"`
	DateCreate    string `json:"dateCreated"`
	Normalization string `json:"normalization:"`
}

type SB100Identification struct {
	Name         LocalizedText `json:"name"`
	Abbreviation LocalizedText `json:"abbreviation"`
}

type SB100Language struct {
	Tag             string        `json:"tag"`
	Name            LocalizedText `json:"name"`
	ScriptDirection string        `json:"scriptDirection"`
}

type SB100Type struct {
	FlavorType SB100FlavorType `json:"flavorType"`
}

type SB100FlavorType struct {
	Name         string      `json:"name"`
	Flavor       SB100Flavor `json:"flavor"`
	CurrentScope *ScopeMap   `json:"currentScope"`
}

type SB100Flavor struct {
	Name string `json:"name"`
}

type SB100LocalizedName struct {
	Short LocalizedText `json:"short"`
	Abbr  LocalizedText `json:"abbr"`
	Long  LocalizedText `json:"long"`
}

type SB100Ingredient struct {
	Checksum map[string]string `json:"checksum"`
	Mimetype string            `json:"mimeType"`
	Size     int64             `json:"size"`
	Scope    *ScopeMap         `json:"scope"`
	Role     string            `json:"role"`
}
