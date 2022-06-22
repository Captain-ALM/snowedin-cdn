SHELL := /bin/bash
BIN := dist/snowedin
HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git show -s --format=%ci ${HASH})
BUILD_DATE := $(shell date '+%Y-%m-%d %H:%M:%S')
VERSION := ${HASH}
LD_FLAGS := -s -w -X 'main.buildVersion=${VERSION}' -X 'main.buildDate=${BUILD_DATE}'
COMP_BIN := go

.PHONY: build run test clean debug

build:
	mkdir -p dist/
	${COMP_BIN} build -o "${BIN}" -ldflags="${LD_FLAGS}" ./cmd/snowedin

run:
	./${BIN}

test:
	go test

clean:
	go clean
	rm ${BIN}

debug:
	mkdir -p dist/
	${COMP_BIN} build -o "dist/debug" -ldflags="${LD_FLAGS}" ./cmd/debug
