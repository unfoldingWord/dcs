// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

// LanguageStat describes language statistics of a repository
type Project struct {
	Identifier     string `json:"identifier"`
	Title          string `json:"title"`
	Path           string `json:"path"`
	AlignmentCount int    `json:"alignment_count"`
}
