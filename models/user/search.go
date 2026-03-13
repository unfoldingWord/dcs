// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// AdminUserOrderByMap represents all possible admin user search orders
// This should only be used for admin API endpoints as we should not expose "updated" ordering which could expose recent user activity including logins.
var AdminUserOrderByMap = map[string]map[string]db.SearchOrderBy{
	"asc": {
		"name":    db.SearchOrderByAlphabetically,
		"created": db.SearchOrderByOldest,
		"updated": db.SearchOrderByLeastUpdated,
		"id":      db.SearchOrderByID,
	},
	"desc": {
		"name":    db.SearchOrderByAlphabeticallyReverse,
		"created": db.SearchOrderByNewest,
		"updated": db.SearchOrderByRecentUpdated,
		"id":      db.SearchOrderByIDReverse,
	},
}

// SearchUserOptions contains the options for searching
type SearchUserOptions struct {
	db.ListOptions

	Keyword       string
	Types         []UserType
	UID           int64
	LoginName     string // this option should be used only for admin user
	SourceID      int64  // this option should be used only for admin user
	OrderBy       db.SearchOrderBy
	Visible       []structs.VisibleType
	Actor         *User // The user doing the search
	SearchByEmail bool  // Search by email as well as username/full name

	SupportedSortOrders container.Set[string] // if not nil, only allow to use the sort orders in this set

	IsActive           optional.Option[bool]
	IsAdmin            optional.Option[bool]
	IsRestricted       optional.Option[bool]
	IsTwoFactorEnabled optional.Option[bool]
	IsProhibitLogin    optional.Option[bool]
	IncludeReserved    bool

	/*** DCS CUSTOMIZATIONS ***/
	IsSpamUser        optional.Option[bool]
	RepoLanguages     []string              // Find repos that have the given language ids in a repo's metadata
	RepoSubjects      []string              // Find repos that have the given subjects in a repo's metadata
	RepoFlavorTypes   []string              // Find repos that have the given flavor types in a repo's metadata
	RepoFlavors       []string              // Find repos that have the given flavors in a repo's metadata
	RepoMetadataTypes []string              // Find repos that have the given metadata types in a repo's metadata
	RepoLanguageIsGL  optional.Option[bool] // Find repos that are gateway languages
	/*** END DCS CUSTOMIZATIONS ***/
}

func (opts *SearchUserOptions) toSearchQueryBase(ctx context.Context) *xorm.Session {
	var cond builder.Cond
	cond = builder.In("type", opts.Types)
	if opts.IncludeReserved {
		switch {
		case slices.Contains(opts.Types, UserTypeIndividual):
			cond = cond.Or(builder.Eq{"type": UserTypeUserReserved}).Or(
				builder.Eq{"type": UserTypeBot},
			).Or(
				builder.Eq{"type": UserTypeRemoteUser},
			)
		case slices.Contains(opts.Types, UserTypeOrganization):
			cond = cond.Or(builder.Eq{"type": UserTypeOrganizationReserved})
		}
	}

	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		keywordCond := builder.Or(
			builder.Like{"lower_name", lowerKeyword},
			builder.Like{"LOWER(full_name)", lowerKeyword},
		)
		if opts.SearchByEmail {
			var emailCond builder.Cond
			emailCond = builder.Like{"LOWER(email)", lowerKeyword}
			if opts.Actor == nil {
				emailCond = emailCond.And(builder.Eq{"keep_email_private": false})
			} else if !opts.Actor.IsAdmin {
				emailCond = emailCond.And(
					builder.Or(
						builder.Eq{"keep_email_private": false},
						builder.Eq{"id": opts.Actor.ID},
					),
				)
			}
			keywordCond = keywordCond.Or(emailCond)
		}

		cond = cond.And(keywordCond)
	}

	// If visibility filtered
	if len(opts.Visible) > 0 {
		cond = cond.And(builder.In("visibility", opts.Visible))
	}

	cond = cond.And(BuildCanSeeUserCondition(opts.Actor))

	if opts.UID > 0 {
		cond = cond.And(builder.Eq{"id": opts.UID})
	}

	if opts.SourceID > 0 {
		cond = cond.And(builder.Eq{"login_source": opts.SourceID})
	}
	if opts.LoginName != "" {
		cond = cond.And(builder.Eq{"login_name": opts.LoginName})
	}

	if opts.IsActive.Has() {
		cond = cond.And(builder.Eq{"is_active": opts.IsActive.Value()})
	}

	if opts.IsAdmin.Has() {
		cond = cond.And(builder.Eq{"is_admin": opts.IsAdmin.Value()})
	}

	if opts.IsRestricted.Has() {
		cond = cond.And(builder.Eq{"is_restricted": opts.IsRestricted.Value()})
	}

	if opts.IsProhibitLogin.Has() {
		cond = cond.And(builder.Eq{"prohibit_login": opts.IsProhibitLogin.Value()})
	}

	/*** DCS Customizations ***/
	if opts.IsSpamUser.Has() && opts.IsSpamUser.Value() {
		cond = cond.And(builder.Expr("type = 0 AND description != '' AND website != ''"))
	}
	/*** END DCS Customizations ***/

	e := db.GetEngine(ctx)
	if !opts.IsTwoFactorEnabled.Has() {
		return e.Where(cond)
	}

	// 2fa filter uses LEFT JOIN to check whether a user has a 2fa record
	// While using LEFT JOIN, sometimes the performance might not be good, but it won't be a problem now, such SQL is seldom executed.
	// There are some possible methods to refactor this SQL in future when we really need to optimize the performance (but not now):
	// (1) add a column in user table (2) add a setting value in user_setting table (3) use search engines (bleve/elasticsearch)
	if opts.IsTwoFactorEnabled.Value() {
		cond = cond.And(builder.Expr("two_factor.uid IS NOT NULL"))
	} else {
		cond = cond.And(builder.Expr("two_factor.uid IS NULL"))
	}

	return e.Join("LEFT OUTER", "two_factor", "two_factor.uid = `user`.id").
		Where(cond)
}

