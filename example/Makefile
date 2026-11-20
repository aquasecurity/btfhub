# Builds the example's eBPF object and its loader. Needs clang, bpftool, and
# libbpf 1.0 or later (headers and library, found with pkg-config).

CLANG ?= clang
BPFTOOL ?= bpftool
PKG_CONFIG ?= pkg-config

# BTF that vmlinux.h is generated from: the build host's own by default. The
# eBPF object then runs on any kernel whose BTF libbpf can read, from the
# kernel itself or from a BTFhub file.
VMLINUX_BTF ?= /sys/kernel/btf/vmlinux

# architecture names of the kernel, which bpf_tracing.h takes
ARCH := $(shell uname -m | sed -e 's/x86_64/x86/' -e 's/aarch64/arm64/' \
	-e 's/^armv.*/arm/' -e 's/s390x/s390/' -e 's/ppc64.*/powerpc/' \
	-e 's/riscv64/riscv/' -e 's/loongarch64/loongarch/')

LIBBPF_CFLAGS := $(shell $(PKG_CONFIG) --cflags libbpf)
LIBBPF_LDLIBS := $(shell $(PKG_CONFIG) --libs libbpf)

CFLAGS ?= -g -O2 -Wall

.PHONY: all
all: example example.bpf.o

vmlinux.h:
	$(BPFTOOL) btf dump file $(VMLINUX_BTF) format c > $@

example.bpf.o: example.bpf.c example.h vmlinux.h
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(ARCH) $(LIBBPF_CFLAGS) -c $< -o $@

example: example.c example.h
	$(CC) $(CFLAGS) $(LIBBPF_CFLAGS) $< -o $@ $(LDFLAGS) $(LIBBPF_LDLIBS)

.PHONY: clean
clean:
	rm -f example example.bpf.o vmlinux.h
