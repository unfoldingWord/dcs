// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"context"
	"strconv"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/structs"
)

// GetTcTsManifestFromBlob reads a manifest.json blob; see ParseTcTsManifest
func GetTcTsManifestFromBlob(ctx context.Context, blob *git.Blob) (*structs.TcTsManifest, error) {
	buf, err := ReadFileFromBlob(ctx, blob)
	if err != nil {
		return nil, err
	}
	t, err := ParseTcTsManifest(buf)
	if err != nil || t.MetadataType == "" {
		return nil, err
	}
	return t, nil
}

// ParseTcTsManifest unmarshals the content of a manifest.json and derives the DCS
// metadata fields from it. MetadataType is left empty when the file is neither a
// translationCore (tc_version >= 7) nor a translationStudio (package_version >= 3) manifest.
func ParseTcTsManifest(buf []byte) (*structs.TcTsManifest, error) {
	t := &structs.TcTsManifest{}
	if err := json.Unmarshal(buf, t); err != nil {
		return nil, err
	}
	if t.TcVersion >= 7 {
		t.MetadataVersion = strconv.Itoa(t.TcVersion)
		t.MetadataType = "tc"
		t.Format = "usfm"
		t.Subject = "Aligned Bible"
	} else if t.TsVersion >= 3 {
		t.MetadataVersion = strconv.Itoa(t.TsVersion)
		t.MetadataType = "ts"

		if t.TsVersion == 3 {
			t.Project.ID = t.ProjectID
			t.Type.ID = "text"
		}

		if t.Resource.ID == "" {
			if t.ResourceID != "" {
				t.Resource.ID = t.ResourceID
			} else {
				t.Resource.ID = t.Project.ID
			}
		}

		if t.Resource.Name == "" {
			t.Resource.Name = strings.ToUpper(t.Resource.ID)
		}

		if t.Project.Name == "" {
			t.Project.Name = strings.ToUpper(t.Project.ID)
		}

		if t.Project.Type == "" {
			t.Project.Type = t.Type.ID
		}

		if t.Resource.ID == "obs" && (t.Project.Type == "tn" || t.Type.ID == "tn") {
			t.Resource.ID = "obs-tn"
			t.Subject = "OBS Translation Notes"
		} else if t.Resource.ID == "obs" && (t.Project.Type == "tq" || t.Type.ID == "tq") {
			t.Resource.ID = "obs-tq"
			t.Subject = "OBS Translation Questions"
		} else if t.Resource.ID == "obs" {
			t.Subject = "Open Bible Stories"
		} else if t.Project.Type == "tn" || t.Type.ID == "tn" {
			t.Resource.ID = "tn"
			t.Subject = "Translation Notes"
		} else if t.Project.Type == "tq" || t.Type.ID == "tq" {
			t.Resource.ID = "tq"
			t.Subject = "Translation Questions"
		} else if t.Project.Type == "tw" || t.Type.ID == "tw" {
			t.Resource.ID = "tw"
			t.Subject = "Translation Words"
		} else if t.Project.Type == "ta" || t.Type.ID == "ta" {
			t.Resource.ID = "ta"
			t.Subject = "Translation Academy"
		} else {
			if t.Resource.ID == "" {
				t.Resource.ID = "bible"
			}
			t.Subject = "Bible"
		}
	} else {
		return t, nil
	}

	// The flavor of the burrito /sb/ converts the repo to, by way of go-ts2rc, which turns
	// ts Translation Notes and Questions into their TSV subjects
	convertedSubject := t.Subject
	if t.Subject == "Translation Notes" || t.Subject == "Translation Questions" {
		convertedSubject = "TSV " + t.Subject
	}
	sbFlavor := ConvertedSBFlavors[convertedSubject]
	t.FlavorType, t.Flavor = sbFlavor.FlavorType, sbFlavor.Flavor

	if t.Resource.Name != "" {
		t.Title = t.Resource.Name
	}
	if !strings.EqualFold(t.Resource.ID, "obs") && t.Project.Name != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(t.Project.Name)) {
		if t.Title != "" {
			t.Title += " - "
		}
		t.Title += t.Project.Name
	}

	return t, nil
}
