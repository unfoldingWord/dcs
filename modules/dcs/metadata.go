// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

// GetMetadataTypeIcon returns the metadata type icon
func GetMetadataTypeIcon(metadataType string) string {
	switch metadataType {
	case "rc":
		return "rc.png"
	case "ts":
		return "ts.png"
	case "tc":
		return "tc.png"
	case "sb":
		return "sb.png"
	default:
		return "uw.png"
	}
}
