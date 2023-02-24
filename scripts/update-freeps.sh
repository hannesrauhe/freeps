#!/bin/bash

set -Eeuo pipefail

if [ "$(whoami)" != freeps ] ; then
    echo "Must be run as user freeps"
    exit 1
fi

pushd /usr/local/freeps

if [ ! -d src ] ; then
    git clone https://github.com/hannesrauhe/freeps.git src
fi

pushd src
git fetch
git clean -f
git reset --hard HEAD

BRANCH=main
if [[ $# -ge 1 ]] ; then
  BRANCH=$1
elif git rev-parse --is-inside-work-tree ; then
  BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi
echo "Updating freeps from branch $BRANCH"

git checkout $BRANCH
make
popd

if [ ! -L bin ] ; then
    ln -s src/build bin
fi

popd
