#!/bin/bash

set -Eeuo pipefail

BRANCH=main

if [[ $# -gt 1 ]] ; then
  BRANCH=$1
fi

adduser freeps --home /usr/local/freeps --system --ingroup video

# TODO(HR): curl
cp systemd/freepsd.service /etc/systemd/system/freepsd.service
cp scripts/update-freeps.sh /usr/local/freeps/
mkdir -p /etc/freepsd && chown freeps /etc/freepsd

sudo -u freeps /usr/local/freeps/update-freeps.sh $BRANCH
ln -s /usr/local/freeps/update-freeps.sh /bin/update-freeps
ln -s /usr/local/freeps/bin/freepsd /bin/freepsd

systemctl daemon-reload
systemctl restart freepsd