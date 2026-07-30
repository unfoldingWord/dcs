// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUSFMBookID(t *testing.T) {
	assert.Equal(t, "GEN", usfmBookID("\\id GEN unfoldingWord Literal Text\n\\c 1\n"))
	assert.Equal(t, "GEN", usfmBookID("\ufeff\\id GEN\n"), "UTF-8 BOM is ignored")
	assert.Equal(t, "TIT", usfmBookID("\n  \\id TIT\r\n\\c 1\n"), "leading whitespace is tolerated")
	assert.Equal(t, "3JN", usfmBookID("\\id 3JN"), "no trailing newline")
	assert.Empty(t, usfmBookID(""))
	assert.Empty(t, usfmBookID("\\c 1\n\\v 1 In the beginning\n"), "no \\id marker")
	assert.Empty(t, usfmBookID("id GEN\n"), "missing backslash")
	assert.Empty(t, usfmBookID("\\id \n"), "empty book code")
}
