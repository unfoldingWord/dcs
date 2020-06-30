// Copyright 2020 unfoldingWord. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import "fmt"
import "reflect"
import "encoding/json"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020CheckingCheckingLevel) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020CheckingCheckingLevel {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020CheckingCheckingLevel, v)
	}
	*j = RC020CheckingCheckingLevel(v)
	return nil
}

// LocalizedText A textual string specified in one or multiple languages, indexed by IETF
// language tag.
type LocalizedText map[string]interface{}

// MimeType An IANA media type (also known as MIME type)
type MimeType string

// Path A file path, delimited by forward slashes.
type Path string

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCore) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["language"]; !ok || v == nil {
		return fmt.Errorf("field language: required")
	}
	if v, ok := raw["subject"]; !ok || v == nil {
		return fmt.Errorf("field subject: required")
	}
	type Plain RC020DublinCore
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["conformsto"]; !ok || v == nil {
		plain.Conformsto = "rc0.2"
	}
	if v, ok := raw["contributor"]; !ok || v == nil {
		plain.Contributor = []string{}
	}
	if v, ok := raw["creator"]; !ok || v == nil {
		plain.Creator = ""
	}
	if v, ok := raw["description"]; !ok || v == nil {
		plain.Description = ""
	}
	if v, ok := raw["format"]; !ok || v == nil {
		plain.Format = ""
	}
	if v, ok := raw["identifier"]; !ok || v == nil {
		plain.Identifier = ""
	}
	if v, ok := raw["issued"]; !ok || v == nil {
		plain.Issued = ""
	}
	if v, ok := raw["modified"]; !ok || v == nil {
		plain.Modified = ""
	}
	if v, ok := raw["publisher"]; !ok || v == nil {
		plain.Publisher = ""
	}
	if v, ok := raw["relation"]; !ok || v == nil {
		plain.Relation = []RelationItem{}

	}
	if v, ok := raw["rights"]; !ok || v == nil {
		plain.Rights = "CC BY-SA 4.0"
	}
	if v, ok := raw["source"]; !ok || v == nil {
		plain.Source = []RC020DublinCoreSourceElem{}

	}
	if v, ok := raw["title"]; !ok || v == nil {
		plain.Title = ""
	}
	if v, ok := raw["type"]; !ok || v == nil {
		plain.Type = ""
	}
	if v, ok := raw["version"]; !ok || v == nil {
		plain.Version = ""
	}
	*j = RC020DublinCore(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *ProjectIdentifier) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesProjectIdentifier {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesProjectIdentifier, v)
	}
	*j = ProjectIdentifier(v)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreLanguageDirection) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020DublinCoreLanguageDirection {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020DublinCoreLanguageDirection, v)
	}
	*j = RC020DublinCoreLanguageDirection(v)
	return nil
}

// LanguageTag A valid IETF language tag as specified by BCP 47.
type LanguageTag string

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020ProjectsElem) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["path"]; !ok || v == nil {
		return fmt.Errorf("field path: required")
	}
	type Plain RC020ProjectsElem
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["categories"]; !ok || v == nil {
		plain.Categories = []interface{}{}

	}
	if v, ok := raw["identifier"]; !ok || v == nil {
		plain.Identifier = ""
	}
	if v, ok := raw["sort"]; !ok || v == nil {
		plain.Sort = 0
	}
	if v, ok := raw["title"]; !ok || v == nil {
		plain.Title = ""
	}
	*j = RC020ProjectsElem(plain)
	return nil
}

// ProjectIdentifierA1Co A1Co
const ProjectIdentifierA1Co ProjectIdentifier = "1co"

// ProjectIdentifierA1Pe A1Pe
const ProjectIdentifierA1Pe ProjectIdentifier = "1pe"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreSubject) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020DublinCoreSubject {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020DublinCoreSubject, v)
	}
	*j = RC020DublinCoreSubject(v)
	return nil
}

// ProjectIdentifierA1Sa A1Sa
const ProjectIdentifierA1Sa ProjectIdentifier = "1sa"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020ProjectsElemVersification) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020ProjectsElemVersification {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020ProjectsElemVersification, v)
	}
	*j = RC020ProjectsElemVersification(v)
	return nil
}

