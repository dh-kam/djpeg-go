GO ?= go
GO_PKG ?= ./cmd/djpeg
OS_LIST ?= linux darwin windows
ARCH_LIST ?= amd64 arm64
BUILD_VARIANTS ?= debug release

OUTPUT_DIR ?= dist
APP_NAME ?= djpeg
SEMVER ?= 0.1.0
UPSTREAM_VERSION ?= 9f
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG ?= github.com/dh-kam/djpeg-go/internal/djpegcli

LD_VERSION_FLAGS := -X '$(VERSION_PKG).Version=$(VERSION)' -X '$(VERSION_PKG).Commit=$(COMMIT)' -X '$(VERSION_PKG).Date=$(DATE)' -X '$(VERSION_PKG).UpstreamVersion=$(UPSTREAM_VERSION)'
GO_DEBUG_FLAGS ?= -trimpath -gcflags="all=-N -l" -ldflags="$(LD_VERSION_FLAGS)"
GO_RELEASE_FLAGS ?= -trimpath -ldflags="-s -w $(LD_VERSION_FLAGS)"

HOST_OS ?= $(shell $(GO) env GOOS)
HOST_ARCH ?= $(shell $(GO) env GOARCH)

rwildcard = $(wildcard $(1)/$(2)) $(foreach d,$(wildcard $(1)/*),$(call rwildcard,$$d,$(2)))
GO_FILES := $(call rwildcard,.,*.go)

artifact = $(OUTPUT_DIR)/$(APP_NAME)-$(1)-$(2)-$(3)$(if $(filter windows,$(1)),.exe,)

define build_target
$(call artifact,$(1),$(2),$(3)): $(GO_FILES) | $(OUTPUT_DIR)
	@mkdir -p $(OUTPUT_DIR)
	GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(if $(filter release,$(3)),0,1) \
	$(GO) build \
	$(if $(filter release,$(3)),$(GO_RELEASE_FLAGS),$(GO_DEBUG_FLAGS)) \
	-o $$@ $(GO_PKG)
endef

define token1
$(word 1,$(subst -, ,$(1)))
endef

define token2
$(word 2,$(subst -, ,$(1)))
endef

define token3
$(word 3,$(subst -, ,$(1)))
endef

OS_ARCH_PAIRS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(os)-$(arch)))
OS_VARIANT_PAIRS := $(foreach os,$(OS_LIST),$(foreach var,$(BUILD_VARIANTS),$(os)-$(var)))
ARCH_VARIANT_PAIRS := $(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(arch)-$(var)))
FULL_SELECTOR_TARGETS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(os)-$(arch)-$(var))))

FULL_TARGETS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(os),$(arch),$(var)))))
RELEASE_TARGETS := $(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(call artifact,$(os),$(arch),release)))
LOCAL_TARGET := $(call artifact,$(HOST_OS),$(HOST_ARCH),debug)

define all_for_os
$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(1),$(arch),$(var))))
endef

define all_for_arch
$(foreach os,$(OS_LIST),$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(os),$(1),$(var))))
endef

define all_for_variant
$(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(call artifact,$(os),$(arch),$(1))))
endef

define all_for_os_arch
$(foreach var,$(BUILD_VARIANTS),$(call artifact,$(1),$(2),$(var)))
endef

define all_for_os_variant
$(foreach arch,$(ARCH_LIST),$(call artifact,$(1),$(arch),$(2)))
endef

define all_for_arch_variant
$(foreach os,$(OS_LIST),$(call artifact,$(os),$(1),$(2)))
endef

.PHONY: all build clean test vet bump-up help $(OS_LIST) $(ARCH_LIST) $(BUILD_VARIANTS) $(OS_ARCH_PAIRS) $(OS_VARIANT_PAIRS) $(ARCH_VARIANT_PAIRS) $(FULL_SELECTOR_TARGETS)

all: $(FULL_TARGETS)

build: $(LOCAL_TARGET)

clean:
	@rm -rf $(OUTPUT_DIR)

test:
	$(GO) test ./... -count=1

vet:
	$(GO) vet ./...

bump-up:
	@scripts/bump-up.sh --semver "$(SEMVER)" --upstream-version "$(UPSTREAM_VERSION)"

$(OS_LIST):
	@$(MAKE) $(call all_for_os,$(@F))

$(ARCH_LIST):
	@$(MAKE) $(call all_for_arch,$(@F))

$(BUILD_VARIANTS):
	@$(MAKE) $(call all_for_variant,$(@F))

$(OS_ARCH_PAIRS):
	@$(MAKE) $(call all_for_os_arch,$(call token1,$(@F)),$(call token2,$(@F)))

$(OS_VARIANT_PAIRS):
	@$(MAKE) $(call all_for_os_variant,$(call token1,$(@F)),$(call token2,$(@F)))

$(ARCH_VARIANT_PAIRS):
	@$(MAKE) $(call all_for_arch_variant,$(call token1,$(@F)),$(call token2,$(@F)))

$(FULL_SELECTOR_TARGETS):
	@$(MAKE) $(call artifact,$(call token1,$(@F)),$(call token2,$(@F)),$(call token3,$(@F)))

$(OUTPUT_DIR):
	@mkdir -p $@

$(foreach os,$(OS_LIST),$(foreach arch,$(ARCH_LIST),$(foreach var,$(BUILD_VARIANTS),$(eval $(call build_target,$(os),$(arch),$(var))))))

help:
	@echo "make build: build $(APP_NAME) for the host platform"
	@echo "make test: run go tests"
	@echo "make vet: run go vet"
	@echo "make all: build all OS/arch/variant targets"
	@echo "make release VERSION=<tag>: build all release targets"
	@echo "make bump-up SEMVER=X.Y.Z UPSTREAM_VERSION=9f: print next release tag"
	@echo "make clean: remove $(OUTPUT_DIR)"
	@echo "make <os>|<arch>|<variant>|<os>-<arch>|<os>-<variant>|<arch>-<variant>|<os>-<arch>-<variant>"

%:
	@echo "Unknown target '$@'"
	@echo "Run 'make help' for valid patterns"
	@exit 1
