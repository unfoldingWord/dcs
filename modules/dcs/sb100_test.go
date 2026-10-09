// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSBIngredientRepoPaths(t *testing.T) {
	// no key carries the prefix and ingredients/ exists -> resolve under it
	assert.Equal(t, map[string]string{"GEN.usfm": "ingredients/GEN.usfm", "./plan.json": "ingredients/plan.json"},
		SBIngredientRepoPaths([]string{"GEN.usfm", "./plan.json"}, true))
	// no key carries the prefix and there is no ingredients/ dir -> as-is
	assert.Equal(t, map[string]string{"GEN.usfm": "GEN.usfm"}, SBIngredientRepoPaths([]string{"GEN.usfm"}, false))
	// any key carrying the prefix pins all keys to the repo root
	assert.Equal(t, map[string]string{"./ingredients/GEN.usfm": "ingredients/GEN.usfm", "LICENSE.md": "LICENSE.md"},
		SBIngredientRepoPaths([]string{"./ingredients/GEN.usfm", "LICENSE.md"}, true))
	// keys Scribe writes on Windows use "\"
	assert.Equal(t, map[string]string{`ingredients\01.md`: "ingredients/01.md", "LICENSE.md": "LICENSE.md"},
		SBIngredientRepoPaths([]string{`ingredients\01.md`, "LICENSE.md"}, true))
	assert.Equal(t, map[string]string{`content\front\intro.md`: "ingredients/content/front/intro.md"},
		SBIngredientRepoPaths([]string{`content\front\intro.md`}, true))
}