// ProjectIdentifierA1Ki A1Ki
const ProjectIdentifierA1Ki ProjectIdentifier = "1ki"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020Checking) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type Plain RC020Checking
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["checking_entity"]; !ok || v == nil {
		plain.CheckingEntity = []string{}

	}
	if v, ok := raw["checking_level"]; !ok || v == nil {
		plain.CheckingLevel = "1"
	}
	*j = RC020Checking(plain)
	return nil
}

// ProjectIdentifierA1Ch A1Ch
const ProjectIdentifierA1Ch ProjectIdentifier = "1ch"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreLanguage) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["identifier"]; !ok || v == nil {
		return fmt.Errorf("field identifier: required")
	}
	type Plain RC020DublinCoreLanguage
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["direction"]; !ok || v == nil {
		plain.Direction = "ltr"
	}
	if v, ok := raw["title"]; !ok || v == nil {
		plain.Title = ""
	}
	*j = RC020DublinCoreLanguage(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreRights) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020DublinCoreRights {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020DublinCoreRights, v)
	}
	*j = RC020DublinCoreRights(v)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreSourceElem) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type Plain RC020DublinCoreSourceElem
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["identifier"]; !ok || v == nil {
		plain.Identifier = ""
	}
	if v, ok := raw["language"]; !ok || v == nil {
		plain.Language = ""
	}
	if v, ok := raw["version"]; !ok || v == nil {
		plain.Version = ""
	}
	*j = RC020DublinCoreSourceElem(plain)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreConformsto) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020DublinCoreConformsto {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020DublinCoreConformsto, v)
	}
	*j = RC020DublinCoreConformsto(v)
	return nil
}

// ProjectIdentifier identifies a project
type ProjectIdentifier string

// ProjectIdentifierA1Th A1Th
const ProjectIdentifierA1Th ProjectIdentifier = "1th"

// ProjectIdentifierA1Ti A1Ti
const ProjectIdentifierA1Ti ProjectIdentifier = "1ti"

// ProjectIdentifierA2Ch A2Ch
const ProjectIdentifierA2Ch ProjectIdentifier = "2ch"

// ProjectIdentifierA2Co A2Co
const ProjectIdentifierA2Co ProjectIdentifier = "2co"

// ProjectIdentifierA2Jn A2Jn
const ProjectIdentifierA2Jn ProjectIdentifier = "2jn"

// ProjectIdentifierA2Ki A2Ki
const ProjectIdentifierA2Ki ProjectIdentifier = "2ki"

// ProjectIdentifierA2Pe A2Pe
const ProjectIdentifierA2Pe ProjectIdentifier = "2pe"

// ProjectIdentifierA2Sa A2Sa
const ProjectIdentifierA2Sa ProjectIdentifier = "2sa"

// ProjectIdentifierA2Th A2Th
const ProjectIdentifierA2Th ProjectIdentifier = "2th"

// ProjectIdentifierA2Ti A2Ti
const ProjectIdentifierA2Ti ProjectIdentifier = "2ti"

// ProjectIdentifierA3Jn A3Jn
const ProjectIdentifierA3Jn ProjectIdentifier = "3jn"

// ProjectIdentifierAct Act
const ProjectIdentifierAct ProjectIdentifier = "act"

// ProjectIdentifierAmo Amo
const ProjectIdentifierAmo ProjectIdentifier = "amo"

// ProjectIdentifierBible Bible
const ProjectIdentifierBible ProjectIdentifier = "bible"

// ProjectIdentifierChecking Checking
const ProjectIdentifierChecking ProjectIdentifier = "checking"

// ProjectIdentifierCol Col
const ProjectIdentifierCol ProjectIdentifier = "col"

// ProjectIdentifierDan Dan
const ProjectIdentifierDan ProjectIdentifier = "dan"

// ProjectIdentifierDeu Deu
const ProjectIdentifierDeu ProjectIdentifier = "deu"

// ProjectIdentifierEcc Ecc
const ProjectIdentifierEcc ProjectIdentifier = "ecc"

// ProjectIdentifierEph Eph
const ProjectIdentifierEph ProjectIdentifier = "eph"

