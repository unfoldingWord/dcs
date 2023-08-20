// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/util"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v2"

	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader" // Loader for Scheuma via HTTP
)

// ValidateYAMLFile validates a yaml file
func ValidateYAMLFile(entry *git.TreeEntry) string {
	if _, err := ReadYAMLFromBlob(entry.Blob()); err != nil {
		return strings.ReplaceAll(err.Error(), " converting YAML to JSON", "")
	}
	return ""
}

// ValidateJSONFile validates a json file
func ValidateJSONFile(entry *git.TreeEntry) string {
	if err := ValidateJSONFromBlob(entry.Blob()); err != nil {
		log.Warn("Error decoding JSON file %s: %v\n", entry.Name(), err)
		return fmt.Sprintf("Error reading JSON file %s: %s\n", entry.Name(), err.Error())
	}
	return ""
}

// StringHasSuffix returns bool if str ends in the suffix
func StringHasSuffix(str, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

// ValidateManifestFileAsHTML validates a manifest file and returns the results as template.HTML
func ValidateManifestFileAsHTML(entry *git.TreeEntry) template.HTML {
	var result *jsonschema.ValidationError
	if r, err := ValidateManifestTreeEntry(entry); err != nil {
		log.Warn("ValidateManifestTreeEntry: %v\n", err)
	} else {
		result = r
	}
	return template.HTML(ConvertValidationErrorToHTML(result))
}

// ValidateManifestTreeEntry validates a tree entry that is a manifest file and returns the results
func ValidateManifestTreeEntry(entry *git.TreeEntry) (*jsonschema.ValidationError, error) {
	if entry == nil {
		return nil, nil
	}
	manifest, err := ReadYAMLFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	return ValidateMapByRC02Schema(manifest)
}

// ValidateMetadataFileAsHTML validates a metadata file and returns the results as template.HTML
func ValidateMetadataFileAsHTML(entry *git.TreeEntry) template.HTML {
	var result *jsonschema.ValidationError
	if r, err := ValidateMetadataTreeEntry(entry); err != nil {
		log.Warn("ValidateManifestTreeEntry: %v\n", err)
	} else {
		result = r
	}
	return template.HTML(ConvertValidationErrorToHTML(result))
}

// ValidateMetadataTreeEntry validates a tree entry that is a metadata file and returns the results
func ValidateMetadataTreeEntry(entry *git.TreeEntry) (*jsonschema.ValidationError, error) {
	if entry == nil {
		return nil, nil
	}
	metadata, err := ReadJSONFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	return ValidateMapBySB100Schema(metadata)
}

// ConvertValidationErrorToString returns a semi-colon & new line separated string of the validation errors
func ConvertValidationErrorToString(valErr *jsonschema.ValidationError) string {
	return convertValidationErrorToString(valErr, nil, "")
}

func convertValidationErrorToString(valErr, parentErr *jsonschema.ValidationError, padding string) string {
	if valErr == nil {
		return ""
	}
	str := padding
	if parentErr == nil {
		str += fmt.Sprintf("Invalid: %s\n", strings.TrimSuffix(valErr.Message, "#"))
		if len(valErr.Causes) > 0 {
			str += "* <root>:\n"
		}
	} else {
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = fmt.Sprintf("%s: ", strings.TrimPrefix(loc, "/"))
			}
		}
		str += fmt.Sprintf("* %s%s\n", loc, valErr.Message)
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	for _, cause := range valErr.Causes {
		str += convertValidationErrorToString(cause, valErr, padding+"  ")
	}
	return str
}

// ConvertValidationErrorToHTML converts a validation error object to an HTML string
func ConvertValidationErrorToHTML(valErr *jsonschema.ValidationError) string {
	return convertValidationErrorToHTML(valErr, nil)
}

func convertValidationErrorToHTML(valErr, parentErr *jsonschema.ValidationError) string {
	if valErr == nil {
		return ""
	}
	var label string
	var html string
	if parentErr == nil {
		html = fmt.Sprintf("<strong>Invalid:</strong> %s\n", strings.TrimSuffix(valErr.Message, "#"))
		html += "<ul>\n"
		if len(valErr.Causes) > 0 {
			label += "<strong>&lt;root&gt;:</strong>\n"
		}
	} else {
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = fmt.Sprintf("<strong>%s:</strong> ", strings.TrimPrefix(loc, "/"))
			}
		}
		msg := ""
		if valErr.Message != "if-else failed" && valErr.Message != "if-then failed" {
			msg = valErr.Message
		}
		label = loc + msg
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	if label != "" {
		html += "<ul><li>" + label + "</li>"
	}
	for _, cause := range valErr.Causes {
		html += convertValidationErrorToHTML(cause, valErr)
	}
	if label != "" {
		html += "</ul>\n"
	}
	return html
}

