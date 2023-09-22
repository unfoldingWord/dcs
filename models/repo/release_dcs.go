// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import "regexp"

func (r *Release) IsCatalogVersion() bool {
	return regexp.MustCompile(`^v\d`).Match([]byte(r.TagName)) || regexp.MustCompile(`^\d\d\d\d`).Match([]byte(r.TagName))
}
