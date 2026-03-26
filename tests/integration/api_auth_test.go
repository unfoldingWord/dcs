// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestf(t, "GET", "/api/v1/user").AddBasicAuth("user2")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequestf(t, "GET", "/api/v1/user").AddBasicAuth("user2", "wrong-password")
	resp := MakeRequest(t, req, http.StatusUnauthorized)
	assert.Contains(t, resp.Body.String(), `{"message":"invalid username, password or token"`)

	req = NewRequestf(t, "GET", "/api/v1/user").AddBasicAuth("user-not-exist")
	resp = MakeRequest(t, req, http.StatusUnauthorized)
	assert.Contains(t, resp.Body.String(), `{"message":"invalid username, password or token"`)

	// Invalid Bearer token on a public endpoint should succeed as unauthenticated.
	// The server treats an invalid/expired token as "no auth" so public endpoints
	// still work — this prevents cascading failures in clients with stale tokens.
	req = NewRequestf(t, "GET", "/api/v1/users/user2/repos").AddTokenAuth("Bearer wrong_token")
	MakeRequest(t, req, http.StatusOK)

	// Invalid Bearer token on a protected endpoint (/api/v1/user requires auth)
	// should still return 401 — reqToken() middleware enforces authentication.
	req = NewRequestf(t, "GET", "/api/v1/user").AddTokenAuth("Bearer wrong_token")
	MakeRequest(t, req, http.StatusUnauthorized)
}
