// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit
// +build !gogit

package git

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"reflect"

	"code.gitea.io/gitea/modules/log"
)

// Blob represents a Git object.
type Blob struct {
	ID SHA1

	gotSize bool
	size    int64
	name    string
	repo    *Repository
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	wr, rd, cancel := b.repo.CatFileBatch()

	_, err := wr.Write([]byte(b.ID.String() + "\n"))
	if err != nil {
		cancel()
		return nil, err
	}
	_, _, size, err := ReadBatchLine(rd)
	if err != nil {
		cancel()
		return nil, err
	}
	b.gotSize = true
	b.size = size

	if size < 4096 {
		bs, err := ioutil.ReadAll(io.LimitReader(rd, size))
		defer cancel()
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		return ioutil.NopCloser(bytes.NewReader(bs)), err
	}

	return &blobReader{
		rd:     rd,
		n:      size,
		cancel: cancel,
	}, nil
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	if b.gotSize {
		return b.size
	}

	wr, rd, cancel := b.repo.CatFileBatchCheck()
	defer cancel()
	_, err := wr.Write([]byte(b.ID.String() + "\n"))
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}
	_, _, b.size, err = ReadBatchLine(rd)
	if err != nil {
		log.Debug("error whilst reading size for %s in %s. Error: %v", b.ID.String(), b.repo.Path, err)
		return 0
	}

	b.gotSize = true

	return b.size
}

type blobReader struct {
	rd     *bufio.Reader
	n      int64
	cancel func()
}

func (b *blobReader) Read(p []byte) (n int, err error) {
	if b.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.n {
		p = p[0:b.n]
	}
	n, err = b.rd.Read(p)
	b.n -= int64(n)
	return
}

// Close implements io.Closer
func (b *blobReader) Close() error {
	defer func() {
		log.Debug("RICH: Before b.cancel()")
		b.cancel()
		log.Debug("RICH: After b.cancel()")
	}()
	log.Debug("RICH: b: %v", b)
	log.Debug("RICH: b.rd == nil: %b", b.rd == nil)
	log.Debug("RICH: typeOf: %s", reflect.TypeOf(b.rd))
	log.Debug("RICH: Kind: %s", reflect.ValueOf(b.rd).Kind())
	if b.n > 0 {
		log.Debug("RICH: b.n > 0")
		for b.n > math.MaxInt32 {
			log.Debug("RICH: b.n > math.MaxInt32: %d", b.n)
			log.Debug("RICH: Before Discard(math.MaxInt32): %d", math.MaxInt32)
			n, err := b.rd.Discard(math.MaxInt32)
			log.Debug("RICH: before b.n -= int64(%d)", n)
			b.n -= int64(n)
			log.Debug("RICH: after b.n -= int64(%d): %d", n, b.n)
			if err != nil {
				log.Debug("RICH: Discard(math.MaxInt32) Error: %v", err)
				return err
			}
			b.n -= math.MaxInt32
			log.Debug("RICH: b.n -= math.MaxInt32: %d", b.n)
		}
		log.Debug("RICH: Before Discard(b.n): %d", b.n)
		n, err := b.rd.Discard(int(b.n))
		log.Debug("RICH: Before b.n -= int64(%d)", n)
		b.n -= int64(n)
		log.Debug("RICH: after b.n -= int64(%d): %d", n, b.n)
		if err != nil {
			log.Debug("RICH: Discard(b.n) Error: %v", err)
			return err
		}
	}
	log.Debug("RICH: before if b.n == 0: %d", b.n)
	if b.n == 0 {
		log.Debug("RICH: in if b.n == 0, before b.rd.Discard(1): %d", b.n)
		_, err := b.rd.Discard(1)
		log.Debug("RICH: after b.rd.Discard(1): %v", err)
		b.n--
		log.Debug("RICH: after b.n--: %d", b.n)
		return err
	}
	log.Debug("RICH: returning nil (end of Close())")
	return nil
}
