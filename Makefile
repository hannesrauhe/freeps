PACKAGE=github.com/hannesrauhe/freeps
VERSION=$(shell git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')
COMMIT_HASH=$(shell git rev-parse --short HEAD)
BUILD_TIMESTAMP=$(shell date '+%Y-%m-%dT%H:%M:%S')

.PHONY: build/freepsd build/freepsd-light

all: build/freepsd build/freepsd-light

freepslisten/static_server_content/chota.min.css:
	curl https://raw.githubusercontent.com/jenil/chota/v0.8.1/dist/chota.min.css -o freepslisten/static_server_content/chota.min.css

build:
	mkdir -p build

build/freepsd: build freepslisten/static_server_content/chota.min.css
	go build -ldflags="-X ${PACKAGE}/utils.Version=${VERSION} -X ${PACKAGE}/utils.CommitHash=${COMMIT_HASH} -X ${PACKAGE}/utils.BuildTime=${BUILD_TIMESTAMP}" -o build/freepsd freepsd/freepsd.go

build/freepsd-light: build freepslisten/static_server_content/chota.min.css
	go build -tags nopostgress -tags nomuteme -ldflags="-X ${PACKAGE}/utils.Version=${VERSION} -X ${PACKAGE}/utils.CommitHash=${COMMIT_HASH} -X ${PACKAGE}/utils.BuildTime=${BUILD_TIMESTAMP}" -o build/freepsd-light freepsd/freepsd.go

install:
	mv build/freepsd /usr/bin/freepsd
	adduser freeps --no-create-home --system --ingroup video
	cp systemd/freepsd.service /etc/systemd/system/freepsd.service
	mkdir -p /etc/freepsd && chown freeps /etc/freepsd
	systemctl daemon-reload
	systemctl restart freepsd
