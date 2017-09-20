// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
)

func TestExploreRepos(t *testing.T) {
	prepareTestEnv(t)

	// we (Door43) require authentication to view /explore/... pages
	sess := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/explore/repos")
	sess.MakeRequest(t, req, http.StatusOK)
}
