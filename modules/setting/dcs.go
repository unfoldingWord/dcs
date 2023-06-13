// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// DCS settings
var DCS = struct {
	Door43PreviewURL string
}{
	Door43PreviewURL: "https://door43.org",
}

func loadDCSFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "dcs", &DCS)
}
