// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"
)

// Webhook represents a web hook object.
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

func (h *Hashtag) BeforeInsert() {
	h.CreatedUnix = time.Now().Unix()
	h.UpdatedUnix = h.CreatedUnix
}

func (h *Hashtag) BeforeUpdate() {
	h.UpdatedUnix = time.Now().Unix()
}
