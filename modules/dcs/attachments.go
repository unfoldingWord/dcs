// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"regexp"
	"strings"
)

// AttachmentContentType is the general kind of content a release attachment
// holds, determined from its name (and remote URL, if any).
type AttachmentContentType int

// Attachment content types, in the order they are tested by
// GetAttachmentContentType. Other is the catch-all for anything that does not
// match audio, video, PDF or a streaming platform.
const (
	AttachmentContentTypeOther AttachmentContentType = iota
	AttachmentContentTypeAudio
	AttachmentContentTypeVideo
	AttachmentContentTypePDF
	AttachmentContentTypeStream
)

// AudioAttachmentExtensions are the file extensions that mark a release
// attachment as audio content. They are matched anywhere in the attachment
// name preceded by ".", "_" or "-", so both "en_obs_01_128kbps.mp3" and
// "fr_obs_v4.3_mp3_128kbps.zip" count as audio.
var AudioAttachmentExtensions = []string{
	"mp3",
	"m4a",
	"aac",
	"ogg",
	"oga",
	"opus",
	"wav",
	"flac",
	"wma",
}

// VideoAttachmentExtensions are the file extensions that mark a release
// attachment as video content, matched the same way as audio extensions.
var VideoAttachmentExtensions = []string{
	"mp4",
	"m4v",
	"mov",
	"avi",
	"mkv",
	"webm",
	"wmv",
	"flv",
	"mpg",
	"mpeg",
	"3gp",
}

// PDFAttachmentExtensions are the file extensions that mark a release
// attachment as PDF content, matched the same way as audio extensions.
var PDFAttachmentExtensions = []string{
	"pdf",
}

// StreamingPlatformDomains are domains of streaming platforms. An attachment
// whose name or remote URL contains one of these is streaming content, e.g.
// "YouTube|https://www.youtube.com/playlist?list=..."
var StreamingPlatformDomains = []string{
	"youtube.com",
	"youtu.be",
	"vimeo.com",
	"dailymotion.com",
	"twitch.tv",
	"rumble.com",
	"bilibili.com",
	"soundcloud.com",
	"spotify.com",
	"wistia.com",
	"brightcove.com",
}

// jsonManifestAttachmentNameRE matches the files.json / links.json (also
// file.json / link.json) manifests that the door43 metadata service expands
// into remote release attachments. See docs/dcs/remote-release-attachments.md.
var jsonManifestAttachmentNameRE = regexp.MustCompile(`(?i)(file|link)s*\.json$`)

// IsJSONManifestAttachmentName returns true if the attachment name is a
// files.json / links.json remote-attachment manifest.
func IsJSONManifestAttachmentName(name string) bool {
	return jsonManifestAttachmentNameRE.MatchString(name)
}

// nameContainsExtension returns true if the lowercased name contains the
// extension preceded by ".", "_" or "-" (e.g. ".mp3", "_mp3" or "-mp3"),
// anywhere in the string.
func nameContainsExtension(lowerName, ext string) bool {
	return strings.Contains(lowerName, "."+ext) ||
		strings.Contains(lowerName, "_"+ext) ||
		strings.Contains(lowerName, "-"+ext)
}

func nameContainsAnyExtension(lowerName string, exts []string) bool {
	for _, ext := range exts {
		if nameContainsExtension(lowerName, ext) {
			return true
		}
	}
	return false
}

// GetAttachmentContentType classifies a release attachment by its name. For
// remote attachments pass the full raw name including the remote URL (the
// "name|url" encoding stored in the attachment table) so streaming URLs are
// seen. Checks are made in this order: audio, video, PDF, streaming platform;
// anything that matches none of them is AttachmentContentTypeOther.
func GetAttachmentContentType(name string) AttachmentContentType {
	lowerName := strings.ToLower(name)
	switch {
	case nameContainsAnyExtension(lowerName, AudioAttachmentExtensions):
		return AttachmentContentTypeAudio
	case nameContainsAnyExtension(lowerName, VideoAttachmentExtensions):
		return AttachmentContentTypeVideo
	case nameContainsAnyExtension(lowerName, PDFAttachmentExtensions):
		return AttachmentContentTypePDF
	default:
		for _, domain := range StreamingPlatformDomains {
			if strings.Contains(lowerName, domain) {
				return AttachmentContentTypeStream
			}
		}
		return AttachmentContentTypeOther
	}
}
