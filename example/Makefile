OUTPUT = ./output
LIBBPF = ./libbpf

LIBBPF_SRC = $(abspath $(LIBBPF)/src)
LIBBPF_OBJ = $(abspath $(OUTPUT)/libbpf.a)

CC = gcc
CLANG = clang

ARCH := $(shell uname -m)
ARCH := $(subst x86_64,amd64,$(ARCH))
GOARCH := $(ARCH)

BPFTOOL = $(shell which bpftool || /bin/false)
BTFFILE = /sys/kernel/btf/vmlinux
DBGVMLINUX = /usr/lib/debug/boot/vmlinux-$(shell uname -r)
GIT = $(shell which git || /bin/false)
VMLINUXH = vmlinux.h

# libbpf

LIBBPF_SRC = $(abspath ./libbpf/src)
LIBBPF_OBJ = $(abspath ./$(OUTPUT)/libbpf.a)
LIBBPF_OBJDIR = $(abspath ./$(OUTPUT)/libbpf)
LIBBPF_DESTDIR = $(abspath ./$(OUTPUT))

CFLAGS = -ggdb -gdwarf -O2 -Wall -fpie -Wno-unused-variable -Wno-unused-function
LDFLAGS =

BPF_CFLAGS_STATIC = "-I$(abspath $(OUTPUT))"
BPF_LDFLAGS_STATIC = "-lelf -lz $(LIBBPF_OBJ)"

CGO_CFLAGS_STATIC = "-I$(abspath $(OUTPUT))"
CGO_LDFLAGS_STATIC = "-lelf -lz $(LIBBPF_OBJ)"
CGO_EXTLDFLAGS_STATIC = '-w -extldflags "-static"'

CGO_CFGLAGS_DYN = "-I. -I/usr/include/"
CGO_LDFLAGS_DYN = "-lelf -lz -lbpf"
CGO_EXTLDFLAGS_DYN = '-w'

## program

.PHONY: $(PROGRAM)
.PHONY: $(PROGRAM).bpf.c

PROGRAM = example

all:
	$(MAKE) -C . $(PROGRAM)-c-static
	$(MAKE) -C . $(PROGRAM)-c-dynamic
	$(MAKE) -C . $(PROGRAM)-static
	$(MAKE) -C . $(PROGRAM)-dynamic

# vmlinux header file

.PHONY: vmlinuxh
vmlinuxh: $(VMLINUXH)

$(VMLINUXH): $(OUTPUT)
ifeq ($(wildcard $(BPFTOOL)),)
	@echo "ERROR: could not find bpftool"
	@exit 1
endif
	@if [ -f $(DBGVMLINUX) ]; then \
		echo "INFO: found dbg kernel, generating $(VMLINUXH) from $(DBGVMLINUX)"; \
		$(BPFTOOL) btf dump file $(DBGVMLINUX) format c > $(VMLINUXH); \
	fi
	@if [ ! -f $(BTFFILE) ] && [ ! -f $(DBGVMLINUX) ]; then \
		echo "ERROR: kernel does not seem to support BTF"; \
		exit 1; \
	fi
	@if [ ! -f $(VMLINUXH) ]; then \
		echo "INFO: generating $(VMLINUXH) from $(BTFFILE)"; \
		$(BPFTOOL) btf dump file $(BTFFILE) format c > $(VMLINUXH); \
	fi

# static libbpf generation for the git submodule

.PHONY: libbpf-static
libbpf-static: $(LIBBPF_OBJ)

$(LIBBPF_OBJ): $(LIBBPF_SRC) $(wildcard $(LIBBPF_SRC)/*.[ch]) | $(OUTPUT)/libbpf
	CC="$(CC)" CFLAGS="$(CFLAGS)" LD_FLAGS="$(LDFLAGS)" \
	   $(MAKE) -C $(LIBBPF_SRC) \
		BUILD_STATIC_ONLY=1 \
		OBJDIR=$(LIBBPF_OBJDIR) \
		DESTDIR=$(LIBBPF_DESTDIR) \
		INCLUDEDIR= LIBDIR= UAPIDIR= prefix= libdir= install

$(LIBBPF_SRC):
ifeq ($(wildcard $@), )
	echo "INFO: updating submodule 'libbpf'"
	$(GIT) submodule update --init --recursive
endif

# output dir

$(OUTPUT):
	mkdir -p $(OUTPUT)

$(OUTPUT)/libbpf:
	mkdir -p $(OUTPUT)/libbpf

## program bpf dependency

$(PROGRAM).bpf.o: $(PROGRAM).bpf.c | vmlinuxh
	$(CLANG) $(CFLAGS) -target bpf -D__TARGET_ARCH_x86 -I. -I$(OUTPUT) -c $< -o $@

## C example

.PHONY: $(PROGRAM).c

.PHONY: $(PROGRAM)-c-static
.PHONY: $(PROGRAM)-c-dynamic

$(PROGRAM)-c-static: libbpf-static
	$(CLANG) $(BPF_CFLAGS_STATIC) -lelf -lz -I. -I$(OUTPUT) $(PROGRAM).c -o $@ $(LIBBPF_OBJ)

$(PROGRAM)-c-dynamic:
	$(CLANG) $(BPF_CFLAGS_STATIC) -lelf -lz -lbpf -I. -I$(OUTPUT) $(PROGRAM).c -o $@

## GO example

.PHONY: $(PROGRAM)-static
.PHONY: $(PROGRAM)-dynamic

$(PROGRAM)-static: libbpf-static | $(PROGRAM).bpf.o
	CC=$(CLANG) \
		CGO_CFLAGS=$(CGO_CFLAGS_STATIC) \
		CGO_LDFLAGS=$(CGO_LDFLAGS_STATIC) \
                GOARCH=$(GOARCH) \
                go build \
                -tags netgo -ldflags $(CGO_EXTLDFLAGS_STATIC) \
                -o $(PROGRAM)-static ./$(PROGRAM).go

$(PROGRAM)-dynamic: $(PROGRAM).bpf.o
	CC=$(CLANG) \
		CGO_CFLAGS=$(CGO_CFLAGS_DYN) \
		CGO_LDFLAGS=$(CGO_LDFLAGS_DYN) \
                GOARCH=$(GOARCH) \
                go build \
                -tags netgo -ldflags $(CGO_EXTLDFLAGS_DYN) \
                -o $(PROGRAM)-dynamic ./$(PROGRAM).go

## clean

clean:
	$(MAKE) -C $(LIBBPF_SRC) clean
	rm -rf $(OUTPUT)
	rm -rf $(VMLINUXH)
	rm -rf $(PROGRAM)-*static $(PROGRAM)-*dynamic
	rm -rf $(PROGRAM).bpf.o $(PROGRAM).o
