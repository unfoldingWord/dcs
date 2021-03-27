// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(repo *models.Repository, mode models.AccessMode) *api.Repository {
	return innerToRepo(repo, mode, false)
}

func innerToRepo(repo *models.Repository, mode models.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= models.AccessModeAdmin,
		Push:  mode >= models.AccessModeWrite,
		Pull:  mode >= models.AccessModeRead,
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
	if unit, err := repo.GetUnit(models.UnitTypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalTracker); err == nil {
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
	if _, err := repo.GetUnit(models.UnitTypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalWiki); err == nil {
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
	if unit, err := repo.GetUnit(models.UnitTypePullRequests); err == nil {
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
	if _, err := repo.GetUnit(models.UnitTypeProjects); err == nil {
		hasProjects = true
	}

	if err := repo.GetOwner(); err != nil {
		return nil
	}

	numReleases, _ := models.GetReleaseCountByRepoID(repo.ID, models.FindReleasesOptions{IncludeDrafts: false, IncludeTags: true})

	/*** DCS Customizations ***/
	catalog := &api.CatalogStages{}
	prod, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageProd)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	preprod, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StagePreProd)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	draft, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageDraft)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}
	latest, err := models.GetDoor43MetadataByRepoIDAndStage(repo.ID, models.StageLatest)
	if err != nil {
		log.Error("GetDoor43MetadataByRepoIDAndStage: %v", err)
	}

	if draft != nil && ((prod != nil && prod.ReleaseDateUnix >= draft.ReleaseDateUnix) ||
		(preprod != nil && preprod.ReleaseDateUnix >= draft.ReleaseDateUnix)) {
		draft = nil
	}
	if prod != nil && preprod != nil && prod.ReleaseDateUnix >= preprod.ReleaseDateUnix {
		preprod = nil
	}
	if prod != nil {
		prod.Repo = repo
		url := prod.GetReleaseURL()
		catalog.Production = &api.CatalogStage{
			Tag:        prod.BranchOrTag,
			ReleaseURL: &url,
			Released:   prod.GetReleaseDateTime(),
			ZipballURL: prod.GetZipballURL(),
			TarballURL: prod.GetTarballURL(),
		}
	}
	if preprod != nil {
		preprod.Repo = repo
		url := preprod.GetReleaseURL()
		catalog.PreProduction = &api.CatalogStage{
			Tag:        preprod.BranchOrTag,
			ReleaseURL: &url,
			Released:   preprod.GetReleaseDateTime(),
			ZipballURL: preprod.GetZipballURL(),
			TarballURL: preprod.GetTarballURL(),
		}
	}
	if draft != nil {
		draft.Repo = repo
		url := draft.GetReleaseURL()
		catalog.Draft = &api.CatalogStage{
			Tag:        draft.BranchOrTag,
			ReleaseURL: &url,
			Released:   draft.GetReleaseDateTime(),
			ZipballURL: draft.GetZipballURL(),
			TarballURL: draft.GetTarballURL(),
		}
	}
	if latest != nil {
		latest.Repo = repo
		catalog.Latest = &api.CatalogStage{
			Tag:        latest.BranchOrTag,
			ReleaseURL: nil,
			Released:   latest.GetReleaseDateTime(),
			ZipballURL: latest.GetZipballURL(),
			TarballURL: latest.GetTarballURL(),
		}
	}

	metadata, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, 0)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
	}
	if metadata == nil {
		metadata, err = repo.GetLatestPreProdCatalogMetadata()
		if err != nil {
			log.Error("GetLatestPreProdCatalogMetadata: %v", err)
		}
	}

	var language, title, subject, checkingLevel string
	var books []string
	if metadata != nil {
		language = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string)
		title = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["title"].(string)
		subject = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string)
		books = metadata.GetBooks()
		checkingLevel = (*metadata.Metadata)["checking"].(map[string]interface{})["checking_level"].(string)
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
		Catalog:                   catalog,
		Internal:                  !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:            mirrorInterval,
	}
}
