// Copyright 2020 The DCS Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Door43 Metadata represents a repository's metadata of a tag or default branch
type Door43Metadata struct {
	ID int64 `json:"id"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
}
