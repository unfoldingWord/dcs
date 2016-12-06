package scrub_test

import (
	"testing"
	"os"
	"io"
	"fmt"
	"path"
	"path/filepath"
	"io/ioutil"

	. "github.com/smartystreets/goconvey/convey"
	"code.gitea.io/gitea/modules/scrub"
	"code.gitea.io/git"
)

// The repos to be tested and the "success" result that should be expected
var TESTING_REPOS = map[string]bool{
	"all_json_files": true,
	"bad_json_file": false,
	"multiple_sensative_fields": true,
	"no_json_files": false,
	"no_sensative_data": true,
}

func TestScrubJsonFiles(t *testing.T) {
	myDir, _ := os.Getwd()
	testFilesDir := path.Join(myDir, "scrub_test_files")
	tempDir, _ := ioutil.TempDir(os.TempDir(), "scrub_test")
	for repoName, expectedResult := range TESTING_REPOS {
		repoDir := path.Join(tempDir, repoName)
		CopyDir(path.Join(testFilesDir, repoName),repoDir)
		git.InitRepository(repoDir, false)
		git.AddChanges(repoDir, true)
		git.CommitChanges(repoDir, git.CommitChangesOptions{
			Message: "Initial Commit",
		})
		Convey(fmt.Sprintf("The repo %s should return %v", repoName, expectedResult), t, func() {
			So(scrub.ScrubJsonFiles(repoDir), ShouldEqual, expectedResult)
		})
	}
}

// Below code to copy directories was retreived by Richard Mahn from https://gist.github.com/m4ng0squ4sh/92462b38df26839a3ca324697c8cba04

// CopyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file. The file mode will be copied from the source and
// the copied data is synced/flushed to stable storage.
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
// Symlinks are ignored and skipped.
func CopyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	fmt.Println(src)
	fmt.Println(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}
