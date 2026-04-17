// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "strings"

// DCS settings
var DCS struct {
	Door43PreviewURL   string
	ConvertRC2SBTopics []string
}

func loadDCSFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "dcs", &DCS)
	sec := rootCfg.Section("dcs")
	DCS.Door43PreviewURL = sec.Key("DOOR43_PREVIEW_URL").MustString("https://preview.door43.org")
	topicsStr := sec.Key("CONVERT_RC2SB_TOPICS").MustString("")
	DCS.ConvertRC2SBTopics = nil
	if topicsStr != "" {
		for _, t := range strings.Split(topicsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				DCS.ConvertRC2SBTopics = append(DCS.ConvertRC2SBTopics, t)
			}
		}
	}
}
