# (c) Copyright 2017 Hewlett Packard Enterprise Development LP
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION = 1.1.0

# Where our code lives
PKG_PATH = ./common/
CMD_PATH = ./cmd/

# This is the last 8 char of the commit id we're building from
COMMIT = $(shell git rev-parse HEAD| cut -b-8)

# The version of make for OSX doesn't allow us to export, so
# we add these variables to the env in each invocation.
GOENV = GOPATH=$(GOPATH) PATH=$$PATH:$(GOPATH)/bin

# Our target binary is for Linux.  To build an exec for your local (non-linux)
# machine, use go build directly.
ifndef GOOS
TEST_ENV  = GOOS=linux GOARCH=amd64
else
TEST_ENV  = GOOS=$(GOOS) GOARCH=amd64
endif
BUILD_ENV = GOOS=linux GOARCH=amd64 CGO_ENABLED=0

# Add the version and hg commit id to the binary in the form of variables.
LD_FLAGS = '-X main.Version=$(VERSION) -X main.Commit=$(COMMIT)'

# gometalinter allows us to have a single target that runs multiple linters in
# the same fashion.  This variable controls which linters are used.
LINTER_FLAGS = --vendor --disable-all --enable=vet --enable=vetshadow --enable=golint --enable=ineffassign --enable=goconst --enable=deadcode --enable=dupl --enable=varcheck --enable=gocyclo --enable=misspell --deadline=300s

# list of packages
PACKAGE_LIST =   $(shell export $(GOENV) && go list ./$(PKG_PATH)...| grep -v vendor)
# list of commands
COMMAND_LIST =   $(shell export $(GOENV) && go list ./$(CMD_PATH)...)

# prefixes to make things pretty
A1 = $(shell printf "»")
A2 = $(shell printf "»»")
A3 = $(shell printf "»»»")

.PHONY: help
help:
	@echo "Targets:"
	@echo "    tools          - Download and install go tooling required to build."
	@echo "    vendor         - Download dependancies."
	@echo "    lint           - Static analysis of source code.  Note that this must pass in order to build."
	@echo "    test           - Run unit tests."
	@echo "    clean          - Remove binaries."
	@echo "    debug          - Display make's view of the world."
	@echo "    dory           - Build dory (FlexVolume driver)."
	@echo "    doryd          - Build doryd (Provisioner)."
	@echo "    doryd_docker   - Build doryd (Provisioner) docker image."

.PHONY: debug
debug:
	@echo "Debug:"
	@echo "    packages:  $(PACKAGE_LIST)"
	@echo "    commands:  $(COMMAND_LIST)"
	@echo "    COMMIT:    $(COMMIT)"
	@echo "    GOPATH:    $(GOPATH)"
	@echo "    LD_FLAGS:  $(LD_FLAGS)"
	@echo "    BUILD_ENV: $(BUILD_ENV)"
	@echo "    GOENV:     $(GOENV)"

tools: ; $(info $(A1) tools)
	@echo "$(A2) get gometalinter"
	export $(GOENV) && go get -u github.com/alecthomas/gometalinter
	@echo "$(A2) install gometalinter"
	export $(GOENV) && gometalinter --install
	@echo "$(A2) get glide"
	export $(GOENV) && go get -u github.com/Masterminds/glide
	export $(GOENV) && go install github.com/Masterminds/glide

vendor: tools; $(info $(A1) vendor)
	@echo "$(A2) glide install"
	export $(GOENV) && glide install

.PHONY: lint
lint: ; $(info $(A1) lint)
	@echo "$(A2) lint $(CMD_PATH)"
	export $(GOENV) $(BUILD_ENV); gometalinter $(LINTER_FLAGS) $(CMD_PATH)...
	@echo "$(A2) lint $(PKG_PATH)"
	export $(GOENV) $(BUILD_ENV); gometalinter $(LINTER_FLAGS) $(PKG_PATH)...

.PHONY: clean
clean: ; $(info $(A1) clean)
	@echo "$(A2) remove dory"
	@rm -f dory
	@rm -f dory.sha256sum
	@echo "$(A2) remove doryd"
	@rm -f doryd
	@rm -f doryd.sha256sum

.PHONY: test
test: ; $(info $(A1) test)
	@echo "$(A2) Package unit tests"
	for pkg in $(PACKAGE_LIST); do echo "»»» Testing $$pkg:" && export $(GOENV) $(TEST_ENV) && go test -cover $$pkg; done
	@echo "$(A2) Command unit tests"
	for cmd in $(COMMAND_LIST); do echo "»»» Testing $$cmd:" && export $(GOENV) $(TEST_ENV) && go test -cover $$cmd; done

dory: lint; $(info $(A1) dory)
	@echo "$(A2) build dory"
	export $(GOENV) $(BUILD_ENV) && go build -ldflags $(LD_FLAGS) $(CMD_PATH)dory/dory.go
	@echo "$(A2) sha256sum dory"
	sha256sum  dory > dory.sha256sum
	@cat dory.sha256sum

doryd: lint; $(info $(A1) dory)
	@echo "$(A2) build doryd"
	export $(GOENV) $(BUILD_ENV) && go build -ldflags $(LD_FLAGS) $(CMD_PATH)doryd/doryd.go
	@echo "$(A2) sha256sum doryd"
	sha256sum  doryd > doryd.sha256sum
	@cat doryd.sha256sum

.PHONY: doryd_docker
doryd_docker: doryd; $(info $(A1) doryd_docker)
	@echo "$(A2) rm current doryd image"
	-docker image rm kube-storage-controller-dory:edge
	@echo "$(A2) build doryd image"
	docker build -t kube-storage-controller-dory:edge -f ./build/docker/doryd/Dockerfile .
