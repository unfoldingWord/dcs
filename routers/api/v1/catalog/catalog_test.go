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
