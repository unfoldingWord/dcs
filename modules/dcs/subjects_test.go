// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"os"
	"path/filepath"
	"strings"
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
			// go-rc2sb resolves a helps subject by its project, so OBS subjects get the obs project
			project, projectPath := "tit", "./x_TIT.tsv"
			if subject == "Open Bible Stories" {
				project, projectPath = "obs", "./content" // the stories' folder, where the others take a TSV
			} else if strings.Contains(subject, "OBS") {
				project, projectPath = "obs", "./x_OBS.tsv"
			}
			write("manifest.yaml", `dublin_core:
  conformsto: rc0.2
  identifier: x
  subject: '`+subject+`'
  title: X
  language: {identifier: en, title: English, direction: ltr}
projects:
  - {identifier: `+project+`, title: X, path: `+projectPath+`}
`)
			write("x_OBS.tsv", "Reference\tID\tTags\tQuote\tOccurrence\tNote\n")
			write("x_TIT.tsv", "Reference\tID\tTags\tQuote\tOccurrence\tNote\n")
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

func TestResourceToSubjectMapPrefersTSV(t *testing.T) {
	for resource, subject := range map[string]string{
		"sn":     "TSV Study Notes",
		"sq":     "TSV Study Questions",
		"obs-sn": "TSV OBS Study Notes",
		"obs-tn": "TSV OBS Translation Notes",
		"ta":     "Translation Academy",
	} {
		assert.Equal(t, subject, ResourceToSubjectMap[resource], resource)
	}
}

func TestSBConvertible(t *testing.T) {
	for _, tc := range []struct {
		metadataType, subject string
		want                  bool
	}{
		{"sb", "Greek Lexicon", true},
		{"tc", "Aligned Bible", true},
		{"rc", "Translation Notes", true},
		{"rc", "OBS Study Notes", true},
		{"rc", "Greek Lexicon", false},
		{"rc", "Bible Study Notes", false},
		{"ts", "Bible", true},
		{"ts", "Open Bible Stories", true},
		{"ts", "Translation Notes", true},
		{"ts", "Translation Questions", true},
		{"ts", "OBS Translation Notes", true},
		{"ts", "OBS Translation Questions", true},
		{"ts", "Translation Words", true},
		{"ts", "Translation Academy", true},
		{"ts", "Study Notes", false},
		{"", "Bible", false},
	} {
		assert.Equal(t, tc.want, SBConvertible(tc.metadataType, tc.subject), "%s %s", tc.metadataType, tc.subject)
	}
}
