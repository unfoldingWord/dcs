// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package repo

import (
	"net/url"

	"code.gitea.io/gitea/modules/setting"
)

// CatalogSearchURL returns the repository catalog search API URL
func (repo *Repository) CatalogSearchURL() string {
	return setting.AppURL + "api/v1/catalog/search/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
}

// CatalogEntryURL returns the repository catalog entry API URL
func (repo *Repository) CatalogEntryURL() string {
	return setting.AppURL + "api/v1/catalog/entry/" + url.PathEscape(repo.OwnerName) + "/" + url.PathEscape(repo.Name)
}
