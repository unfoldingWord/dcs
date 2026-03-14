// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convertrc2sb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
)

// qualifyingTopics are the repo topics that mark a repo for RC-to-SB conversion.
var qualifyingTopics = []string{"tc-create", "tc-ready", "pushing2sb", "rc2sb"}

// RepoQualifiesForConversion checks if a repo meets all criteria for RC-to-SB conversion:
// 1. DefaultBranch DM MetadataType is "rc"
// 2. DefaultBranch is "master"
// 3. Repo has at least one qualifying topic
func RepoQualifiesForConversion(ctx context.Context, repo *repo_model.Repository) (bool, error) {
	if repo == nil {
		return false, nil
	}

	if repo.DefaultBranch != "master" {
		log.Debug("ConvertRC2SB: %s does not qualify — DefaultBranch is %q (need \"master\")", repo.FullName(), repo.DefaultBranch)
		return false, nil
	}

	if !repoHasQualifyingTopic(repo) {
		log.Debug("ConvertRC2SB: %s does not qualify — no qualifying topic (has: %v)", repo.FullName(), repo.Topics)
		return false, nil
	}

	if err := repo.LoadLatestDMs(ctx); err != nil {
		return false, fmt.Errorf("LoadLatestDMs: %w", err)
	}

	dm := repo.DefaultBranchDM
	if dm == nil {
		dm = repo.RepoDM
	}
	if dm == nil {
		log.Debug("ConvertRC2SB: %s does not qualify — no Door43Metadata found", repo.FullName())
		return false, nil
	}
	if dm.MetadataType != "rc" {
		log.Debug("ConvertRC2SB: %s does not qualify — MetadataType is %q (need \"rc\")", repo.FullName(), dm.MetadataType)
		return false, nil
	}

	log.Info("ConvertRC2SB: %s qualifies for conversion", repo.FullName())
	return true, nil
}

// repoHasQualifyingTopic checks if repo.Topics contains at least one qualifying topic.
func repoHasQualifyingTopic(repo *repo_model.Repository) bool {
	for _, topic := range repo.Topics {
		for _, qt := range qualifyingTopics {
			if strings.EqualFold(topic, qt) {
				return true
			}
		}
	}
	return false
}

