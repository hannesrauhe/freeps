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
git clean -f
git reset --hard HEAD

BRANCH=main
if [[ $# -ge 1 ]] ; then
  BRANCH=$1
elif git rev-parse --is-inside-work-tree ; then
  BRANCH=$(git rev-parse --abbrev-ref HEAD)
fi
echo "Updating freeps from branch $BRANCH"

git fetch --all
git checkout $BRANCH
git pull --ff-only
make
popd

rm -rf bin.bak
mv bin bin.bak
mkdir -p bin
cp src/build/freepsd src/scripts/update-freeps.sh bin

popd
