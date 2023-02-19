// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package dcs

// GetMetadataTypeTitle returns the metadata type title
func GetMetadataTypeTitle(metadataType string) string {
	switch metadataType {
	case "ts":
		return "translationStudio"
	case "tc":
		return "translationCore"
	case "rc":
		return "Resource Container"
	case "sb":
		return "Scripture Burrito"
	default:
		return metadataType
	}
}

// GetMetadataTypeIconURL returns the metadata type icon URL
func GetMetadataTypeIconURL(metadataType string) string {
	switch metadataType {
	case "ts":
		return "https://images.squarespace-cdn.com/content/v1/591e003db8a79bd6e6c9ffae/1551971267914-WQIJ24YUJHADQ5HHQF72/icon-ts.png?format=300w"
	case "tc":
		return "https://images.squarespace-cdn.com/content/v1/5b927f2055b02cde84ad8b52/1536327552849-6GNVW6FSTRG1KMSNMM94/icon-tc.png"
	case "rc":
		return ""
	case "sb":
		return "https://docs.burrito.bible/en/v1.0.0-rc1/_images/burrito_logo.png"
	default:
		return "https://avatars.githubusercontent.com/u/48364298?s=32&v=4"
	}
}
