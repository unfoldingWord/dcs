// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "strings"

// DCS settings
var DCS struct {
	Door43PreviewURL string
	Convert2SBTopics []string
	// CacheSBArchives keeps converted SB archives per commit, whatever [repository] STREAM_ARCHIVES says:
	// a conversion costs far more than a "git archive", and /sb/raw/ reads files out of the kept archive.
	CacheSBArchives bool
}

func loadDCSFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "dcs", &DCS)
	sec := rootCfg.Section("dcs")
	DCS.Door43PreviewURL = sec.Key("DOOR43_PREVIEW_URL").MustString("https://preview.door43.org")
	DCS.CacheSBArchives = sec.Key("CACHE_SB_ARCHIVES").MustBool(true)
	// CONVERT_RC2SB_TOPICS was renamed to CONVERT2SB_TOPICS when tS-to-SB conversion was added
	deprecatedSetting(rootCfg, "dcs", "CONVERT_RC2SB_TOPICS", "dcs", "CONVERT2SB_TOPICS", "v1.28.0")
	topicsStr := sec.Key("CONVERT2SB_TOPICS").MustString(sec.Key("CONVERT_RC2SB_TOPICS").MustString(""))
	DCS.Convert2SBTopics = nil
	if topicsStr != "" {
		for t := range strings.SplitSeq(topicsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				DCS.Convert2SBTopics = append(DCS.Convert2SBTopics, t)
			}
		}
	}
}