// ForRelease converts an RC repo at the given release tag to SB format
// and pushes the result to the "main" branch.
func ForRelease(ctx context.Context, repo *repo_model.Repository, release *repo_model.Release) error {
	if repo == nil || release == nil {
		return errors.New("repo and release must not be nil")
	}

	log.Info("ConvertRC2SB: starting conversion for %s tag %s", repo.FullName(), release.TagName)

	// Load owner for commit identity
	if err := repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("LoadOwner: %w", err)
	}

	// Create temp directory for conversion work
	tmpDir, cleanup, err := setting.AppDataTempDir("repo-rc2sb-convert").MkdirTempRandom(fmt.Sprintf("%d-", repo.ID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer cleanup()

	// Step 1: Shallow clone at the release tag
	rcDir := filepath.Join(tmpDir, "rc")
	if err := cloneAtTag(ctx, repo.RepoPath(), release.TagName, rcDir); err != nil {
		return fmt.Errorf("clone at tag %s: %w", release.TagName, err)
	}

	// Step 2: Prepare payload path for TWL repos (same logic as sbarchiver)
	payloadPath, err := preparePayloadPath(ctx, tmpDir, repo)
	if err != nil {
		return fmt.Errorf("preparePayloadPath: %w", err)
	}

	// Step 3: Run rc2sb conversion
	sbDir := filepath.Join(tmpDir, "sb")
	result, err := rc2sb.Convert(ctx, rcDir, sbDir, rc2sb.Options{
		PayloadPath: payloadPath,
	})
	if err != nil {
		return fmt.Errorf("rc2sb.Convert: %w", err)
	}
	log.Info("ConvertRC2SB: conversion successful for %s — subject=%s, ingredients=%d",
		repo.FullName(), result.Subject, result.Ingredients)

	// Step 4: Clone the full repo to a working directory for pushing
	workDir := filepath.Join(tmpDir, "work")
	if err := git.Clone(ctx, repo.RepoPath(), workDir, git.CloneRepoOptions{Quiet: true}); err != nil {
		return fmt.Errorf("clone work dir: %w", err)
	}

	// Step 5: Create or checkout the "main" branch
	if err := prepareMainBranch(ctx, workDir, release.TagName); err != nil {
		return fmt.Errorf("prepareMainBranch: %w", err)
	}

	// Step 6: Remove all files in working tree (except .git)
	if err := clearWorkingTree(workDir); err != nil {
		return fmt.Errorf("clearWorkingTree: %w", err)
	}

	// Step 7: Copy converted SB files into working tree
	if err := copyDir(sbDir, workDir); err != nil {
		return fmt.Errorf("copy SB files: %w", err)
	}

	// Step 8: Stage all changes
	if _, _, err := gitcmd.NewCommand("add", "-A").RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir}); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are any changes to commit
	stdout, _, err := gitcmd.NewCommand("status", "--porcelain").RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir})
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(stdout) == "" {
		log.Info("ConvertRC2SB: no changes to commit for %s tag %s", repo.FullName(), release.TagName)
		return nil
	}

	// Step 9: Commit
	commitMsg := "Convert RC to SB from tag " + release.TagName
	doer := repo.Owner
	sig := doer.NewGitSig()

	_, _, err = gitcmd.NewCommand("commit",
		"-m").AddDynamicArguments(commitMsg).
		AddArguments("--author").AddDynamicArguments(fmt.Sprintf("%s <%s>", sig.Name, sig.Email)).
		RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir})
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Step 10: Push to the bare repo
	// Use PushingEnvironment (not InternalPushingEnvironment) so hooks fire
	// and Door43Metadata is processed for the new main branch
	env := repo_module.PushingEnvironment(doer, repo)
	if err := git.Push(ctx, workDir, git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: "main",
		Env:    env,
	}); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	log.Info("ConvertRC2SB: successfully pushed SB content to main branch for %s", repo.FullName())
	return nil
}

// ConvertRC2SBAllRepos finds all qualifying repos and converts their latest published release.
func ConvertRC2SBAllRepos(ctx context.Context) error { //nolint:revive // name is used by cron task reference
	log.Trace("Doing: ConvertRC2SBAllRepos")

	repos, err := repo_model.GetReposForMetadata(ctx)
	if err != nil {
		return fmt.Errorf("GetReposForMetadata: %w", err)
	}

	for _, repo := range repos {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		qualifies, err := RepoQualifiesForConversion(ctx, repo)
		if err != nil {
			log.Warn("ConvertRC2SBAllRepos: error checking qualification for %s: %v", repo.FullName(), err)
			continue
		}
		if !qualifies {
			continue
		}

		// Find latest published (non-draft, non-prerelease) release
		release, err := getLatestPublishedRelease(ctx, repo)
		if err != nil {
			log.Warn("ConvertRC2SBAllRepos: error getting latest release for %s: %v", repo.FullName(), err)
			continue
		}
		if release == nil {
			log.Debug("ConvertRC2SBAllRepos: no published release for %s, skipping", repo.FullName())
			continue
		}

		if err := ForRelease(ctx, repo, release); err != nil {
			log.Error("ConvertRC2SBAllRepos: conversion failed for %s tag %s: %v", repo.FullName(), release.TagName, err)
			if noticeErr := system_model.CreateRepositoryNotice(
				"ConvertRC2SB failed for repository (%s) tag (%s): %v", repo.FullName(), release.TagName, err,
			); noticeErr != nil {
				log.Error("CreateRepositoryNotice: %v", noticeErr)
			}
			continue
		}
	}

	log.Trace("Finished: ConvertRC2SBAllRepos")
	return nil
}

// getLatestPublishedRelease returns the most recent non-draft, non-prerelease release for a repo.
func getLatestPublishedRelease(ctx context.Context, repo *repo_model.Repository) (*repo_model.Release, error) {
	rel, err := repo_model.GetLatestReleaseByRepoID(ctx, repo.ID, false, optional.None[bool]())
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return rel, nil
}