// SearchUsers takes options i.e. keyword and part of user name to search,
// it returns results in given range and number of total results.
func SearchUsers(ctx context.Context, opts SearchUserOptions) (users []*User, _ int64, _ error) {
	sessCount := opts.toSearchQueryBase(ctx)
	defer sessCount.Close()
	count, err := sessCount.Count(new(User))
	if err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = db.SearchOrderByAlphabetically
	}

	sessQuery := opts.toSearchQueryBase(ctx).OrderBy(opts.OrderBy.String())
	defer sessQuery.Close()
	if opts.Page > 0 {
		sessQuery = db.SetSessionPagination(sessQuery, &opts)
	}

	/*** DCS Customizations ***/
	repoMetadataCond := builder.NewCond()
	hasMetadataCond := false
	if len(opts.RepoLanguages) > 0 {
		hasMetadataCond = true
		repoLangsCond := builder.NewCond()
		for _, values := range opts.RepoLanguages {
			for _, value := range strings.Split(values, ",") {
				repoLangsCond = repoLangsCond.Or(builder.Eq{"`door43_metadata`.language": strings.TrimSpace(value)})
			}
		}
		repoMetadataCond = repoMetadataCond.And(repoLangsCond)
	}
	if len(opts.RepoSubjects) > 0 {
		hasMetadataCond = true
		repoSubsCond := builder.NewCond()
		for _, values := range opts.RepoSubjects {
			for _, value := range strings.Split(values, ",") {
				repoSubsCond = repoSubsCond.Or(builder.Eq{"`door43_metadata`.subject": strings.TrimSpace(value)})
			}
		}
		repoMetadataCond = repoMetadataCond.And(repoSubsCond)
	}
	if len(opts.RepoFlavorTypes) > 0 {
		hasMetadataCond = true
		repoFlavorTypesCond := builder.NewCond()
		for _, values := range opts.RepoFlavorTypes {
			for _, value := range strings.Split(values, ",") {
				repoFlavorTypesCond = repoFlavorTypesCond.Or(builder.Eq{"`door43_metadata`.flavor_type": strings.TrimSpace(value)})
			}
		}
		repoMetadataCond = repoMetadataCond.And(repoFlavorTypesCond)
	}
	if len(opts.RepoFlavors) > 0 {
		hasMetadataCond = true
		repoFlavorCond := builder.NewCond()
		for _, values := range opts.RepoFlavorTypes {
			for _, value := range strings.Split(values, ",") {
				repoFlavorCond = repoFlavorCond.Or(builder.Eq{"`door43_metadata`.flavor": strings.TrimSpace(value)})
			}
		}
		repoMetadataCond = repoMetadataCond.And(repoFlavorCond)
	}
	if len(opts.RepoMetadataTypes) > 0 {
		hasMetadataCond = true
		repoMetadataTypesCond := builder.NewCond()
		for _, values := range opts.RepoMetadataTypes {
			for _, value := range strings.Split(values, ",") {
				repoMetadataTypesCond = repoMetadataTypesCond.Or(builder.Eq{"`door43_metadata`.metadata_type": strings.TrimSpace(value)})
			}
		}
		repoMetadataCond = repoMetadataCond.And(repoMetadataTypesCond)
	}
	if opts.RepoLanguageIsGL.Has() {
		hasMetadataCond = true
		repoMetadataCond = repoMetadataCond.And(builder.Eq{"`door43_metadata`.language_is_gl": opts.RepoLanguageIsGL.Value()})
	}
	if hasMetadataCond {
		metadataSelect := builder.Select("owner_id").
			From("repository").
			Join("INNER", "`door43_metadata`", "repo_id = `repository`.id").
			Where(repoMetadataCond)
		sessQuery.In("`user`.id", metadataSelect)
	}
	/*** END DCS Customizations ***/

	// the sql may contain JOIN, so we must only select User related columns
	sessQuery = sessQuery.Select("`user`.*")
	users = make([]*User, 0, opts.PageSize)
	return users, count, sessQuery.Find(&users)
}

// BuildCanSeeUserCondition creates a condition which can be used to restrict results to users/orgs the actor can see
func BuildCanSeeUserCondition(actor *User) builder.Cond {
	if actor != nil {
		// If Admin - they see all users!
		if !actor.IsAdmin {
			// Users can see an organization they are a member of
			cond := builder.In("`user`.id", builder.Select("org_id").From("org_user").Where(builder.Eq{"uid": actor.ID}))
			if !actor.IsRestricted {
				// Not-Restricted users can see public and limited users/organizations
				cond = cond.Or(builder.In("`user`.visibility", structs.VisibleTypePublic, structs.VisibleTypeLimited))
			}
			// Don't forget about self
			return cond.Or(builder.Eq{"`user`.id": actor.ID})
		}

		return nil
	}

	// Force visibility for privacy
	// Not logged in - only public users
	return builder.In("`user`.visibility", structs.VisibleTypePublic)
}
