// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import "code.gitea.io/gitea/modules/util"

// QueryStrings After calling QueryStrings on the context, it also separates strings that have commas into substrings
func (b *Base) QueryStrings(name string) []string {
	strs := b.FormStrings(name)
	if len(strs) == 0 {
		return strs
	}
	var newStrs []string
	for _, str := range strs {
		newStrs = append(newStrs, util.SplitAtCommaNotInString(str, false)...)
	}
	return newStrs
}
