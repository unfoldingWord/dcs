// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The schemas are compiled from options/schema only; these tests fail if any $ref
// escapes that directory (which would silently require network access in production).
func TestGetSB100SchemaCompilesFromLocalOptions(t *testing.T) {
	setting.SetupGiteaTestEnv()

	schema, err := GetSB100Schema(true)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Reusing the compiled schema must not recompile it
	again, err := GetSB100Schema(false)
	require.NoError(t, err)
	assert.Same(t, schema, again)

	valErr, err := ValidateMapBySB100Schema(map[string]any{"format": "not a burrito"})
	require.NoError(t, err)
	require.NotNil(t, valErr, "a document missing every required SB property must fail validation")

	valErr, err = ValidateMapBySB100Schema(nil)
	require.NoError(t, err)
	require.NotNil(t, valErr)
}

func TestGetRC02SchemaCompilesFromLocalOptions(t *testing.T) {
	setting.SetupGiteaTestEnv()

	schema, err := GetRC02Schema(true)
	require.NoError(t, err)
	require.NotNil(t, schema)

	valErr, err := ValidateMapByRC02Schema(map[string]any{"dublin_core": map[string]any{}})
	require.NoError(t, err)
	require.NotNil(t, valErr, "a manifest without the required dublin_core fields must fail validation")
}
