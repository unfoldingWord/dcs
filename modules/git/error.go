// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// ErrNotExist commit not exist error
type ErrNotExist struct {
	ID      string
	RelPath string
}

// IsErrNotExist if some error is ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("object does not exist [id: %s, rel_path: %s]", err.ID, err.RelPath)
}

func (err ErrNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrBranchNotExist represents a "BranchNotExist" kind of error.
type ErrBranchNotExist struct {
	Name string
}

// IsErrBranchNotExist checks if an error is a ErrBranchNotExist.
func IsErrBranchNotExist(err error) bool {
	_, ok := err.(ErrBranchNotExist)
	return ok
}

func (err ErrBranchNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [name: %s]", err.Name)
}

func (err ErrBranchNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrPushOutOfDate represents an error if merging fails due to the base branch being updated
type ErrPushOutOfDate struct {
	StdOut string
	StdErr string
	Err    error
}

// IsErrPushOutOfDate checks if an error is a ErrPushOutOfDate.
func IsErrPushOutOfDate(err error) bool {
	_, ok := err.(*ErrPushOutOfDate)
	return ok
}

func (err *ErrPushOutOfDate) Error() string {
	return fmt.Sprintf("PushOutOfDate Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// Unwrap unwraps the underlying error
func (err *ErrPushOutOfDate) Unwrap() error {
	return fmt.Errorf("%w - %s", err.Err, err.StdErr)
}

// ErrPushRejected represents an error if merging fails due to rejection from a hook
type ErrPushRejected struct {
	Message string
	StdOut  string
	StdErr  string
	Err     error
}

// IsErrPushRejected checks if an error is a ErrPushRejected.
func IsErrPushRejected(err error) bool {
	_, ok := err.(*ErrPushRejected)
	return ok
}

func (err *ErrPushRejected) Error() string {
	return fmt.Sprintf("PushRejected Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// Unwrap unwraps the underlying error
func (err *ErrPushRejected) Unwrap() error {
	return fmt.Errorf("%w - %s", err.Err, err.StdErr)
}

// GenerateMessage generates the remote message from the stderr
func (err *ErrPushRejected) GenerateMessage() {
	// The stderr is like this:
	//
	// > remote: error: push is rejected .....
	// > To /work/gitea/tests/integration/gitea-integration-sqlite/gitea-repositories/user2/repo1.git
	// >  ! [remote rejected] 44e67c77559211d21b630b902cdcc6ab9d4a4f51 -> develop (pre-receive hook declined)
	// > error: failed to push some refs to '/work/gitea/tests/integration/gitea-integration-sqlite/gitea-repositories/user2/repo1.git'
	//
	// The local message contains sensitive information, so we only need the remote message
	const prefixRemote = "remote: "
	const prefixError = "error: "
	pos := strings.Index(err.StdErr, prefixRemote)
	if pos < 0 {
		err.Message = "push is rejected"
		return
	}

	messageBuilder := &strings.Builder{}
	lines := strings.SplitSeq(err.StdErr, "\n")
	for line := range lines {
		line, ok := strings.CutPrefix(line, prefixRemote)
		if !ok {
			continue
		}
		line = strings.TrimPrefix(line, prefixError)
		messageBuilder.WriteString(strings.TrimSpace(line) + "\n")
	}
	err.Message = strings.TrimSpace(messageBuilder.String())
}

// ErrMoreThanOne represents an error if pull request fails when there are more than one sources (branch, tag) with the same name
type ErrMoreThanOne struct {
	StdOut string
	StdErr string
	Err    error
}

// IsErrMoreThanOne checks if an error is a ErrMoreThanOne
func IsErrMoreThanOne(err error) bool {
	_, ok := err.(*ErrMoreThanOne)
	return ok
}

func (err *ErrMoreThanOne) Error() string {
	return fmt.Sprintf("ErrMoreThanOne Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

/*** DCS Customizations ***/

// ErrRefNotFound represents a "RefDoesMotExist" kind of error.
type ErrRefNotFound struct {
	RefName string
}

// IsErrRefNotFound checks if an error is a ErrRefNotFound.
func IsErrRefNotFound(err error) bool {
	_, ok := err.(ErrRefNotFound)
	return ok
}

func (err ErrRefNotFound) Error() string {
	return fmt.Sprintf("ref does not exist [ref_name: %s]", err.RefName)
}

// ErrInvalidRefName represents a "InvalidRefName" kind of error.
type ErrInvalidRefName struct {
	RefName string
	Reason  string
}

// IsErrInvalidRefName checks if an error is a ErrInvalidRefName.
func IsErrInvalidRefName(err error) bool {
	_, ok := err.(ErrInvalidRefName)
	return ok
}

func (err ErrInvalidRefName) Error() string {
	return fmt.Sprintf("ref name is not valid: %s [ref_name: %s]", err.Reason, err.RefName)
}

// ErrProtectedRefName represents a "ProtectedRefName" kind of error.
type ErrProtectedRefName struct {
	RefName string
	Message string
}

// IsErrProtectedRefName checks if an error is a ErrProtectedRefName.
func IsErrProtectedRefName(err error) bool {
	_, ok := err.(ErrProtectedRefName)
	return ok
}

func (err ErrProtectedRefName) Error() string {
	str := fmt.Sprintf("ref name is protected [ref_name: %s]", err.RefName)
	if err.Message != "" {
		str = fmt.Sprintf("%s: %s", str, err.Message)
	}
	return str
}

// ErrRefAlreadyExists represents an error that ref with such name already exists.
type ErrRefAlreadyExists struct {
	RefName string
}

// IsErrRefAlreadyExists checks if an error is an ErrRefAlreadyExists.
func IsErrRefAlreadyExists(err error) bool {
	_, ok := err.(ErrRefAlreadyExists)
	return ok
}

func (err ErrRefAlreadyExists) Error() string {
	return fmt.Sprintf("ref already exists [name: %s]", err.RefName)
}

/*** END DCS Customizations ***/
