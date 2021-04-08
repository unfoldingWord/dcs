// Copyright 2021 unfoldingWord. All rights reserved.
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

func GetLanguageFromRepoName(repoName string) string {
	parts := strings.Split(repoName, "_")
	if len(parts) > 1 {
		ln := GetLangNames()
		if _, ok := ln[parts[0]]; ok {
			return parts[0]
		}
	}
	return ""
}