// cloneAtTag does a shallow clone of the repo at the specified tag.
func cloneAtTag(ctx context.Context, repoPath, tagName, destination string) error {
	err := git.Clone(ctx, repoPath, destination, git.CloneRepoOptions{
		Quiet:  true,
		Depth:  1,
		Branch: tagName,
	})
	if err != nil {
		// Fallback to full clone if shallow clone fails
		if removeErr := util.RemoveAll(destination); removeErr != nil {
			log.Warn("cloneAtTag: failed to clean shallow clone destination: %v", removeErr)
		}
		if err := git.Clone(ctx, repoPath, destination, git.CloneRepoOptions{Quiet: true}); err != nil {
			return err
		}
		// Checkout the tag
		_, _, checkoutErr := gitcmd.NewCommand("checkout").AddDynamicArguments(tagName).
			RunStdString(ctx, &gitcmd.RunOpts{Dir: destination})
		if checkoutErr != nil {
			return fmt.Errorf("checkout tag %s: %w", tagName, checkoutErr)
		}
	}
	return nil
}

// prepareMainBranch creates or checks out the "main" branch in the working directory.
func prepareMainBranch(ctx context.Context, workDir, tagName string) error {
	// Check if main branch exists
	_, _, err := gitcmd.NewCommand("rev-parse", "--verify", "refs/heads/main").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir})
	if err != nil {
		// main doesn't exist — create it from the tag
		_, _, err = gitcmd.NewCommand("checkout", "-b", "main").AddDynamicArguments(tagName).
			RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir})
		if err != nil {
			return fmt.Errorf("create main branch from tag %s: %w", tagName, err)
		}
		return nil
	}

	// main exists — check it out
	_, _, err = gitcmd.NewCommand("checkout", "main").
		RunStdString(ctx, &gitcmd.RunOpts{Dir: workDir})
	if err != nil {
		return fmt.Errorf("checkout main: %w", err)
	}
	return nil
}

// clearWorkingTree removes all files and directories in the working tree except .git.
func clearWorkingTree(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyDir recursively copies src directory contents into dst directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// preparePayloadPath prepares a TW payload directory for TWL repos (same logic as sbarchiver).
func preparePayloadPath(ctx context.Context, tmpDir string, repo *repo_model.Repository) (string, error) {
	if err := repo.LoadLatestDMs(ctx); err != nil {
		return "", nil
	}

	dm := repo.DefaultBranchDM
	if dm == nil {
		dm = repo.RepoDM
	}
	if dm == nil || dm.Subject != "TSV Translation Words Links" || dm.Language == "" {
		return "", nil
	}

	twRepoName := dm.Language + "_tw"
	twRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, twRepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("GetRepositoryByOwnerAndName(%s): %w", twRepoName, err)
	}

	twGitRepo, err := gitrepo.OpenRepository(ctx, twRepo)
	if err != nil {
		return "", fmt.Errorf("OpenRepository(%s): %w", twRepo.FullName(), err)
	}
	defer twGitRepo.Close()

	commitID, err := twGitRepo.ConvertToGitID(twRepo.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("resolve TW default branch: %w", err)
	}

	payloadDir := filepath.Join(tmpDir, "payload")
	if err := cloneAtTag(ctx, twRepo.RepoPath(), twRepo.DefaultBranch, payloadDir); err != nil {
		_ = util.RemoveAll(payloadDir)
		// Full clone fallback
		if err := git.Clone(ctx, twRepo.RepoPath(), payloadDir, git.CloneRepoOptions{Quiet: true}); err != nil {
			return "", fmt.Errorf("clone TW payload: %w", err)
		}
		_, _, checkoutErr := gitcmd.NewCommand("checkout", "--detach").AddDynamicArguments(commitID.String()).
			RunStdString(ctx, &gitcmd.RunOpts{Dir: payloadDir})
		if checkoutErr != nil {
			return "", fmt.Errorf("checkout TW commit: %w", checkoutErr)
		}
	}

	return payloadDir, nil
}
