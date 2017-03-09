// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hashtag

import (
	"regexp"
	"strings"
	"code.gitea.io/gitea/models"
)

func ConvertHashtagsToLinks(repo *models.Repository, html []byte) []byte {
	repoName := repo.LowerName
	indexOfUbn := strings.Index(repoName, "-ubn-")
	if indexOfUbn > 0 {
		hashtagsUrl := repo.HTMLURL() + "/hashtags"
		re, _ := regexp.Compile(`(^|\n|<p>)#([A-Za-uw-z0-9:_-][\w:-]+|v[A-Za-z:_-][\w:-]*)`)
		html = re.ReplaceAll(html, []byte("$1<a href=\""+hashtagsUrl+"/$2\">#$2</a>$3"))
	}
	return html
}
