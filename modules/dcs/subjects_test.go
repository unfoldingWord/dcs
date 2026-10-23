// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rc2sb "github.com/unfoldingWord/go-rc2sb"
	"github.com/unfoldingWord/go-rc2sb/handler"
)

// TestConvertedSBFlavors converts a minimal RC of every subject go-rc2sb registers and
// checks that ConvertedSBFlavors gives the flavor of the burrito it produces.
func TestConvertedSBFlavors(t *testing.T) {
	subjects := handler.SupportedSubjects()
	assert.Len(t, ConvertedSBFlavors, len(subjects), "ConvertedSBFlavors lists a subject go-rc2sb no longer converts")
	for _, subject := range subjects {
		t.Run(subject, func(t *testing.T) {
			inDir := t.TempDir()
			write := func(path, content string) {
				path = filepath.Join(inDir, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
			}
			projectPath := "./x_OBS.tsv"
			if subject == "Open Bible Stories" {
				projectPath = "./content" // the stories' folder, where the others take a TSV
			}
			write("manifest.yaml", `dublin_core:
  conformsto: rc0.2
  identifier: x
  subject: '`+subject+`'
  title: X
  language: {identifier: en, title: English, direction: ltr}
projects:
  - {identifier: obs, title: X, path: `+projectPath+`}
`)
			write("x_OBS.tsv", "Reference\tID\tTags\tQuote\tOccurrence\tNote\n")
			write("bible/kt/god.md", "# God\n")

			outDir := t.TempDir()
			_, err := rc2sb.Convert(t.Context(), inDir, outDir, rc2sb.Options{})
			require.NoError(t, err)
			data, err := os.ReadFile(filepath.Join(outDir, "metadata.json"))
			require.NoError(t, err)
			var metadata struct {
				Type struct {
					FlavorType struct {
						Name   string `json:"name"`
						Flavor struct {
							Name string `json:"name"`
						} `json:"flavor"`
					} `json:"flavorType"`
				} `json:"type"`
			}
			require.NoError(t, json.Unmarshal(data, &metadata))
			assert.Equal(t, SBFlavor{metadata.Type.FlavorType.Name, metadata.Type.FlavorType.Flavor.Name}, ConvertedSBFlavors[subject])
		})
	}
}