// ValidateMapBySB100Schema Validates a map structure by the RC v0.2.0 schema and returns the result
func ValidateMapBySB100Schema(data *map[string]interface{}) (*jsonschema.ValidationError, error) {
	if data == nil {
		return &jsonschema.ValidationError{Message: "file cannot be empty"}, nil
	}
	schema, err := GetSB100Schema(false)
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(*data); err != nil {
		switch e := err.(type) {
		case *jsonschema.ValidationError:
			return e, nil
		default:
			return nil, e
		}
	}
	return nil, nil
}

// ValidateMapByRC02Schema Validates a map structure by the RC v0.2.0 schema and returns the result
func ValidateMapByRC02Schema(data *map[string]interface{}) (*jsonschema.ValidationError, error) {
	if data == nil {
		return &jsonschema.ValidationError{Message: "file cannot be empty"}, nil
	}
	schema, err := GetRC02Schema(false)
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(*data); err != nil {
		switch e := err.(type) {
		case *jsonschema.ValidationError:
			return e, nil
		default:
			return nil, e
		}
	}
	return nil, nil
}

// ToStringKeys takes an interface and change it to map[string]interface{} on all levels
func ToStringKeys(val interface{}) (interface{}, error) {
	var err error
	switch val := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return nil, errors.New("found non-string key")
			}
			m[k], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case map[string]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			m[k], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []interface{}:
		l := make([]interface{}, len(val))
		for i, v := range val {
			l[i], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return l, nil
	default:
		return val, nil
	}
}

var (
	rc02Schema  *jsonschema.Schema
	sb100Schema *jsonschema.Schema
)

// GetRC02Schema returns the schema for RC v0.2
func GetRC02Schema(reload bool) (*jsonschema.Schema, error) {
	githubPrefix := "https://raw.githubusercontent.com/unfoldingWord/rc-schema/master/"
	if rc02Schema == nil || reload {
		jsonschema.Loaders["https"] = func(url string) (io.ReadCloser, error) {
			res, err := http.Get(url)
			if err == nil && res != nil && res.StatusCode == 200 {
				return res.Body, nil
			}
			log.Warn("GetRC02Schema: not able to get the schema file remotely [%q]: %v", url, err)
			uriPath := strings.TrimPrefix(url, githubPrefix)
			fileBuf, err := options.AssetFS().ReadFile("schema", "rc02", uriPath)
			if err != nil {
				log.Error("GetRC02Schema: local schema file not found: [options/schema/rc02/%s]: %v", uriPath, err)
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(fileBuf)), nil
		}
		var err error
		rc02Schema, err = jsonschema.Compile(githubPrefix + "rc.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return rc02Schema, nil
}

// GetSB100Schema returns the schema for SB v1.0.0
func GetSB100Schema(reload bool) (*jsonschema.Schema, error) {
	// We must use githubURLPrefix due to certificate issues
	burritoBiblePrefix := "https://burrito.bible/schema/"
	githubPrefix := "https://raw.githubusercontent.com/bible-technology/scripture-burrito/v1.0.0/schema/"
	if sb100Schema == nil || reload {
		jsonschema.Loaders["https"] = func(url string) (io.ReadCloser, error) {
			uriPath := strings.TrimPrefix(url, burritoBiblePrefix)
			githubURL := githubPrefix + uriPath
			res, err := http.Get(githubURL)
			if err == nil && res != nil && res.StatusCode == 200 {
				return res.Body, nil
			}
			log.Error("GetSB100Schema: not able to get the schema file remotely [%q]: %v", url, err)
			fileBuf, err := options.AssetFS().ReadFile("schema", "sb100", uriPath)
			if err != nil {
				log.Error("GetSB100Schema: local schema file not found: [options/schema/sb100/%s]: %v", uriPath, err)
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(fileBuf)), nil
		}
		var err error
		sb100Schema, err = jsonschema.Compile(burritoBiblePrefix + "metadata.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return sb100Schema, nil
}

// ReadFileFromBlob reads a file from a blob and returns the content
func ReadFileFromBlob(blob *git.Blob) ([]byte, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return nil, err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return nil, err
	}
	return buf, nil
}

// ReadYAMLFromBlob reads a yaml file from a blob and unmarshals it
func ReadYAMLFromBlob(blob *git.Blob) (*map[string]interface{}, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}

	var result *map[string]interface{}
	if err := yaml.Unmarshal(buf, &result); err != nil {
		log.Error("yaml.Unmarshal: %v", err)
		return nil, err
	}
	if result != nil {
		for k, v := range *result {
			if val, err := ToStringKeys(v); err != nil {
				log.Error("ToStringKeys: %v", err)
			} else {
				(*result)[k] = val
			}
		}
	}
	return result, nil
}

type SBEncodedMetadata struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

type SBMetadata100 struct {
	Format         string                         `json:"format"`
	Meta           SB100Meta                      `json:"meta"`
	Identification SB100Identification            `json:"identification"`
	Languages      []SB100Language                `json:"languages"`
	Type           SB100Type                      `json:"type"`
	LocalizedNames *map[string]SB100LocalizedName `json:"localizedNames"`
	Metadata       *map[string]interface{}
}

type SB100Meta struct {
	Version       string `json:"version"`
	DefaultLocal  string `json:"defaultLocale"`
	DateCreate    string `json:"dateCreated"`
	Normalization string `json:"normalization:"`
}

type SB100Identification struct {
	Name         SB100En `json:"name"`
	Abbreviation SB100En `json:"abbreviation"`
}

type SB100En struct {
	En string `json:"en"`
}

type SB100Language struct {
	Tag  string  `json:"tag"`
	Name SB100En `json:"name"`
}

type SB100Type struct {
	FlavorType SB100FlavorType `json:"flavorType"`
}

type SB100FlavorType struct {
	Name   string      `json:"name"`
	Flavor SB100Flavor `json:"flavor"`
}

type SB100Flavor struct {
	Name string `json:"name"`
}

type SB100LocalizedName struct {
	Short SB100En `json:"short"`
	Abbr  SB100En `json:"abbr"`
	Long  SB100En `json:"long"`
}

// tc v7: https://git.door43.org/qa99/en_ult_rom_book/raw/branch/master/manifest.json
// tc v8: https://git.door43.org/pjoakes/en_ust_2co_book/src/branch/master/manifest.json
// ts v3: https://git.door43.org/test2/uw-obs-aas/src/branch/master/manifest.json
// ts v5: https://git.door43.org/69c530493aab80e7/uw-mrk-lol/raw/branch/master/manifest.json
// ts v6: https://git.door43.org/Sitorabi/def_obs_text_obs/src/branch/master/manifest.json
type TcTsManifest struct {
	TcVersion       int    `json:"tc_version"`      // for tC
	TsVersion       int    `json:"package_version"` // for tS
	MetadataVersion string // To be filled in below
	MetadataType    string // To Be filled in below
	Format          string `json:"format"`
	Subject         string // To be filled in below
	Title           string // To be filled in below
	TargetLanguage  struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Direction string `json:"direction"`
	} `json:"target_language"`
	Type struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"type"`
	ResourceID string `json:"resource_id"` // for tS package_version < 5
	Resource   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"resource"`
	Project struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
}

func GetTcTsManifestFromBlob(blob *git.Blob) (*TcTsManifest, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}
	t := &TcTsManifest{}
	err = json.Unmarshal(buf, t)
	if err != nil {
		return nil, err
	}
	if t.TcVersion >= 7 {
		t.MetadataVersion = strconv.Itoa(t.TcVersion)
		t.MetadataType = "tc"
		t.Format = "usfm"
		t.Subject = "Aligned Bible"
	} else if t.TsVersion >= 3 {
		t.MetadataVersion = strconv.Itoa(t.TsVersion)
		t.MetadataType = "ts"
		if t.Resource.ID == "" {
			t.Resource.ID = t.ResourceID
		}
		if t.Resource.Name == "" {
			t.Resource.Name = strings.ToUpper(t.Resource.ID)
		}

		if t.Project.Name == "" {
			t.Project.Name = strings.ToUpper(t.Project.ID)
		}
		if t.Resource.ID == "obs" {
			t.Subject = "Open Bible Stories"
		} else {
			t.Subject = "Bible"
		}
	} else {
		return nil, nil
	}

	if t.Resource.Name != "" {
		t.Title = t.Resource.Name
	}
	if strings.ToLower(t.Resource.ID) != "obs" && t.Project.Name != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(t.Project.Name)) {
		if t.Title != "" {
			t.Title += " - "
		}
		t.Title += t.Project.Name
	}

	return t, nil
}

