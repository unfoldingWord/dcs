// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

// SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	/*** DCS Customizatios - adds `repository` to repo options, creates user options. ***/
	SearchOrderByAlphabetically            SearchOrderBy = "`repository`.name ASC"
	SearchOrderByAlphabeticallyReverse     SearchOrderBy = "`repository`.name DESC"
	SearchOrderByLeastUpdated              SearchOrderBy = "`repository`.updated_unix ASC"
	SearchOrderByRecentUpdated             SearchOrderBy = "`repository`.updated_unix DESC"
	SearchOrderByOldest                    SearchOrderBy = "`repository`.created_unix ASC"
	SearchOrderByNewest                    SearchOrderBy = "`repository`.created_unix DESC"
	SearchOrderBySize                      SearchOrderBy = "`repository`.size ASC"
	SearchOrderBySizeReverse               SearchOrderBy = "`repository`.size DESC"
	SearchOrderByID                        SearchOrderBy = "`repository`.id ASC"
	SearchOrderByIDReverse                 SearchOrderBy = "`repository`.id DESC"
	SearchOrderByStars                     SearchOrderBy = "`repository`.num_stars ASC"
	SearchOrderByStarsReverse              SearchOrderBy = "`repository`.num_stars DESC"
	SearchOrderByForks                     SearchOrderBy = "`repository`.num_forks ASC"
	SearchOrderByForksReverse              SearchOrderBy = "`repository`.num_forks DESC"
	SearchUserOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchUserOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchUserOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchUserOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchUserOrderByID                    SearchOrderBy = "id ASC"
	SearchUserOrderByIDReverse             SearchOrderBy = "id DESC"
	/*** END DCS Customizatios ***/
)
