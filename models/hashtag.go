// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
	"fmt"
	"github.com/go-xorm/xorm"
)

// Hashtag represents a hashtag object.
type Hashtag struct {
	ID          int64 `xorm:"pk autoincr"`
	UserID      int64
	User        *User  `xorm:"-"`
	RepoID      int64
	Repo        *Repository  `xorm:"-"`
	Lang        string `xorm:"TEXT NOT NULL"`
	TagName     string `xorm:"TEXT NOT NULL"`
	FilePath    string `xorm:"TEXT NOT NULL"`
	FileURL     string `xorm:"-"`
	CreatedUnix int64
	UpdatedUnix int64
}

// LoadAttributes loads the attribute of this hashtag.
func (hashtag *Hashtag) LoadAttributes() error {
	return hashtag.loadAttributes(x)
}

func (hashtag *Hashtag) loadAttributes(e Engine) (err error) {
	if hashtag.Repo == nil {
		hashtag.Repo, err = getRepositoryByID(e, hashtag.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %v", hashtag.RepoID, err)
		}
	}

	if hashtag.User == nil {
		hashtag.User, err = getUserByID(e, hashtag.UserID)
		if err != nil {
			hashtag.UserID = -1
			hashtag.User = NewGhostUser()
			if !IsErrUserNotExist(err) {
				return fmt.Errorf("getUserByID.(user) [%d]: %v", hashtag.UserID, err)
			}
			err = nil
			return
		}
	}

	if hashtag.FileURL == "" {
		hashtag.FileURL = hashtag.Repo.HTMLURL() + "/src/master/" + hashtag.FilePath
	}

	return nil
}

// GetHashtagSummary gets a summary of the hashtags for repositories with the specified prefix.
func GetHashtagSummary(repoPrefix string, ownerID int64) ([]map[string]string, error) {
	return getHashtagSummary(x, repoPrefix, ownerID)
}

func getHashtagSummary(engine *xorm.Engine, repoPrefix string, ownerID int64) ([]map[string]string, error) {

	sql := `SELECT h.tag_name, COUNT(*) AS count_of_occurrences
FROM repository AS r INNER JOIN hashtag AS h ON r.id = h.repo_id
WHERE r.owner_id = ? AND r.name LIKE ?
GROUP BY h.tag_name
ORDER BY LOWER(h.tag_name)`

	// get the requested list of tags
	results, err := engine.Query(sql, ownerID, repoPrefix+"%")

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

// GetHashtags gets all hashtags by the repo prefix, UserID, and TagName
func GetHashtags(repoPrefix string, userID int64, tagName string) ([]Hashtag, error) {
	return getHashtags(x, repoPrefix, userID, tagName)
}

func getHashtags(engine *xorm.Engine, repoPrefix string, userID int64, tagName string) ([]Hashtag, error) {
	hashtags := []Hashtag{}
	if err := engine.
		Join("INNER", "repository", "hashtag.repo_id = repository.id").
		Where("tag_name = ?", tagName).
		And("user_id = ?", userID).
		And("repository.lower_name LIKE ?", repoPrefix+"%").
		And("repository.lower_name LIKE ?", repoPrefix+"%").
		Asc("file_path").
		Find(&hashtags); err != nil {
		return nil, err;
	}

	for i := range hashtags {
		if err := hashtags[i].LoadAttributes(); err != nil {
			return nil, fmt.Errorf("LoadAttributes [%d]: %v", hashtags[i].ID, err)
		}
	}

	return hashtags, nil
}

func (h *Hashtag) BeforeInsert() {
	h.CreatedUnix = time.Now().Unix()
	h.UpdatedUnix = h.CreatedUnix
}

func (h *Hashtag) BeforeUpdate() {
	h.UpdatedUnix = time.Now().Unix()
}
