// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

// Schemas reads the content of a specific schema from static/bindata or custom path.
func Schemas(name string) ([]byte, error) {
	return AssetFS().ReadFile("schema", name)
}
