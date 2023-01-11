PACKAGE=github.com/hannesrauhe/freeps
VERSION=$(shell git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')
COMMIT_HASH=$(shell git rev-parse --short HEAD)
BUILD_TIMESTAMP=$(shell date '+%Y-%m-%dT%H:%M:%S')

build:
	go build -ldflags="-X ${PACKAGE}/utils.Version=${VERSION} -X ${PACKAGE}/utils.CommitHash=${COMMIT_HASH} -X ${PACKAGE}/utils.BuildTime=${BUILD_TIMESTAMP}" -o freepsd/freepsd freepsd/freepsd.go

install: freepsd/freepsd
	mv freepsd/freepsd /usr/bin/freepsd
	adduser freeps --no-create-home --system --ingroup video
	cp systemd/freepsd.service /etc/systemd/system/freepsd.service
	systemctl daemon-reload
	systemctl restart freepsd
