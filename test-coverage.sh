#! /usr/bin/env bash

set -e
set -o verbose

for PKG in $(go list ./... | grep -v /vendor/);
do
  go test -cover -coverprofile $GOPATH/src/$PKG/coverage.out $PKG || exit 1;
done;
gocovmerge $(find -type f -name "coverage.out") > coverage.out;
goveralls -coverprofile=coverage.out;
