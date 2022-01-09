#!/usr/bin/env bash

SHELL := /bin/bash

PKG := github.com/dh1tw/natsProxy
COMMITID := $(shell git describe --always --long --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --tags)

PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/)

all: build

build:
	go build -v -ldflags="-X github.com/dh1tw/natsProxy/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/natsProxy/cmd.version=${VERSION}"

# strip off dwraf table - used for travis CI

dist:
	go build -v -ldflags="-w -s -X github.com/dh1tw/natsProxy/cmd.commitHash=${COMMIT} \
		-X github.com/dh1tw/natsProxy/cmd.version=${VERSION}"
	# compress binary
	if [ "${GOOS}" == "windows" ]; then upx natsProxy.exe; else upx natsProxy; fi

# test:
# 	@go test -short ${PKG_LIST}

vet:
	@go vet ${PKG_LIST}

lint:
	@for file in ${GO_FILES} ;  do \
		golint $$file ; \
	done

test:
	go test ./...

clean:
	-@rm natsProxy natsProxy-v*

.PHONY: build vet lint clean