// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/structs"
)

func GetTcTsManifestFromBlob(blob *git.Blob) (*structs.TcTsManifest, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}
	t := &structs.TcTsManifest{}
	err = json.Unmarshal(buf, t)
	if err != nil {
		return nil, err
	}
	if t.TcVersion >= 7 {
		t.MetadataVersion = strconv.Itoa(t.TcVersion)
		t.MetadataType = "tc"
		t.Format = "usfm"
		t.Subject = "Aligned Bible"
		t.FlavorType = "scripture"
		t.Flavor = "textTranslation"
	} else if t.TsVersion >= 3 {
		t.MetadataVersion = strconv.Itoa(t.TsVersion)
		t.MetadataType = "ts"
		if t.Resource.ID == "" {
			t.Resource.ID = t.ResourceID
		}
		if t.Resource.Name == "" {
			t.Resource.Name = strings.ToUpper(t.Resource.ID)
		}
		if t.Project.Name == "" {
			t.Project.Name = strings.ToUpper(t.Project.ID)
		}
		if t.Resource.ID == "obs" {
			t.Subject = "Open Bible Stories"
			t.FlavorType = "gloss"
			t.Flavor = "textStories"
		} else {
			t.Subject = "Bible"
			t.FlavorType = "scripture"
			t.Flavor = "textTranslation"

		}
	} else {
		return nil, nil
	}

	if t.Resource.Name != "" {
		t.Title = t.Resource.Name
	}
	if strings.ToLower(t.Resource.ID) != "obs" && t.Project.Name != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(t.Project.Name)) {
		if t.Title != "" {
			t.Title += " - "
		}
		t.Title += t.Project.Name
	}

	return t, nil
}
