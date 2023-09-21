// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
)

var _langnamesJSON []map[string]interface{}
var _langnamesJSONKeyed map[string]map[string]interface{}

// GetLangnamesJSON returns an array of maps from https://td.door43.org/exports/langnames.json
// Will use custom/options/languages/langnames.json instead if exists
func GetLangnamesJSON() []map[string]interface{} {
	if _langnamesJSON == nil {
		if langnames, err := GetLangnamesJSONFromCustom(); err == nil && langnames != nil {
			_langnamesJSON = langnames
		} else {
			langnames, err := GetLangnamesJSONFromTD()
			if err != nil {
				log.Error(err.Error())
			} else {
				_langnamesJSON = langnames
			}
		}
	}
	return _langnamesJSON
}

func GetLangnamesJSONFromCustom() ([]map[string]interface{}, error) {
	fileBuf, err := options.AssetFS().ReadFile("languages", "langnames.json")
	if err != nil {
		log.Debug("HERE: %s: %v", fileBuf, err)
		return nil, err
	}
	reader := bytes.NewReader(fileBuf)
	langnames := []map[string]interface{}{}
	if err := json.NewDecoder(reader).Decode(&langnames); err != nil {
		return nil, fmt.Errorf("unable to decode langnames.json from custom/options/languages/langnames.json: %v", err)
	}
	return langnames, nil
}

func GetLangnamesJSONFromTD() ([]map[string]interface{}, error) {
	langnames := []map[string]interface{}{}
	url := "https://td.unfoldingword.org/exports/langnames.json"
	myClient := &http.Client{Timeout: 10 * time.Second}
	response, err := myClient.Get(url)
	if err == nil {
		defer response.Body.Close()
		if err := json.NewDecoder(response.Body).Decode(&langnames); err != nil {
			return nil, fmt.Errorf("unable to decode langnames.json from tD: %v", err)
		}
	}
	return langnames, nil
}

func GetLangnamesJSONKeyed() map[string]map[string]interface{} {
	if _langnamesJSONKeyed == nil {
		_langnamesJSONKeyed = map[string]map[string]interface{}{}
		langnames := GetLangnamesJSON()
		for _, value := range langnames {
			_langnamesJSONKeyed[value["lc"].(string)] = value
		}
	}
	return _langnamesJSONKeyed
}

// GetLanguageFromRepoName determines the language of a repo by its repo name
func GetLanguageFromRepoName(repoName string) string {
	parts := strings.Split(strings.ToLower(repoName), "_")
	if len(parts) >= 2 && IsValidLanguage(parts[0]) && IsValidResource(parts[1]) {
		return parts[0]
	}
	parts = strings.Split(strings.ToLower(repoName), "-")
	if len(parts) == 3 && IsValidLanguage(parts[0]) && (parts[1] == "texttranslation" || parts[2] == "textstories") {
		return parts[0]
	}
	return ""
}

// IsValidLanguage returns true if string is a valid language code
func IsValidLanguage(lang string) bool {
	langnames := GetLangnamesJSONKeyed()
	_, ok := langnames[lang]
	return ok
}

// GetLanguageDirection returns the language direction
func GetLanguageDirection(lang string) string {
	langnames := GetLangnamesJSONKeyed()
	if data, ok := langnames[lang]; ok {
		if val, ok := data["ld"].(string); ok {
			return val
		}
	}
	return "ltr"
}

// GetLanguageTitle returns the language title
func GetLanguageTitle(lang string) string {
	langnames := GetLangnamesJSONKeyed()
	if data, ok := langnames[lang]; ok {
		if val, ok := data["ln"].(string); ok {
			return val
		}
	}
	return ""
}

// LanguageIsGL returns true if string is a valid language and is a GL
func LanguageIsGL(lang string) bool {
	langnames := GetLangnamesJSONKeyed()
	if data, ok := langnames[lang]; ok {
		if val, ok := data["gw"].(bool); ok {
			return val
		}
	}
	return false
}
