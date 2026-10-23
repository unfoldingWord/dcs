// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDCSCacheSBArchives(t *testing.T) {
	defer test.MockVariableValue(&DCS)()

	cfg, err := NewConfigProviderFromData(``)
	require.NoError(t, err)
	loadDCSFrom(cfg)
	assert.True(t, DCS.CacheSBArchives, "kept by default, unlike upstream's streamed git archives")

	cfg, err = NewConfigProviderFromData("[dcs]\nCACHE_SB_ARCHIVES = false\n")
	require.NoError(t, err)
	loadDCSFrom(cfg)
	assert.False(t, DCS.CacheSBArchives)
}
