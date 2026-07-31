// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveSBIngredientPrefix(t *testing.T) {
	// no key carries the prefix and ingredients/ exists -> resolve under it
	assert.Equal(t, "ingredients/", resolveSBIngredientPrefix([]string{"GEN.usfm", "plan.json"}, true))
	// no key carries the prefix and there is no ingredients/ dir -> as-is
	assert.Empty(t, resolveSBIngredientPrefix([]string{"GEN.usfm"}, false))
	// any key carrying the prefix pins all keys to the repo root
	assert.Empty(t, resolveSBIngredientPrefix([]string{"ingredients/GEN.usfm", "LICENSE.md"}, true))
	assert.Empty(t, resolveSBIngredientPrefix([]string{"./ingredients/GEN.usfm", "GEN.usfm"}, true))
	// "./" prefixes are tolerated when detecting the convention
	assert.Equal(t, "ingredients/", resolveSBIngredientPrefix([]string{"./GEN.usfm"}, true))
}
