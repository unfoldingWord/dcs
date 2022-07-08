// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dcs

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var langNames = map[string]interface{}{}

// GetLangNames returns the langnames.json file from tD in a keyed map, loads from tD if not already loaded
func GetLangNames() map[string]interface{} {
	myClient := &http.Client{Timeout: 10 * time.Second}
	if len(langNames) == 0 {
		url := "https://td.unfoldingword.org/exports/langnames.json"
		response, err := myClient.Get(url)
		if err == nil {
			defer response.Body.Close()
			langNamesArr := &[]map[string]interface{}{}
			if err := json.NewDecoder(response.Body).Decode(langNamesArr); err != nil {
				log.Error("Unable to decode langnames.json from tD: %v", err)
			}
			for _, value := range *langNamesArr {
				langNames[value["lc"].(string)] = value
			}
		}
	}
	return langNames
}

// GetLanguageFromRepoName determines the language of a repo by its repo name
func GetLanguageFromRepoName(repoName string) string {
	parts := strings.Split(repoName, "_")
	if len(parts) == 2 && IsValidLanguage(parts[0]) && IsValidSubject(parts[1]) {
		return parts[0]
	}
	return ""
}

// IsValidLanguage returns true if string is a valid language code
func IsValidLanguage(lang string) bool {
	ln := GetLangNames()
	_, ok := ln[lang]
	return ok
}

// GetLanguageDirection returns the language direction
func GetLanguageDirection(lang string) string {
	ln := GetLangNames()
	if data, ok := ln[lang]; ok {
		if val, ok := data.(map[string]interface{})["ld"].(string); ok {
			return val
		}
	}
	return "ltr"
}

// GetLanguageTitle returns the language title
func GetLanguageTitle(lang string) string {
	ln := GetLangNames()
	if data, ok := ln[lang]; ok {
		if val, ok := data.(map[string]interface{})["ln"].(string); ok {
			return val
		}
	}
	return ""
}

// LanguageIsGL returns true if string is a valid language and is a GL
func LanguageIsGL(lang string) bool {
	ln := GetLangNames()
	if data, ok := ln[lang]; ok {
		if val, ok := data.(map[string]interface{})["gw"].(bool); ok {
			return val
		}
	}
	return false
}
