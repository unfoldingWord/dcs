// Copyright 2019 The DCS Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeRenamedToFiles(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", path.Join("user1", "repo2"))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the PR button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	test, _ := htmlDoc.doc.Html()
	assert.EqualValues(t, "Files", test)
}
