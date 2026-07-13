// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAttachmentContentType(t *testing.T) {
	tests := []struct {
		name     string
		expected AttachmentContentType
	}{
		// Audio: extension as the actual file extension
		{"en_obs_v6_01_128kbps.mp3|https://cdn.door43.org/en/obs/v6/128kbps/en_obs_01_128kbps.mp3", AttachmentContentTypeAudio},
		// Audio: extension embedded in a zip name with "_" separator
		{"fr_obs_v4.3_mp3_128kbps.zip|https://cdn.door43.org/fr/obs/v4.3/128kbps/fr_obs_v4.3_mp3_128kbps.zip", AttachmentContentTypeAudio},
		{"en_ulb_wav.zip", AttachmentContentTypeAudio},
		{"en_obs-mp3-64kbps.zip", AttachmentContentTypeAudio},
		{"EN_OBS_MP3_32KBPS.ZIP", AttachmentContentTypeAudio},
		// Video
		{"en_obs_v6_01_720p.mp4", AttachmentContentTypeVideo},
		{"hi_obs_mp4_360p.zip", AttachmentContentTypeVideo},
		{"en_obs-webm-720p.zip", AttachmentContentTypeVideo},
		// Audio wins over video when both could match (checked first)
		{"en_obs_mp3_and_mp4.zip", AttachmentContentTypeAudio},
		// PDF
		{"en_obs_v9.pdf", AttachmentContentTypePDF},
		{"en_obs_pdf_letter.zip", AttachmentContentTypePDF},
		// Stream: platform domain in the remote URL of a "name|url" attachment
		{"YouTube|https://www.youtube.com/playlist?list=PLdE_o99zEY27D4H6XdxmN9ZmUluJKZZTo", AttachmentContentTypeStream},
		{"Vimeo|https://vimeo.com/123456789", AttachmentContentTypeStream},
		{"Short link|https://youtu.be/dQw4w9WgXcQ", AttachmentContentTypeStream},
		// Other: catch-all
		{"en_obs_v9.docx", AttachmentContentTypeOther},
		{"en_obs_bloom.bloompub", AttachmentContentTypeOther},
		{"en_obs.epub", AttachmentContentTypeOther},
		{"source.zip", AttachmentContentTypeOther},
	}
	for _, test := range tests {
		assert.Equal(t, test.expected, GetAttachmentContentType(test.name), "name: %s", test.name)
	}
}

func TestIsJSONManifestAttachmentName(t *testing.T) {
	assert.True(t, IsJSONManifestAttachmentName("files.json"))
	assert.True(t, IsJSONManifestAttachmentName("file.json"))
	assert.True(t, IsJSONManifestAttachmentName("links.json"))
	assert.True(t, IsJSONManifestAttachmentName("link.json"))
	assert.True(t, IsJSONManifestAttachmentName("Links.JSON"))
	assert.False(t, IsJSONManifestAttachmentName("metadata.json"))
	assert.False(t, IsJSONManifestAttachmentName("files.json.zip"))
}