func GetSBDataFromBlob(blob *git.Blob) (*SBMetadata100, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}
	sbEncoded := &SBEncodedMetadata{}
	if err = json.Unmarshal(buf, sbEncoded); err == nil {
		buf = sbEncoded.Data
	}

	sb100 := &SBMetadata100{}
	if err := json.Unmarshal(buf, sb100); err != nil {
		return nil, err
	}

	// Now make a generic map of the buffer to store in the database table
	sb100.Metadata = &map[string]interface{}{}
	if err := json.Unmarshal(buf, sb100.Metadata); err != nil {
		return nil, err
	}

	return sb100, nil
}

// ReadJSONFromBlob reads a json file from a blob and unmarshals it
func ReadJSONFromBlob(blob *git.Blob) (*map[string]interface{}, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}

	var result *map[string]interface{}
	if err := json.Unmarshal(buf, &result); err != nil {
		log.Error("json.Unmarshal: %v", err)
		return nil, err
	}
	if result != nil {
		for k, v := range *result {
			if val, err := ToStringKeys(v); err != nil {
				log.Error("ToStringKeys: %v", err)
			} else {
				(*result)[k] = val
			}
		}
	}
	return result, nil
}

// ValidateJSONFromBlob reads a json file from a blob and unmarshals it returning any errors
func ValidateJSONFromBlob(blob *git.Blob) error {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return err
	}

	var result interface{}
	err = json.Unmarshal(buf, &result)
	if err != nil {
		log.Error("json.Unmarshal: %v", err)
	}
	return err
}
