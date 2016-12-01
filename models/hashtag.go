// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
	"github.com/go-xorm/xorm"
)

// Hashtag represents a hashtag object.
type Hashtag struct {
	ID           int64 `xorm:"pk autoincr"`
	UserID       int64
	RepoID       int64
	Lang         string `xorm:"TEXT NOT NULL"`
	TagName      string `xorm:"TEXT NOT NULL"`
	FilePath     string `xorm:"TEXT NOT NULL"`
	CreatedUnix  int64
	UpdatedUnix  int64
}

// Gets a summary of the hashtags for repositories with the specified prefix.
// The `engine` parameter is for unit testing.
// If `engine` is nil, then the `x` engine from models.go is used.
func GetHashtagSummary(engine *xorm.Engine, repoPrefix string) ([]map[string]string, error) {

	sql := `SELECT h.tag_name, COUNT(*) AS count_of_occurrences
FROM repository AS r INNER JOIN hashtag AS h ON r.id = h.repo_id
WHERE r.name LIKE ?
GROUP BY h.tag_name
ORDER BY LOWER(h.tag_name)`

	// if engine is nil, use the `x` engine from models.go
	if engine == nil {
		engine = x
	}

	// get the requested list of tags
	results, err := engine.Query(sql, repoPrefix + "%")

	if err != nil {
		return nil, err
	}

	// convert the byte arrays returned by the Query function into regular strings
	returnVal := make([]map[string]string, len(results))
	for idx, row := range results {
		values := make(map[string]string)
		for key, value := range row {
			values[key] = string(value)
		}
		returnVal[idx] = values
	}

	return returnVal, nil
}

func (h *Hashtag) BeforeInsert() {
	h.CreatedUnix = time.Now().Unix()
	h.UpdatedUnix = h.CreatedUnix
}

func (h *Hashtag) BeforeUpdate() {
	h.UpdatedUnix = time.Now().Unix()
}