// ProjectIdentifierEst Est
const ProjectIdentifierEst ProjectIdentifier = "est"

// ProjectIdentifierExo Exo
const ProjectIdentifierExo ProjectIdentifier = "exo"

// ProjectIdentifierEzk Ezk
const ProjectIdentifierEzk ProjectIdentifier = "ezk"

// ProjectIdentifierEzr Ezr
const ProjectIdentifierEzr ProjectIdentifier = "ezr"

// ProjectIdentifierGal Gal
const ProjectIdentifierGal ProjectIdentifier = "gal"

// ProjectIdentifierGen Gen
const ProjectIdentifierGen ProjectIdentifier = "gen"

// ProjectIdentifierHab Hab
const ProjectIdentifierHab ProjectIdentifier = "hab"

// ProjectIdentifierHag Hag
const ProjectIdentifierHag ProjectIdentifier = "hag"

// ProjectIdentifierHeb Heb
const ProjectIdentifierHeb ProjectIdentifier = "heb"

// ProjectIdentifierHos Hos
const ProjectIdentifierHos ProjectIdentifier = "hos"

// ProjectIdentifierIntro Intro
const ProjectIdentifierIntro ProjectIdentifier = "intro"

// ProjectIdentifierIsa Isa
const ProjectIdentifierIsa ProjectIdentifier = "isa"

// ProjectIdentifierJas Jas
const ProjectIdentifierJas ProjectIdentifier = "jas"

// ProjectIdentifierJdg Jdg
const ProjectIdentifierJdg ProjectIdentifier = "jdg"

// ProjectIdentifierJer Jer
const ProjectIdentifierJer ProjectIdentifier = "jer"

// ProjectIdentifierJhn Jhn
const ProjectIdentifierJhn ProjectIdentifier = "jhn"

// ProjectIdentifierJob Job
const ProjectIdentifierJob ProjectIdentifier = "job"

// ProjectIdentifierJol Jol
const ProjectIdentifierJol ProjectIdentifier = "jol"

// ProjectIdentifierJon Jon
const ProjectIdentifierJon ProjectIdentifier = "jon"

// ProjectIdentifierJos Jos
const ProjectIdentifierJos ProjectIdentifier = "jos"

// ProjectIdentifierJud Jud
const ProjectIdentifierJud ProjectIdentifier = "jud"

// ProjectIdentifierLam Lam
const ProjectIdentifierLam ProjectIdentifier = "lam"

// ProjectIdentifierLev Lev
const ProjectIdentifierLev ProjectIdentifier = "lev"

// ProjectIdentifierLuk Luk
const ProjectIdentifierLuk ProjectIdentifier = "luk"

// ProjectIdentifierMal Mal
const ProjectIdentifierMal ProjectIdentifier = "mal"

// ProjectIdentifierMat Mat
const ProjectIdentifierMat ProjectIdentifier = "mat"

// ProjectIdentifierMic Mic
const ProjectIdentifierMic ProjectIdentifier = "mic"

// ProjectIdentifierMrk Mrk
const ProjectIdentifierMrk ProjectIdentifier = "mrk"

// ProjectIdentifierNam Nam
const ProjectIdentifierNam ProjectIdentifier = "nam"

// ProjectIdentifierNeh Neh
const ProjectIdentifierNeh ProjectIdentifier = "neh"

// ProjectIdentifierNum Num
const ProjectIdentifierNum ProjectIdentifier = "num"

// ProjectIdentifierOba Oba
const ProjectIdentifierOba ProjectIdentifier = "oba"

// ProjectIdentifierObs Obs
const ProjectIdentifierObs ProjectIdentifier = "obs"

// ProjectIdentifierPhm Phm
const ProjectIdentifierPhm ProjectIdentifier = "phm"

// ProjectIdentifierPhp Php
const ProjectIdentifierPhp ProjectIdentifier = "php"

// ProjectIdentifierPro Pro
const ProjectIdentifierPro ProjectIdentifier = "pro"

// ProjectIdentifierProcess Process
const ProjectIdentifierProcess ProjectIdentifier = "process"

// ProjectIdentifierPsa Psa
const ProjectIdentifierPsa ProjectIdentifier = "psa"

