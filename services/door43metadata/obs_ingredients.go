// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package door43metadata

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"gitea.dev/modules/dcs"
	"gitea.dev/modules/git"
	"gitea.dev/modules/structs"
)

const (
	obsStoryCount = 50
	obsBackSort   = obsStoryCount + 1
)

// newOBSIngredient returns the ingredient for the OBS story ("01".."50") or the front or
// back matter that a file or dir name stands for, or nil if it stands for neither. Each is
// a markdown file (NN.md, front.md) or a dir (a tS NN/, an RC content/front/).
func newOBSIngredient(name, ingredientPath string, isDir bool) *structs.Ingredient {
	id, isMarkdown := strings.CutSuffix(name, ".md")
	if isDir == isMarkdown {
		return nil
	}
	ingredient := &structs.Ingredient{Identifier: id, Path: "./" + ingredientPath, IsDir: isDir}
	switch id {
	case "front":
		ingredient.Title = dcs.GetBookName("frt")
	case "back":
		ingredient.Title = dcs.GetBookName("bak")
		ingredient.Sort = obsBackSort
	default:
		num, err := strconv.Atoi(id)
		if err != nil || num < 1 || num > obsStoryCount || fmt.Sprintf("%02d", num) != id {
			return nil
		}
		ingredient.Sort = num
	}
	return ingredient
}

// getOBSTreeIngredients lists the stories and front/back matter found in dir: an RC's
// content dir ("./content") or a tS repo's root (".")
func getOBSTreeIngredients(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, dir string) []*structs.Ingredient {
	if commit == nil {
		return nil
	}
	if dir = path.Clean(dir); dir == "." {
		dir = ""
	}
	tree, err := commit.SubTree(ctx, gitRepo, dir)
	if err != nil {
		return nil
	}
	entries, err := tree.ListEntries(ctx, gitRepo)
	if err != nil {
		return nil
	}
	var ingredients []*structs.Ingredient
	for _, entry := range entries {
		if ingredient := newOBSIngredient(entry.Name(), path.Join(dir, entry.Name()), entry.IsDir()); ingredient != nil {
			setOBSIngredientFromEntry(ctx, gitRepo, commit, ingredient, entry)
			ingredients = append(ingredients, ingredient)
		}
	}
	sortOBSIngredients(ingredients)
	return ingredients
}

// getSBOBSIngredients lists the stories and front/back matter that a burrito's metadata.json
// declares, so a story file that exists but isn't declared is left out, as the healthcheck
// reports. Front/back matter is declared as the files of a front/ or back/ dir (rc2sb's
// ingredients/content/front/intro.md) or as a front.md or back.md file (Scribe's).
func getSBOBSIngredients(ctx context.Context, gitRepo *git.Repository, sbMetadata *dcs.SBMetadata100, commit *git.Commit) []*structs.Ingredient {
	var ingredients []*structs.Ingredient
	seen := map[string]bool{}
	keys := slices.Collect(maps.Keys(sbMetadata.Ingredients))
	for _, repoPath := range dcs.SBIngredientRepoPaths(keys, dcs.HasSBIngredientsDir(ctx, gitRepo, commit)) {
		names := strings.Split(repoPath, "/")
		for i, name := range names {
			ingredient := newOBSIngredient(name, path.Join(names[:i+1]...), i < len(names)-1)
			if ingredient == nil {
				continue
			}
			if !seen[ingredient.Path] {
				seen[ingredient.Path] = true
				if commit != nil {
					if entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, ingredient.Path); err == nil {
						setOBSIngredientFromEntry(ctx, gitRepo, commit, ingredient, entry)
					}
				}
				ingredients = append(ingredients, ingredient)
			}
			break
		}
	}
	sortOBSIngredients(ingredients)
	return ingredients
}

// setOBSIngredientFromEntry fills in what the repo's file or dir says about the ingredient,
// including a story's title, which front/back matter already has
func setOBSIngredientFromEntry(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, ingredient *structs.Ingredient, entry *git.TreeEntry) {
	ingredient.Exists = true
	ingredient.IsDir = entry.IsDir()
	ingredient.Size = entry.GetSize(ctx, gitRepo)
	if ingredient.Title == "" {
		ingredient.Title = readOBSStoryTitle(ctx, gitRepo, commit, ingredient.Path, entry)
	}
}

// readOBSStoryTitle returns the heading that starts a story's NN.md file, or the first line
// of a tS story dir's title.txt
func readOBSStoryTitle(ctx context.Context, gitRepo *git.Repository, commit *git.Commit, storyPath string, entry *git.TreeEntry) string {
	blob := entry.Blob(gitRepo)
	if entry.IsDir() {
		var err error
		if blob, err = commit.GetBlobByPath(ctx, gitRepo, path.Join(storyPath, "title.txt")); err != nil {
			return ""
		}
	}
	content, err := blob.GetBlobContent(ctx, 1024)
	if err != nil {
		return ""
	}
	return parseOBSStoryTitle(content, !entry.IsDir())
}

// parseOBSStoryTitle returns the title on the first line of content, which for markdown must
// be a heading
func parseOBSStoryTitle(content string, isMarkdown bool) string {
	line, _, _ := strings.Cut(strings.TrimPrefix(content, "\ufeff"), "\n")
	if isMarkdown && !strings.HasPrefix(line, "#") {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(line, "#"))
}

func sortOBSIngredients(ingredients []*structs.Ingredient) {
	slices.SortFunc(ingredients, func(a, b *structs.Ingredient) int {
		return cmp.Or(cmp.Compare(a.Sort, b.Sort), strings.Compare(a.Path, b.Path))
	})
}
