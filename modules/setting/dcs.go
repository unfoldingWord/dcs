// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// DCS settings
var DCS struct {
	Door43PreviewURL string
}

func loadDCSFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "dcs", &DCS)
	sec := rootCfg.Section("dcs")
	DCS.Door43PreviewURL = sec.Key("DOOR43_PREVIEW_URL").MustString("https://door43.org")
}