// ProjectIdentifierRev Rev
const ProjectIdentifierRev ProjectIdentifier = "rev"

// ProjectIdentifierRom Rom
const ProjectIdentifierRom ProjectIdentifier = "rom"

// ProjectIdentifierRut Rut
const ProjectIdentifierRut ProjectIdentifier = "rut"

// ProjectIdentifierSng Sng
const ProjectIdentifierSng ProjectIdentifier = "sng"

// ProjectIdentifierTit Tit
const ProjectIdentifierTit ProjectIdentifier = "tit"

// ProjectIdentifierTranslate Translate
const ProjectIdentifierTranslate ProjectIdentifier = "translate"

// ProjectIdentifierZec Zec
const ProjectIdentifierZec ProjectIdentifier = "zec"

// ProjectIdentifierZep Zep
const ProjectIdentifierZep ProjectIdentifier = "zep"

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020DublinCoreType) UnmarshalJSON(b []byte) error {
	var v string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	var ok bool
	for _, expected := range enumValuesRC020DublinCoreType {
		if reflect.DeepEqual(v, expected) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("invalid value (expected one of %#v): %#v", enumValuesRC020DublinCoreType, v)
	}
	*j = RC020DublinCoreType(v)
	return nil
}

// RC020Manifest Manifest
type RC020Manifest struct {
	// Checking corresponds to the JSON schema field "checking".
	Checking RC020Checking `json:"checking"`

	// DublinCore corresponds to the JSON schema field "dublin_core".
	DublinCore RC020DublinCore `json:"dublin_core"`

	// Projects corresponds to the JSON schema field "projects".
	Projects []RC020ProjectsElem `json:"projects"`
}

// RC020Checking Checking
type RC020Checking struct {
	// CheckingEntity corresponds to the JSON schema field "checking_entity".
	CheckingEntity []string `json:"checking_entity"`

	// CheckingLevel corresponds to the JSON schema field "checking_level".
	CheckingLevel RC020CheckingCheckingLevel `json:"checking_level"`
}

// RC020CheckingCheckingLevel CheckingCheckingLevel
type RC020CheckingCheckingLevel string

// RC020CheckingCheckingLevelA1 CheckingLevelA1
const RC020CheckingCheckingLevelA1 RC020CheckingCheckingLevel = "1"

// RC020CheckingCheckingLevelA2 CheckingLevelA2
const RC020CheckingCheckingLevelA2 RC020CheckingCheckingLevel = "2"

// RC020CheckingCheckingLevelA3 CheckingLevelA3
const RC020CheckingCheckingLevelA3 RC020CheckingCheckingLevel = "3"

// RC020DublinCore DublinCore
type RC020DublinCore struct {
	// Conformsto corresponds to the JSON schema field "conformsto".
	Conformsto RC020DublinCoreConformsto `json:"conformsto"`

	// Contributor corresponds to the JSON schema field "contributor".
	Contributor []string `json:"contributor"`

	// Creator corresponds to the JSON schema field "creator".
	Creator string `json:"creator"`

	// Description corresponds to the JSON schema field "description".
	Description string `json:"description"`

	// Format corresponds to the JSON schema field "format".
	Format MimeType `json:"format"`

	// Identifier corresponds to the JSON schema field "identifier".
	Identifier string `json:"identifier"`

	// Issued corresponds to the JSON schema field "issued".
	Issued Timestamp `json:"issued"`

	// Language corresponds to the JSON schema field "language".
	Language RC020DublinCoreLanguage `json:"language"`

	// Modified corresponds to the JSON schema field "modified".
	Modified Timestamp `json:"modified"`

	// Publisher corresponds to the JSON schema field "publisher".
	Publisher string `json:"publisher"`

	// Relation corresponds to the JSON schema field "relation".
	Relation []RelationItem `json:"relation"`

	// Rights corresponds to the JSON schema field "rights".
	Rights RC020DublinCoreRights `json:"rights"`

	// Source corresponds to the JSON schema field "source".
	Source []RC020DublinCoreSourceElem `json:"source"`

	// Subject corresponds to the JSON schema field "subject".
	Subject RC020DublinCoreSubject `json:"subject"`

	// Title corresponds to the JSON schema field "title".
	Title string `json:"title"`

	// Type corresponds to the JSON schema field "type".
	Type RC020DublinCoreType `json:"type"`

	// Version corresponds to the JSON schema field "version".
	Version string `json:"version"`
}

