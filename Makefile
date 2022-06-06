SHELL = /bin/sh

# Go and compilation related variables
BUILD_DIR ?= out

.SUFFIXES: .go

CC = clang
PREFIX ?= ${XDG_BIN_HOME}
INSTALL=install

.PHONY: all
all: build

$(BUILD_DIR):
	mkdir -p $@

$(BUILD_DIR)/vz: $(BUILD_DIR) cmd/vz/main.go $(wildcard internal/vz/*.go) $(wildcard internal/net/*.go)
	go build -o $@ cmd/vz/main.go

$(BUILD_DIR)/docker-machine-driver-vz: $(BUILD_DIR) cmd/docker-machine-driver-vz/main.go $(wildcard internal/driver/*.go) $(wildcard internal/vz/*.go) $(wildcard internal/net/*.go)
	go build -o $@ cmd/docker-machine-driver-vz/main.go

.PHONY: codesign
codesign: $(BUILD_DIR)/vz
	codesign --sign - -i com.github.brholstein.vz --entitlements Info.plist --force "$(BUILD_DIR)/vz"

.PHONY: build
build: $(BUILD_DIR)/vz $(BUILD_DIR)/docker-machine-driver-vz codesign

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: install
install: build
	$(INSTALL) -t "${PREFIX}" $(BUILD_DIR)/vz $(BUILD_DIR)/docker-machine-driver-vz
	#cp launchctl.plist ~/Library/LaunchAgents/com.github.brholstein.vz.plist
	#launchctl load ~/Library/LaunchAgents/com.github.brholstein.vz.plist

.PHONY: uninstall
uninstall:
	#launchctl unload ~/Library/LaunchAgents/com.github.brholstein.vz.plist
	#rm ~/Library/LaunchAgents/com.github.brholstein.vz.plist
	rm "${PREFIX}/vz" "${PREFIX}/docker-machine-driver-vz"
