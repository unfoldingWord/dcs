// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package door43metadata

/*** Stage ***/

// Stage type for choosing which level of stage to return in the Catalog results
type Stage int

// Stage values
const (
	StageProd    Stage = iota // 0
	StagePreProd Stage = 1
	StageDraft   Stage = 2
	StageLatest  Stage = 3
)

// StageMap map from string to Stage (int)
var StageMap = map[string]Stage{
	"prod":    StageProd,
	"preprod": StagePreProd,
	"draft":   StageDraft,
	"latest":  StageLatest,
}

// StageToStringMap map from stage (int) to string
var StageToStringMap = map[Stage]string{
	StageProd:    "prod",
	StagePreProd: "preprod",
	StageDraft:   "draft",
	StageLatest:  "latest",
}

// String returns string repensation of a Stage (int)
func (s *Stage) String() string {
	return StageToStringMap[*s]
}

/*** END Stage ***/
