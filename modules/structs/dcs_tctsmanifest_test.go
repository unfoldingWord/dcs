// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs_test

import (
	"testing"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// TestTcTsManifest_ProjectBinding guards against a GOEXPERIMENT=jsonv2
// regression: the default modules/json unmarshaler matches member names
// case-sensitively, so an untagged Go field only binds an exactly-cased JSON
// key. A tS manifest.json uses lowercase "project": {"id", "name"}, so the
// Project field (and its nested id/name) must carry explicit json tags or the
// project/book name silently drops out of the computed resource title.
func TestTcTsManifest_ProjectBinding(t *testing.T) {
	const manifest = `{
		"package_version": 3,
		"format": "usfm",
		"target_language": {"id": "awa", "name": "Awadhi", "direction": "ltr"},
		"type": {"id": "text", "name": "Text"},
		"project": {"id": "gen", "name": "Genesis"},
		"resource": {"id": "ult", "name": "unfoldingWord Literal Text"}
	}`

	m := &structs.TcTsManifest{}
	assert.NoError(t, json.Unmarshal([]byte(manifest), m))

	assert.Equal(t, "gen", m.Project.ID, `lowercase "project" must bind under jsonv2 (needs json:"project" + nested id/name tags)`)
	assert.Equal(t, "Genesis", m.Project.Name)

	// Siblings that were already tagged correctly, as a control.
	assert.Equal(t, "ult", m.Resource.ID)
	assert.Equal(t, "awa", m.TargetLanguage.ID)
}
