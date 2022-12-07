// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

/*** DCS Customizations - Tests for YAML rendering ***/

package yaml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderYaml(t *testing.T) {
	// LF line endings
	contents := "a: 1\nb: 2\n"
	rendered, err := Render([]byte(contents))
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(rendered), "</table>"))

	// CRLF line endings
	contents = "a: 1\r\nb: 2\r\n"
	_, err = Render([]byte(contents))
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(rendered), "</table>"))

	contents = "misformatted&"
	_, err = Render([]byte(contents))
	assert.Error(t, err)
}
