// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

/*** Stage ***/

// Stage type for choosing which level of stage to return in the Catalog results
type Stage int

// Stage values
const (
	StageProd    Stage = 1
	StagePreProd Stage = 2
	StageLatest  Stage = 3
	StageOther   Stage = 4
)

// StageMap map from string to Stage (int)
var StageMap = map[string]Stage{
	"prod":    StageProd,
	"preprod": StagePreProd,
	"latest":  StageLatest,
	"other":   StageOther,
	"tag":     StageOther,
	"branch":  StageOther,
}

// StageToStringMap map from stage (int) to string
var StageToStringMap = map[Stage]string{
	StageProd:    "prod",
	StagePreProd: "preprod",
	StageLatest:  "latest",
	StageOther:   "other",
}

// String returns string repensation of a Stage (int)
func (s *Stage) String() string {
	return StageToStringMap[*s]
}

/*** END Stage ***/
