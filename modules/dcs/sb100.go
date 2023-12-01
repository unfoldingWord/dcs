// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"

	"github.com/santhosh-tekuri/jsonschema/v5"

	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader" // Loader for Schema via HTTP
)

var sb100Schema *jsonschema.Schema

func GetSBDataFromBlob(blob *git.Blob) (*SBMetadata100, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}
	sbEncoded := &SBEncodedMetadata{}
	if err = json.Unmarshal(buf, sbEncoded); err == nil {
		buf = sbEncoded.Data
	}

	sb100 := &SBMetadata100{}
	if err := json.Unmarshal(buf, sb100); err != nil {
		return nil, err
	}

	// Now make a generic map of the buffer to store in the database table
	sb100.Metadata = map[string]interface{}{}
	if err := json.Unmarshal(buf, sb100.Metadata); err != nil {
		return nil, err
	}

	return sb100, nil
}

// GetSB100Schema returns the schema for SB v1.0.0
func GetSB100Schema(reload bool) (*jsonschema.Schema, error) {
	// We must use githubURLPrefix due to certificate issues
	burritoBiblePrefix := "https://burrito.bible/schema/"
	githubPrefix := "https://raw.githubusercontent.com/bible-technology/scripture-burrito/v1.0.0/schema/"
	if sb100Schema == nil || reload {
		jsonschema.Loaders["https"] = func(url string) (io.ReadCloser, error) {
			uriPath := strings.TrimPrefix(url, burritoBiblePrefix)
			githubURL := githubPrefix + uriPath
			res, err := http.Get(githubURL)
			if err == nil && res != nil && res.StatusCode == 200 {
				return res.Body, nil
			}
			log.Error("GetSB100Schema: not able to get the schema file remotely [%q]: %v", url, err)
			fileBuf, err := options.AssetFS().ReadFile("schema", "sb100", uriPath)
			if err != nil {
				log.Error("GetSB100Schema: local schema file not found: [options/schema/sb100/%s]: %v", uriPath, err)
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(fileBuf)), nil
		}
		var err error
		sb100Schema, err = jsonschema.Compile(burritoBiblePrefix + "metadata.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return sb100Schema, nil
}

// ValidateMapBySB100Schema Validates a map structure by the RC v0.2.0 schema and returns the result
func ValidateMapBySB100Schema(data map[string]interface{}) (*jsonschema.ValidationError, error) {
	if data == nil {
		return &jsonschema.ValidationError{Message: "file cannot be empty"}, nil
	}
	schema, err := GetSB100Schema(false)
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(data); err != nil {
		switch e := err.(type) {
		case *jsonschema.ValidationError:
			return e, nil
		default:
			return nil, e
		}
	}
	return nil, nil
}

type SBEncodedMetadata struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type SBMetadata100 struct {
	Format         string                        `json:"format"`
	Meta           SB100Meta                     `json:"meta"`
	Identification SB100Identification           `json:"identification"`
	Languages      []SB100Language               `json:"languages"`
	Type           SB100Type                     `json:"type"`
	LocalizedNames map[string]SB100LocalizedName `json:"localizedNames"`
	Metadata       map[string]interface{}
	Ingredients    map[string]SB100Ingredient `xorm:"JSON"`
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

type ScopeMap map[string][]interface{}

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
	Mimetype string            `json:"mimetype"`
	Size     int64             `json:"size"`
	Scope    *ScopeMap         `json:"scope"`
	Role     string            `json:"role"`
}
