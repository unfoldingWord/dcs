// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(repo *models.Repository, mode perm.AccessMode) *api.Repository {
	return innerToRepo(repo, mode, false)
}

func innerToRepo(repo *models.Repository, mode perm.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= perm.AccessModeAdmin,
		Push:  mode >= perm.AccessModeWrite,
		Pull:  mode >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo()
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(repo.BaseRepo, mode, true)
		}
	}

	//check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(unit_model.TypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	defaultMergeStyle := models.MergeStyleMerge
	if unit, err := repo.GetUnit(unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		defaultMergeStyle = config.GetDefaultMergeStyle()
	}
	hasProjects := false
	if _, err := repo.GetUnit(unit_model.TypeProjects); err == nil {
		hasProjects = true
	}

	if err := repo.GetOwner(); err != nil {
		return nil
	}

	numReleases, _ := models.GetReleaseCountByRepoID(repo.ID, models.FindReleasesOptions{IncludeDrafts: false, IncludeTags: false})

	/*** DCS Customizations ***/

	// TODO: Load in Repository's LoadAttributes() function and save to repo.Metadata
	metadata, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, 0)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
	}
	// if metadata == nil {
	// 	metadata, err = repo.GetLatestPreProdCatalogMetadata()
	// 	if err != nil {
	// 		log.Error("GetLatestPreProdCatalogMetadata: %v", err)
	// 	}
	// }

	var language, title, subject, checkingLevel string
	var books []string
	if metadata != nil {
		language = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
		title = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["title"].(string)
		subject = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string)
		books = metadata.GetBooks()
		checkingLevel = (*metadata.Metadata)["checking"].(map[string]interface{})["checking_level"].(string)
	} else {
		language = dcs.GetLanguageFromRepoName(repo.LowerName)
		subject = dcs.GetSubjectFromRepoName(repo.LowerName)
	}
	/*** END DCS Customizations ***/

	mirrorInterval := ""
	if repo.IsMirror {
		if err := repo.GetMirror(); err == nil {
			mirrorInterval = repo.Mirror.Interval.String()
		}
	}

	return &api.Repository{
		ID:                        repo.ID,
		Owner:                     ToUserWithAccessMode(repo.Owner, mode),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Template:                  repo.IsTemplate,
		Empty:                     repo.IsEmpty,
		Archived:                  repo.IsArchived,
		Size:                      int(repo.Size / 1024),
		Fork:                      repo.IsFork,
		Parent:                    parent,
		Mirror:                    repo.IsMirror,
		HTMLURL:                   repo.HTMLURL(),
		SSHURL:                    cloneLink.SSH,
		CloneURL:                  cloneLink.HTTPS,
		OriginalURL:               repo.SanitizedOriginalURL(),
		Website:                   repo.Website,
		Stars:                     repo.NumStars,
		Forks:                     repo.NumForks,
		Watchers:                  repo.NumWatches,
		OpenIssues:                repo.NumOpenIssues,
		OpenPulls:                 repo.NumOpenPulls,
		Releases:                  int(numReleases),
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   hasWiki,
		HasProjects:               hasProjects,
		ExternalWiki:              externalWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		DefaultMergeStyle:         string(defaultMergeStyle),
		AvatarURL:                 repo.AvatarLink(),
		Language:                  language,
		Title:                     title,
		Subject:                   subject,
		Books:                     books,
		CheckingLevel:             checkingLevel,
		Internal:                  !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:            mirrorInterval,
	}
}
