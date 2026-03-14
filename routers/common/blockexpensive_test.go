// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockExpensive(t *testing.T) {
	cases := []struct {
		expensive bool
		routePath string
	}{
		{false, "/user/xxx"},
		{false, "/login/xxx"},
		{false, "/{username}/{reponame}/archive/xxx"}, // DCS Customizations: not expensive
		{true, "/{username}/{reponame}/graph"},
		{false, "/{username}/{reponame}/src/xxx"},       // DCS Customizations: not expensive (only src/commit/ is)
		{true, "/{username}/{reponame}/src/commit/xxx"}, // DCS Customizations
		{true, "/{username}/{reponame}/wiki/xxx"},
		{true, "/{username}/{reponame}/activity/xxx"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expensive, isRoutePathExpensive(c.routePath), "routePath: %s", c.routePath)
	}

	assert.True(t, isRoutePathForLongPolling("/user/events"))
}
