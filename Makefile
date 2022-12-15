# ==============================================================================
# Colors

BLACK        := $(shell printf "\033[30m")
BLACK_BOLD   := $(shell printf "\033[30;1m")
RED          := $(shell printf "\033[31m")
RED_BOLD     := $(shell printf "\033[31;1m")
GREEN        := $(shell printf "\033[32m")
GREEN_BOLD   := $(shell printf "\033[32;1m")
YELLOW       := $(shell printf "\033[33m")
YELLOW_BOLD  := $(shell printf "\033[33;1m")
BLUE         := $(shell printf "\033[34m")
BLUE_BOLD    := $(shell printf "\033[34;1m")
MAGENTA      := $(shell printf "\033[35m")
MAGENTA_BOLD := $(shell printf "\033[35;1m")
CYAN         := $(shell printf "\033[36m")
CYAN_BOLD    := $(shell printf "\033[36;1m")
WHITE        := $(shell printf "\033[37m")
WHITE_BOLD   := $(shell printf "\033[37;1m")
CNone        := $(shell printf "\033[0m")

# ==============================================================================
# Logger

TIME_LONG	= `date +%Y-%m-%d' '%H:%M:%S`
TIME_SHORT	= `date +%H:%M:%S`
TIME		= $(TIME_SHORT)

INFO	= echo ${TIME} ${BLUE}[ .. ]${CNone}
WARN	= echo ${TIME} ${YELLOW}[WARN]${CNone}
ERR		= echo ${TIME} ${RED}[FAIL]${CNone}
OK		= echo ${TIME} ${GREEN}[ OK ]${CNone}
FAIL	= (echo ${TIME} ${RED}[FAIL]${CNone} && false)

# ====================================================================================
# Platform options

# all supported platforms we build for this can be set to other platforms if desired
# we use the golang os and arch names for convenience
PLATFORMS ?= darwin_amd64 darwin_arm64 windows_amd64 linux_amd64 linux_arm64

# Set the host's OS. Only linux and darwin supported for now
HOSTOS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ifeq ($(filter darwin linux,$(HOSTOS)),)
$(error build only supported on linux and darwin host currently)
endif

# Set the host's arch.
HOSTARCH := $(shell uname -m)

# If SAFEHOSTARCH and TARGETARCH have not been defined yet, use HOST
ifeq ($(origin SAFEHOSTARCH),undefined)
SAFEHOSTARCH := $(HOSTARCH)
endif

# Automatically translate x86_64 to amd64
ifeq ($(HOSTARCH),x86_64)
SAFEHOSTARCH := amd64
endif

# Automatically translate aarch64 to arm64
ifeq ($(HOSTARCH),aarch64)
SAFEHOSTARCH := arm64
endif

ifeq ($(filter amd64 arm64 ,$(SAFEHOSTARCH)),)
$(error build only supported on amd64, arm64 and ppc64le host currently)
endif

# Standardize Host Platform variables
HOST_PLATFORM := $(HOSTOS)_$(HOSTARCH)
SAFEHOSTPLATFORM := $(HOSTOS)-$(SAFEHOSTARCH)
SAFEHOST_PLATFORM := $(HOSTOS)_$(SAFEHOSTARCH)

# ==============================================================================
# Common variables

SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))

ifeq ($(origin ROOT_DIR),undefined)
ROOT_DIR := $(abspath $(SELF_DIR))
endif

CACHE_DIR ?= $(ROOT_DIR)/.cache
TOOLS_HOST_DIR ?= $(CACHE_DIR)/tools/$(SAFEHOST_PLATFORM)

GO_LINT_DIR := $(abspath $(OUTPUT_DIR)/lint)
GO_LINT_OUTPUT := $(GO_LINT_DIR)/$(PLATFORM)

GITHUB_URL ?= https://github.com

ifneq ($(CI),)
RUNNING_IN_CI := true
endif

# ==============================================================================
# Tools

GOLANGCILINT_VERSION ?= 1.49.0
GOLANGCILINT := $(TOOLS_HOST_DIR)/golangci-lint-v$(GOLANGCILINT_VERSION)
GOLANGCILINT_TEMP := $(TOOLS_HOST_DIR)/tmp-golangci-lint

$(GOLANGCILINT):
	@$(INFO) installing golangci-lint-v$(GOLANGCILINT_VERSION) $(SAFEHOSTPLATFORM)
	@mkdir -p $(GOLANGCILINT_TEMP) || $(FAIL)
	@curl -fsSL $(GITHUB_URL)/golangci/golangci-lint/releases/download/v$(GOLANGCILINT_VERSION)/golangci-lint-$(GOLANGCILINT_VERSION)-$(SAFEHOSTPLATFORM).tar.gz | tar -xz --strip-components=1 -C $(GOLANGCILINT_TEMP) || $(FAIL)
	@mv $(GOLANGCILINT_TEMP)/golangci-lint $(GOLANGCILINT) || $(FAIL)
	@rm -fr $(GOLANGCILINT_TEMP)
	@$(OK) installing golangci-lint-v$(GOLANGCILINT_VERSION) $(SAFEHOSTPLATFORM)

GORELEASER_VERSION ?= 1.11.2
GORELEASER := $(TOOLS_HOST_DIR)/goreleaser-v$(GORELEASER_VERSION)
GORELEASER_TEMP := $(TOOLS_HOST_DIR)/tmp-goreleaser
ifeq ($(SAFEHOSTARCH),amd64)
GORELEASER_ARCH := x86_64
else
GORELEASER_ARCH := $(SAFEHOSTARCH)
endif

$(GORELEASER):
	@$(INFO) installing goreleaser-v$(GORELEASER_VERSION) $(SAFEHOSTPLATFORM)
	@mkdir -p $(GORELEASER_TEMP) || $(FAIL)
	@curl -fsSL $(GITHUB_URL)/goreleaser/goreleaser/releases/download/v$(GORELEASER_VERSION)/goreleaser_$(HOSTOS)_$(GORELEASER_ARCH).tar.gz | tar -xz -C $(GORELEASER_TEMP) || $(FAIL)
	@mv $(GORELEASER_TEMP)/goreleaser $(GORELEASER) || $(FAIL)
	@rm -fr $(GORELEASER_TEMP)
	@$(OK) installing goreleaser-v$(GORELEASER_VERSION) $(SAFEHOSTPLATFORM)

# ==============================================================================
# Targets

build: $(GORELEASER)
	@$(INFO) Building snapshot for host platform
	@$(GORELEASER) build --rm-dist --snapshot --single-target || $(FAIL)
	@$(OK) Building snapshot for host platform

build.all: $(GORELEASER)
	@$(INFO) Building binaries for all platforms
	@$(GORELEASER) build --rm-dist --snapshot || $(FAIL)
	@$(OK) Building binaries for all platforms

release: $(GORELEASER)
	@$(INFO) Building release for all platforms
	@$(GORELEASER) release --rm-dist
	@$(OK) Building release for all platforms

ifeq ($(RUNNING_IN_CI),true)
# The timeout is increased to 10m, to accommodate CI machines with low resources.
GO_LINT_ARGS += --timeout 10m0s
endif

lint: $(GOLANGCILINT)
	@$(INFO) golangci-lint
	@$(GOLANGCILINT) run $(GO_LINT_ARGS) || $(FAIL)
	@$(OK) golangci-lint