// RC020DublinCoreConformsto DublinCoreConformsto
type RC020DublinCoreConformsto string

// RC020DublinCoreConformstoRc02 ConformstoRc02
const RC020DublinCoreConformstoRc02 RC020DublinCoreConformsto = "rc0.2"

// RC020DublinCoreLanguage DublinCoreLanguage
type RC020DublinCoreLanguage struct {
	// Direction corresponds to the JSON schema field "direction".
	Direction RC020DublinCoreLanguageDirection `json:"direction"`

	// Identifier corresponds to the JSON schema field "identifier".
	Identifier LanguageTag `json:"identifier"`

	// Title corresponds to the JSON schema field "title".
	Title string `json:"title"`
}

// RC020DublinCoreLanguageDirection DublinCoreLanguageDirection
type RC020DublinCoreLanguageDirection string

// RC020DublinCoreLanguageDirectionLtr LanguageDirectionLtr
const RC020DublinCoreLanguageDirectionLtr RC020DublinCoreLanguageDirection = "ltr"

// RC020DublinCoreLanguageDirectionRtl LanguageDirectionRtl
const RC020DublinCoreLanguageDirectionRtl RC020DublinCoreLanguageDirection = "rtl"

// RC020DublinCoreRights DublinCoreRights
type RC020DublinCoreRights string

// RC020DublinCoreRightsCCBY30 RightsCCBY30
const RC020DublinCoreRightsCCBY30 RC020DublinCoreRights = "CC BY 3.0"

// RC020DublinCoreRightsCCBYSA30 RightsCCBYSA30
const RC020DublinCoreRightsCCBYSA30 RC020DublinCoreRights = "CC BY-SA 3.0"

// RC020DublinCoreRightsCCBYSA40 RightsCCBYSA40
const RC020DublinCoreRightsCCBYSA40 RC020DublinCoreRights = "CC BY-SA 4.0"

// RC020DublinCoreRightsFreeTranslate20InternationalPublicLicense RightsFreeTranslate20InternationalPublicLicense
const RC020DublinCoreRightsFreeTranslate20InternationalPublicLicense RC020DublinCoreRights = "Free Translate 2.0 International Public License"

// RC020DublinCoreRightsPublicDomain RightsPublicDomain
const RC020DublinCoreRightsPublicDomain RC020DublinCoreRights = "Public Domain"

// RC020DublinCoreSourceElem DublinCoreSourceElem
type RC020DublinCoreSourceElem struct {
	// Identifier corresponds to the JSON schema field "identifier".
	Identifier string `json:"identifier"`

	// Language corresponds to the JSON schema field "language".
	Language LanguageTag `json:"language"`

	// Version corresponds to the JSON schema field "version".
	Version string `json:"version"`
}

// RC020DublinCoreSubject DublinCoreSubject
type RC020DublinCoreSubject string

// RC020DublinCoreSubjectAlignedBible SubjectAlignedBible
const RC020DublinCoreSubjectAlignedBible RC020DublinCoreSubject = "Aligned Bible"

// RC020DublinCoreSubjectBible SubjectBible
const RC020DublinCoreSubjectBible RC020DublinCoreSubject = "Bible"

// RC020DublinCoreSubjectGreekNewTestament SubjectGreekNewTestament
const RC020DublinCoreSubjectGreekNewTestament RC020DublinCoreSubject = "Greek New Testament"

// RC020DublinCoreSubjectHebrewOldTestament SubjectHebrewOldTestament
const RC020DublinCoreSubjectHebrewOldTestament RC020DublinCoreSubject = "Hebrew Old Testament"

// RC020DublinCoreSubjectOBSTranslationNotes SubjectOBSTranslationNotes
const RC020DublinCoreSubjectOBSTranslationNotes RC020DublinCoreSubject = "OBS Translation Notes"

