// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func addReaction(t *testing.T, doer *user_model.User, issue *Issue, comment *Comment, content string) {
	var reaction *Reaction
	var err error
	if comment == nil {
		reaction, err = CreateIssueReaction(doer, issue, content)
	} else {
		reaction, err = CreateCommentReaction(doer, issue, comment, content)
	}
	assert.NoError(t, err)
	assert.NotNil(t, reaction)
}

func TestIssueAddReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	addReaction(t, user1, issue1, nil, "heart")

	unittest.AssertExistsAndLoadBean(t, &Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1.ID})
}

func TestIssueAddDuplicateReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	addReaction(t, user1, issue1, nil, "heart")

	reaction, err := CreateReaction(&ReactionOptions{
		Doer:  user1,
		Issue: issue1,
		Type:  "heart",
	})
	assert.Error(t, err)
	assert.Equal(t, ErrReactionAlreadyExist{Reaction: "heart"}, err)

	existingR := unittest.AssertExistsAndLoadBean(t, &Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1.ID}).(*Reaction)
	assert.Equal(t, existingR.ID, reaction.ID)
}

func TestIssueDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	addReaction(t, user1, issue1, nil, "heart")

	err := DeleteIssueReaction(user1, issue1, "heart")
	assert.NoError(t, err)

	unittest.AssertNotExistsBean(t, &Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1.ID})
}

func TestIssueReactionCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.UI.ReactionMaxUserNum = 2

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	ghost := user_model.NewGhostUser()

	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 2}).(*Issue)

	addReaction(t, user1, issue, nil, "heart")
	addReaction(t, user2, issue, nil, "heart")
	addReaction(t, user3, issue, nil, "heart")
	addReaction(t, user3, issue, nil, "+1")
	addReaction(t, user4, issue, nil, "+1")
	addReaction(t, user4, issue, nil, "heart")
	addReaction(t, ghost, issue, nil, "-1")

	err := issue.loadReactions(db.DefaultContext)
	assert.NoError(t, err)

	assert.Len(t, issue.Reactions, 7)

	reactions := issue.Reactions.GroupByType()
	assert.Len(t, reactions["heart"], 4)
	assert.Equal(t, 2, reactions["heart"].GetMoreUserCount())
	assert.Equal(t, user1.DisplayName()+", "+user2.DisplayName(), reactions["heart"].GetFirstUsers())
	assert.True(t, reactions["heart"].HasUser(1))
	assert.False(t, reactions["heart"].HasUser(5))
	assert.False(t, reactions["heart"].HasUser(0))
	assert.Len(t, reactions["+1"], 2)
	assert.Equal(t, 0, reactions["+1"].GetMoreUserCount())
	assert.Len(t, reactions["-1"], 1)
}

func TestIssueCommentAddReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	comment1 := unittest.AssertExistsAndLoadBean(t, &Comment{ID: 1}).(*Comment)

	addReaction(t, user1, issue1, comment1, "heart")

	unittest.AssertExistsAndLoadBean(t, &Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1.ID, CommentID: comment1.ID})
}

func TestIssueCommentDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue1.RepoID}).(*repo_model.Repository)

	comment1 := unittest.AssertExistsAndLoadBean(t, &Comment{ID: 1}).(*Comment)

	addReaction(t, user1, issue1, comment1, "heart")
	addReaction(t, user2, issue1, comment1, "heart")
	addReaction(t, user3, issue1, comment1, "heart")
	addReaction(t, user4, issue1, comment1, "+1")

	err := comment1.LoadReactions(repo1)
	assert.NoError(t, err)
	assert.Len(t, comment1.Reactions, 4)

	reactions := comment1.Reactions.GroupByType()
	assert.Len(t, reactions["heart"], 3)
	assert.Len(t, reactions["+1"], 1)
}

func TestIssueCommentReactionCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

	issue1 := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	comment1 := unittest.AssertExistsAndLoadBean(t, &Comment{ID: 1}).(*Comment)

	addReaction(t, user1, issue1, comment1, "heart")
	assert.NoError(t, DeleteCommentReaction(user1, issue1, comment1, "heart"))

	unittest.AssertNotExistsBean(t, &Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1.ID, CommentID: comment1.ID})
}
