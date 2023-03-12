PACKAGE=github.com/hannesrauhe/freeps
VERSION=$(shell git describe --tags --always --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')
COMMIT_HASH=$(shell git rev-parse --short HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
BUILD_TIMESTAMP=$(shell date '+%Y-%m-%dT%H:%M:%S')
INSTALL_PREFIX=/usr/local

.PHONY: build/freepsd build/freepsd-light

all: build/freepsd build/freepsd-light

freepslisten/static_server_content/chota.min.css:
	curl https://raw.githubusercontent.com/jenil/chota/v0.8.1/dist/chota.min.css -o freepslisten/static_server_content/chota.min.css

build:
	mkdir -p build

build/freepsd: build freepslisten/static_server_content/chota.min.css
	go build -ldflags="-X ${PACKAGE}/utils.Version=${VERSION} -X ${PACKAGE}/utils.CommitHash=${COMMIT_HASH} -X ${PACKAGE}/utils.BuildTime=${BUILD_TIMESTAMP} -X ${PACKAGE}/utils.Branch=${BRANCH}" -o build/freepsd freepsd/freepsd.go

build/freepsd-light: build freepslisten/static_server_content/chota.min.css
	go build -tags nopostgress -tags nomuteme -ldflags="-X ${PACKAGE}/utils.Version=${VERSION} -X ${PACKAGE}/utils.CommitHash=${COMMIT_HASH} -X ${PACKAGE}/utils.BuildTime=${BUILD_TIMESTAMP} -X ${PACKAGE}/utils.Branch=${BRANCH}" -o build/freepsd-light freepsd/freepsd.go

# if you are reading this to learn how freepsd is deployed: freepsd runs without any additional libraries. Just run it.
# this just creates a user and a srevice and an optional update-script
install:
	adduser freeps --home ${INSTALL_PREFIX}/freeps --system --ingroup video
	cp systemd/freepsd.service /etc/systemd/system/freepsd.service
	mkdir -p /etc/freepsd && chown freeps /etc/freepsd
	mkdir -p ${INSTALL_PREFIX}/freeps/bin
	mv build/freepsd scripts/update-freeps.sh ${INSTALL_PREFIX}/freeps/bin/
	chown -R freeps ${INSTALL_PREFIX}/freeps
	ln -s ${INSTALL_PREFIX}/freeps/bin/freepsd /usr/bin
	systemctl daemon-reload
	systemctl restart freepsd