// RC020DublinCoreSubjectOBSTranslationQuestions SubjectOBSTranslationQuestions
const RC020DublinCoreSubjectOBSTranslationQuestions RC020DublinCoreSubject = "OBS Translation Questions"

// RC020DublinCoreSubjectOpenBibleStories SubjectOpenBibleStories
const RC020DublinCoreSubjectOpenBibleStories RC020DublinCoreSubject = "Open Bible Stories"

// RC020DublinCoreSubjectTSVTranslationNotes SubjectTSVTranslationNotes
const RC020DublinCoreSubjectTSVTranslationNotes RC020DublinCoreSubject = "TSV Translation Notes"

// RC020DublinCoreSubjectTranslationAcademy SubjectTranslationAcademy
const RC020DublinCoreSubjectTranslationAcademy RC020DublinCoreSubject = "Translation Academy"

// RC020DublinCoreSubjectTranslationNotes SubjectTranslationNotes
const RC020DublinCoreSubjectTranslationNotes RC020DublinCoreSubject = "Translation Notes"

// RC020DublinCoreSubjectTranslationQuestions SubjectTranslationQuestions
const RC020DublinCoreSubjectTranslationQuestions RC020DublinCoreSubject = "Translation Questions"

// RC020DublinCoreSubjectTranslationWords SubjectTranslationWords
const RC020DublinCoreSubjectTranslationWords RC020DublinCoreSubject = "Translation Words"

// RC020DublinCoreType DublinCoreType
type RC020DublinCoreType string

// RC020DublinCoreTypeBook TypeBook
const RC020DublinCoreTypeBook RC020DublinCoreType = "book"

// RC020DublinCoreTypeBundle TypeBundle
const RC020DublinCoreTypeBundle RC020DublinCoreType = "bundle"

// RC020DublinCoreTypeDict TypeDict
const RC020DublinCoreTypeDict RC020DublinCoreType = "dict"

// RC020DublinCoreTypeHelp TypeHelp
const RC020DublinCoreTypeHelp RC020DublinCoreType = "help"

// RC020DublinCoreTypeMan TypeMan
const RC020DublinCoreTypeMan RC020DublinCoreType = "man"

// RC020ProjectsElem ProjectsElem
type RC020ProjectsElem struct {
	// Categories corresponds to the JSON schema field "categories".
	Categories interface{} `json:"categories,omitempty"`

	// Identifier corresponds to the JSON schema field "identifier".
	Identifier ProjectIdentifier `json:"identifier"`

	// Path corresponds to the JSON schema field "path".
	Path Path `json:"path"`

	// Sort corresponds to the JSON schema field "sort".
	Sort int `json:"sort,omitempty"`

	// Title corresponds to the JSON schema field "title".
	Title string `json:"title"`

	// Versification corresponds to the JSON schema field "versification".
	Versification *RC020ProjectsElemVersification `json:"versification,omitempty"`
}

// RC020ProjectsElemVersification ProjectsElemVersification
type RC020ProjectsElemVersification string

// RC020ProjectsElemVersificationAvd Avd
const RC020ProjectsElemVersificationAvd RC020ProjectsElemVersification = "avd"

// RC020ProjectsElemVersificationBlank Blank
const RC020ProjectsElemVersificationBlank RC020ProjectsElemVersification = ""

// RC020ProjectsElemVersificationObs Obs
const RC020ProjectsElemVersificationObs RC020ProjectsElemVersification = "obs"

// RC020ProjectsElemVersificationOdx Odx
const RC020ProjectsElemVersificationOdx RC020ProjectsElemVersification = "odx"

// RC020ProjectsElemVersificationOdxHr OdxHr
const RC020ProjectsElemVersificationOdxHr RC020ProjectsElemVersification = "odx-hr"

// RC020ProjectsElemVersificationOther Other
const RC020ProjectsElemVersificationOther RC020ProjectsElemVersification = "other"

// RC020ProjectsElemVersificationRsc Rsc
const RC020ProjectsElemVersificationRsc RC020ProjectsElemVersification = "rsc"

// RC020ProjectsElemVersificationUfw Ufw
const RC020ProjectsElemVersificationUfw RC020ProjectsElemVersification = "ufw"

