#!/usr/bin/env bash
# -*- coding: utf8 -*-
#
#  Copyright (c) 2017 unfoldingWord
#  http://creativecommons.org/licenses/MIT/
#  See LICENSE file for details.
#
#  Contributors:
#  Jesse Griffin <jesse@unfoldingword.org>

# Temp and log location
TMPDIR=/tmp/gogs_test

get_tmp() {
    # Setup temporary environment
    rm -rf "$TMPDIR"
    mkdir -p "$TMPDIR"
    cd "$TMPDIR"
}

help() {
    echo "Runs acceptance tests against our Door43 Content Service"
    echo
    echo "Usage: $0 [options]"
    echo
    echo "Options:"
    echo "    -H       Host to run against, default is localhost:3000"
    echo "    -t       Token for testing API"
    echo "    -p       Password for acceptance_test user"
    echo "    -h       Displays this messsage"
}

while test $# -gt 0; do
    case "$1" in
        -H|--hostname) shift; HOST=$1;;
        -t|--token) shift; TOKEN=$1;;
        -p|--pass) shift; PASS=":$1";;
        -h|--help) help && exit 1;;
    esac
    shift;
done

# Setup variable defaults in case flags were not set
: ${HOST='localhost:3000'}
: ${TOKEN=false}
: ${PASS=""}

echo -n "Testing $HOST"

# Show commands being executed and exit upon any error
set -xe

export GOPATH=/home/travis/gopath
export GOGSPATH=$GOPATH/src/code.gitea.io/gitea
$GOPATH/bin/gitea &

wget -q -O - http://$HOST

rm -rf "$TMPDIR"
