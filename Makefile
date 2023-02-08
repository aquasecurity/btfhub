.PHONY: all
all: btfhub

#
# make
#

.ONESHELL:
SHELL = /bin/sh

PARALLEL = $(shell $(CMD_GREP) -c ^processor /proc/cpuinfo)
MAKE = make
MAKEFLAGS += --no-print-directory

#
# tools
#

CMD_TR ?= tr
CMD_CUT ?= cut
CMD_AWK ?= awk
CMD_SED ?= sed
CMD_GIT ?= git
CMD_RM ?= rm
CMD_MKDIR ?= mkdir
CMD_TOUCH ?= touch
CMD_GO ?= go
CMD_GREP ?= grep
CMD_CAT ?= cat
CMD_MD5 ?= md5sum

.check_%:
#
	@command -v $* >/dev/null
	if [ $$? -ne 0 ]; then
		echo "missing required tool $*"
		exit 1
	else
		touch $@ # avoid target rebuilds due to inexistent file
	fi

#
# tools version
#

GO_VERSION = $(shell $(CMD_GO) version 2>/dev/null | $(CMD_AWK) '{print $$3}' | $(CMD_SED) 's:go::g' | $(CMD_CUT) -d. -f1,2)
GO_VERSION_MAJ = $(shell echo $(GO_VERSION) | $(CMD_CUT) -d'.' -f1)
GO_VERSION_MIN = $(shell echo $(GO_VERSION) | $(CMD_CUT) -d'.' -f2)

.checkver_$(CMD_GO): \
	| .check_$(CMD_GO)
#
	@if [ ${GO_VERSION_MAJ} -eq 1 ]; then
		if [ ${GO_VERSION_MIN} -lt 18 ]; then
			echo -n "you MUST use golang 1.18 or newer, "
			echo "your current golang version is ${GO_VERSION}"
			exit 1
		fi
	fi
	touch $@

#
# version
#

LAST_GIT_TAG ?= $(shell $(CMD_GIT) describe --tags --match 'v*' 2>/dev/null)
VERSION ?= $(if $(RELEASE_TAG),$(RELEASE_TAG),$(LAST_GIT_TAG))

#
# environment
#

DEBUG ?= 0
UNAME_M := $(shell uname -m)
UNAME_R := $(shell uname -r)

ifeq ($(DEBUG),1)
	GO_DEBUG_FLAG =
else
	GO_DEBUG_FLAG = -w
endif

ifeq ($(UNAME_M),x86_64)
   ARCH = x86_64
   LINUX_ARCH = x86
   GO_ARCH = amd64
endif

ifeq ($(UNAME_M),aarch64)
   ARCH = arm64
   LINUX_ARCH = arm64
   GO_ARCH = arm64
endif

#
# variables
#

PROGRAM ?= btfhub

#
# btfhub tool
#

STATIC ?= 0
GO_TAGS_EBPF = none

ifeq ($(STATIC), 1)
    GO_TAGS := $(GO_TAGS),netgo
endif

GO_ENV =
GO_ENV += GOOS=linux
GO_ENV += CC=$(CMD_CLANG)
GO_ENV += GOARCH=$(GO_ARCH)

SRC_DIRS = ./cmd/
SRC = $(shell find $(SRC_DIRS) -type f -name '*.go' ! -name '*_test.go')

.PHONY: btfhub
btfhub: $(PROGRAM)

$(PROGRAM): \
	$(SRC) \
	| .checkver_$(CMD_GO)
#
	$(GO_ENV) $(CMD_GO) build \
		-tags $(GO_TAGS_EBPF) \
		-ldflags="$(GO_DEBUG_FLAG) \
			-X main.version=\"$(VERSION)\" \
			" \
		-v -o $@ \
		./cmd/btfhub/

.PHONY: clean
clean:
#
	$(CMD_RM) -rf $(PROGRAM)
	$(CMD_RM) -f .check*