// RC020ProjectsElemVersificationUfwBn UfwBn
const RC020ProjectsElemVersificationUfwBn RC020ProjectsElemVersification = "ufw-bn"

// RC020ProjectsElemVersificationUfwMl UfwMl
const RC020ProjectsElemVersificationUfwMl RC020ProjectsElemVersification = "ufw-ml"

// RC020ProjectsElemVersificationUfwOdx UfwOdx
const RC020ProjectsElemVersificationUfwOdx RC020ProjectsElemVersification = "ufw-odx"

// RC020ProjectsElemVersificationUfwRev UfwRev
const RC020ProjectsElemVersificationUfwRev RC020ProjectsElemVersification = "ufw-rev"

// RelationItem A relation has valid IETF language tag as specified by BCP 47 and a valid
// resource, separated with a slash.
type RelationItem string

// Timestamp A date timestamp
type Timestamp string

// TrimmedText A string without surrounding whitespace characters.
type TrimmedText string

// URL A valid **Uniform Resource Locator**.
type URL string

var enumValuesProjectIdentifier = []interface{}{
	"gen",
	"exo",
	"lev",
	"num",
	"deu",
	"jos",
	"jdg",
	"rut",
	"1sa",
	"2sa",
	"1ki",
	"2ki",
	"1ch",
	"2ch",
	"ezr",
	"neh",
	"est",
	"job",
	"psa",
	"pro",
	"ecc",
	"sng",
	"isa",
	"jer",
	"lam",
	"ezk",
	"dan",
	"hos",
	"jol",
	"amo",
	"oba",
	"jon",
	"mic",
	"nam",
	"hab",
	"zep",
	"hag",
	"zec",
	"mal",
	"mat",
	"mrk",
	"luk",
	"jhn",
	"act",
	"rom",
	"1co",
	"2co",
	"gal",
	"eph",
	"php",
	"col",
	"1th",
	"2th",
	"1ti",
	"2ti",
	"tit",
	"phm",
	"heb",
	"jas",
	"1pe",
	"2pe",
	"1jn",
	"2jn",
	"3jn",
	"jud",
	"rev",
	"obs",
	"intro",
	"process",
	"translate",
	"checking",
	"bible",
}
var enumValuesRC020CheckingCheckingLevel = []interface{}{
	"1",
	"2",
	"3",
}
var enumValuesRC020DublinCoreConformsto = []interface{}{
	"rc0.2",
}
var enumValuesRC020DublinCoreLanguageDirection = []interface{}{
	"ltr",
	"rtl",
}
var enumValuesRC020DublinCoreRights = []interface{}{
	"CC BY 3.0",
	"CC BY-SA 3.0",
	"CC BY-SA 4.0",
	"Free Translate 2.0 International Public License",
	"Public Domain",
}
var enumValuesRC020DublinCoreSubject = []interface{}{
	"Aligned Bible",
	"Bible",
	"Greek New Testament",
	"Hebrew Old Testament",
	"OBS Translation Notes",
	"OBS Translation Questions",
	"Open Bible Stories",
	"Translation Academy",
	"Translation Notes",
	"Translation Questions",
	"Translation Words",
	"TSV Translation Notes",
}
var enumValuesRC020DublinCoreType = []interface{}{
	"book",
	"bundle",
	"dict",
	"help",
	"man",
}
var enumValuesRC020ProjectsElemVersification = []interface{}{
	"avd",
	"odx",
	"odx-hr",
	"other",
	"rsc",
	"ufw",
	"ufw-bn",
	"ufw-ml",
	"ufw-odx",
	"ufw-rev",
	"obs",
	"",
	nil,
}

// UnmarshalJSON implements json.Unmarshaler.
func (j *RC020Manifest) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["checking"]; !ok || v == nil {
		return fmt.Errorf("field checking: required")
	}
	if v, ok := raw["dublin_core"]; !ok || v == nil {
		return fmt.Errorf("field dublin_core: required")
	}
	type Plain RC020Manifest
	var plain Plain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	if v, ok := raw["projects"]; !ok || v == nil {
		plain.Projects = []RC020ProjectsElem{}

	}
	*j = RC020Manifest(plain)
	return nil
}
