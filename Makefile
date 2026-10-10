SHELL := /bin/bash
GO ?= go
BIN := bin/homeapps

## The patch number is the commit count; scripts/version.sh works it out and
## the linker stamps it in. Empty (no git, a shallow clone) leaves the Go
## default of 0, which reads as "unstamped build".
VERSION_PKG := github.com/chinmay28/homeapps/server/internal/version
PATCH := $(shell scripts/version.sh --patch 2>/dev/null)
LDFLAGS := -s -w $(if $(PATCH),-X $(VERSION_PKG).Patch=$(PATCH))

.PHONY: build test test-installer test-go test-web test-android-core vet fmt run android version bump-version clean

## build: the single static binary, web client embedded
build:
	cd server && CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o ../$(BIN) ./cmd/homeapps

## test: everything that runs without an Android SDK
test: test-go test-web test-android-core

## test-installer: run scripts/quickstart.sh for real in a sandbox (needs root)
test-installer:
	sudo env "PATH=$$PATH" scripts/test-quickstart.sh

test-go:
	cd server && $(GO) test -race ./...

## test-web: the client's pure view logic (node only, no npm install)
test-web:
	node --test web/*.test.mjs

## test-android-core: the shell's pure Kotlin logic (JDK 17 + gradle)
test-android-core:
	cd apps/android/core && ../gradlew test

vet:
	cd server && $(GO) vet ./... && test -z "$$(gofmt -l .)"

fmt:
	cd server && gofmt -w .

## run: serve against a HostMan (HOMEAPPS_HOSTMAN, default http://127.0.0.1:8899),
## web client read from disk so edits show on reload
run:
	cd server && $(GO) run ./cmd/homeapps serve --web-dist internal/web/assets

## android: debug APK (needs the Android SDK); see apps/android/README.md
android:
	scripts/build-android.sh assembleDebug

version:
	@scripts/version.sh

## bump-version: move the version line to this month (UTC); the first commit
## on a branch does this
bump-version:
	@scripts/version.sh --bump

clean:
	rm -rf bin apps/android/app/build apps/android/core/build
