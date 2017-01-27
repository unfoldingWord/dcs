package models

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
	_ "github.com/mattn/go-sqlite3"
	"github.com/go-xorm/xorm"
	"io/ioutil"
	"os"
	"path"
	"fmt"
	"github.com/go-xorm/core"
)

func TestHashtag(t *testing.T) {

	userSql := `INSERT INTO user (id, lower_name, name, full_name, email, passwd, login_type, login_source, login_name, type, location, website, rands, salt, created_unix, updated_unix, last_login_unix, last_repo_visibility, max_repo_creation, is_active, is_admin, allow_git_hook, allow_import_local, prohibit_login, avatar, avatar_email, use_custom_avatar, num_followers, num_following, num_stars, num_repos, description, num_teams, num_members, diff_view_style)
VALUES ('1', 'phil', 'phil', '', 'phil@email.org', 'c024408767b2b15755e65c6799154c38ea9975c4a0d4642f47981c7169e76ca6cc507b4b8587efa2e656e0afd65017951bbd', '0', '0', '', '0', '', '', 'nekqlODpS1', 'fjsVRQvR0M', '1479588664', '1480428210', '1480428191', 'false', '-1', 'true', 'true', 'false', 'false', 'false', 'c1f199e0525420aceca0859cd4c9f990', 'phil@email.org', 'false', '0', '0', '0', '1', '', '0', '0', ''),
       ('2', 'rich', 'rich', '', 'rich@email.org', 'c024408767b2b15755e65c6799154c38ea9975c4a0d4642f47981c7169e76ca6cc507b4b8587efa2e656e0afd65017951bbd', '0', '0', '', '0', '', '', 'nekqlODpS1', 'fjsVRQvR0M', '1479588664', '1480428210', '1480428191', 'false', '-1', 'true', 'true', 'false', 'false', 'false', 'c1f199e0525420aceca0859cd4c9f990', 'rich@email.org', 'false', '0', '0', '0', '1', '', '0', '0', ''),
       ('3', 'bruce', 'bruce', '', 'bruce@email.org', 'c024408767b2b15755e65c6799154c38ea9975c4a0d4642f47981c7169e76ca6cc507b4b8587efa2e656e0afd65017951bbd', '0', '0', '', '0', '', '', 'nekqlODpS1', 'fjsVRQvR0M', '1479588664', '1480428210', '1480428191', 'false', '-1', 'true', 'true', 'false', 'false', 'false', 'c1f199e0525420aceca0859cd4c9f990', 'bruce@email.org', 'false', '0', '0', '0', '1', '', '0', '0', '')`

	repoSql := `INSERT INTO repository (id, owner_id, lower_name, name, description, website, default_branch, num_watches, num_stars, num_forks, num_issues, num_closed_issues, num_pulls, num_closed_pulls, num_milestones, num_closed_milestones, is_private, is_bare, is_mirror, enable_wiki, enable_external_wiki, external_wiki_url, enable_issues, enable_external_tracker, external_tracker_url, external_tracker_format, external_tracker_style, enable_pulls, is_fork, fork_id, created_unix, updated_unix)
VALUES ('1', '1', 'en-ubn-tags', 'en-ubn-tags', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1479589835', '1479731504'),
       ('2', '1', 'en-ubn', 'en-ubn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426137', '1480426138'),
       ('3', '1', 'en-tw', 'en-tw', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426161', '1480426161'),
       ('4', '1', 'en-ubnn', 'en-ubnn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426633', '1480426633'),
       ('5', '1', 'en-ub-test', 'en-ub-test', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480428210', '1480428210'),
       ('6', '2', 'en-ubn-tags', 'en-ubn-tags', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1479589835', '1479731504'),
       ('7', '2', 'en-ubn', 'en-ubn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426137', '1480426138'),
       ('8', '2', 'en-tw', 'en-tw', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426161', '1480426161'),
       ('9', '2', 'en-ubnn', 'en-ubnn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426633', '1480426633'),
       ('10', '2', 'en-ub-test', 'en-ub-test', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480428210', '1480428210'),
       ('11', '3', 'en-ubn-tags', 'en-ubn-tags', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1479589835', '1479731504'),
       ('12', '3', 'en-ubn', 'en-ubn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426137', '1480426138'),
       ('13', '3', 'en-tw', 'en-tw', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426161', '1480426161'),
       ('14', '3', 'en-ubnn', 'en-ubnn', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480426633', '1480426633'),
       ('15', '3', 'en-ub-test', 'en-ub-test', '', '', 'master', '1', '0', '0', '0', '0', '0', '0', '0', '0', 'false', 'false', 'false', 'true', 'false', '', 'true', 'false', '', '', 'numeric', 'true', 'false', '0', '1480428210', '1480428210')`

	hashtagSql := `INSERT INTO hashtag (id, user_id, repo_id, lang, tag_name, file_path, created_unix, updated_unix)
VALUES ('1','1','1','en','amill','article1.md','1480336882','1480336882'),
       ('2','1','2','en','amill','article2.md','1480336882','1480336882'),
       ('3','1','2','en','amill','article3.md','1480336882','1480336882'),
       ('4','1','1','en','salv','article1.md','1480336882','1480336882'),
       ('5','1','2','en','salv','article3.md','1480336882','1480336882'),
       ('6','1','2','en','m-asiaminor','article2.md','1480336882','1480336882'),
       ('7','1','2','en','m-endofearth','article3.md','1480336882','1480336882'),
       ('8','1','1','en','m-galilee','article1.md','1480336882','1480336882'),
       ('9','1','2','en','m-greece','article3.md','1480336882','1480336882'),
       ('10','1','2','en','xq-gen12:3','article2.md','1480336882','1480336882'),
       ('11','1','2','en','xa-isa40:3','article3.md','1480336882','1480336882'),
       ('12','1','1','en','xa-jer12:1','article1.md','1480336882','1480336882'),
       ('13','1','2','en','A1933','article3.md','1480336882','1480336882'),
       ('14','1','2','en','G58','article3.md','1480336882','1480336882'),
       ('15','1','2','en','H305','article3.md','1480336882','1480336882'),
       ('16','1','2','en','da-hsbaptism','article2.md','1480336882','1480336882'),
       ('17','1','2','en','da-kingdomofgod','article3.md','1480336882','1480336882'),
       ('18','1','1','en','da-modeofbaptism','article1.md','1480336882','1480336882'),
       ('19','1','2','en','da-ntuseofot','article3.md','1480336882','1480336882'),
       ('20','1','2','en','da-spiritualgifts','article3.md','1480336882','1480336882'),
       ('21','1','2','en','da-waterbaptism','article3.md','1480336882','1480336882'),
       ('22','1','2','en','dg-brother','article2.md','1480336882','1480336882'),
       ('23','1','2','en','dg-pentecost','article3.md','1480336882','1480336882'),
       ('24','1','1','en','bapt','article1.md','1480336882','1480336882'),
       ('25','1','2','en','episangl','article3.md','1480336882','1480336882'),
       ('26','1','2','en','jew','article3.md','1480336882','1480336882'),
       ('27','1','2','en','luth','article3.md','1480336882','1480336882'),
       ('28','2','6','en','amill','article1.md','1480336882','1480336882'),
       ('29','2','7','en','amill','article2.md','1480336882','1480336882'),
       ('30','2','7','en','amill','article3.md','1480336882','1480336882'),
       ('31','2','6','en','salv','article1.md','1480336882','1480336882'),
       ('32','2','7','en','salv','article3.md','1480336882','1480336882'),
       ('33','2','7','en','m-asiaminor','article2.md','1480336882','1480336882'),
       ('34','2','7','en','m-endofearth','article3.md','1480336882','1480336882'),
       ('35','2','6','en','m-galilee','article1.md','1480336882','1480336882'),
       ('36','2','7','en','m-greece','article3.md','1480336882','1480336882'),
       ('37','2','7','en','xq-gen12:3','article2.md','1480336882','1480336882'),
       ('38','2','7','en','xa-isa40:3','article3.md','1480336882','1480336882'),
       ('39','2','6','en','xa-jer12:1','article1.md','1480336882','1480336882'),
       ('40','2','7','en','A1933','article3.md','1480336882','1480336882'),
       ('41','2','7','en','G58','article3.md','1480336882','1480336882'),
       ('42','2','7','en','H305','article3.md','1480336882','1480336882'),
       ('43','2','7','en','da-hsbaptism','article2.md','1480336882','1480336882'),
       ('44','2','7','en','da-kingdomofgod','article3.md','1480336882','1480336882'),
       ('45','2','6','en','da-modeofbaptism','article1.md','1480336882','1480336882'),
       ('46','2','7','en','da-ntuseofot','article3.md','1480336882','1480336882'),
       ('47','2','7','en','da-spiritualgifts','article3.md','1480336882','1480336882'),
       ('48','2','7','en','da-waterbaptism','article3.md','1480336882','1480336882'),
       ('49','2','7','en','dg-brother','article2.md','1480336882','1480336882'),
       ('50','2','7','en','dg-pentecost','article3.md','1480336882','1480336882'),
       ('51','2','6','en','bapt','article1.md','1480336882','1480336882'),
       ('52','2','7','en','episangl','article3.md','1480336882','1480336882'),
       ('53','2','7','en','jew','article3.md','1480336882','1480336882'),
       ('54','2','7','en','luth','article3.md','1480336882','1480336882')`

	statements := []string{userSql, repoSql, hashtagSql}

	// **** setting up the testing sqlite database ****
	var testEngine *xorm.Engine

	// 1. create temp directory
	tempDir, err := ioutil.TempDir(os.TempDir(), "_hashtag_test_")
	if err != nil {
		print(fmt.Sprintf("Temp dir error: %v", err))
	}
	dbFile := path.Join(tempDir, "hashtag_test.sql")
	os.MkdirAll(path.Dir(dbFile), os.ModePerm)

	// 2. start the engine
	testEngine, err2 := xorm.NewEngine("sqlite3", dbFile)
	if err2 != nil {
		print(fmt.Sprintf("Engine error: %v", err2) + "\n")
	}
	testEngine.SetMapper(core.GonicMapper{})
	testEngine.StoreEngine("InnoDB").Sync2(tables...)

	// 3. insert test data
	for _, sql := range statements {
		_, err = testEngine.Exec(sql)
		if err != nil {
			print(fmt.Sprintf("Temp dir error: %v", err))
		}
	}

	// **** remove the temp directory when the class is disposed ****
	defer os.RemoveAll(tempDir)

	Convey("Test Hashtag.GetHashtagSummary()", t, func() {
		results, err := getHashtagSummary(testEngine, "en-ubn", 1)
		So(err, ShouldBeNil)

		//print("Hashtags found: " + strconv.Itoa(len(results)) + "\n")
		So(len(results), ShouldEqual, 24)

		for _, hashtag := range results {
			//print(fmt.Sprintf("%v: %v", hashtag["tag_name"], hashtag["count_of_occurrences"]) + "\n")
			So(hashtag["tag_name"], ShouldNotBeEmpty)
		}

		// check for correct count
		var tag = results[0]  // A1933
		So(tag["count_of_occurrences"], ShouldEqual, "1")

		tag = results[1]      // amill
		So(tag["count_of_occurrences"], ShouldEqual, "3")
	})
}
