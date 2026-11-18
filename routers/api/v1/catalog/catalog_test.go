// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package catalog

import (
	"testing"

	"gitea.dev/models/door43metadata"

	"github.com/stretchr/testify/assert"
)

func TestSearchOrderByMapFlavorSortDirections(t *testing.T) {
	assert.Equal(t, door43metadata.CatalogOrderByFlavorType, searchOrderByMap["asc"]["flavortype"])
	assert.Equal(t, door43metadata.CatalogOrderByFlavorTypeReverse, searchOrderByMap["desc"]["flavortype"])
	assert.Equal(t, door43metadata.CatalogOrderByFlavor, searchOrderByMap["asc"]["flavor"])
	assert.Equal(t, door43metadata.CatalogOrderByFlavorReverse, searchOrderByMap["desc"]["flavor"])
}

// TestBookPackageSubjects checks that the default book package of each Study Notes and
// Study Questions subject holds the subjects door43-preview-renderers renders it with.
func TestBookPackageSubjects(t *testing.T) {
	for subject, needs := range map[string][]string{
		"TSV Study Notes":         {"Aligned Bible", "Hebrew Old Testament", "Greek New Testament"},
		"TSV Study Questions":     {"Aligned Bible", "Hebrew Old Testament", "Greek New Testament"},
		"TSV OBS Study Notes":     {"Open Bible Stories"},
		"TSV OBS Study Questions": {"Open Bible Stories"},
	} {
		assert.Subset(t, bookPackageSubjects(subject), append(needs, subject), subject)
	}
	assert.Equal(t, []string{"Bible"}, bookPackageSubjects("Bible"))
}
