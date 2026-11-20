// SPDX-License-Identifier: GPL-2.0
/*
 * Loads example.bpf.o and prints its events. With -b, libbpf takes the kernel
 * types from the given BTF file instead of /sys/kernel/btf/vmlinux, which is
 * how a kernel without embedded BTF can run a CO-RE program.
 */
#include <errno.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include <bpf/libbpf.h>

#include "example.h"

static volatile sig_atomic_t exiting;
static bool verbose;

static void sig_handler(int sig)
{
	exiting = 1;
}

static int print_fn(enum libbpf_print_level level, const char *format, va_list args)
{
	if (level == LIBBPF_DEBUG && !verbose)
		return 0;

	return vfprintf(stderr, format, args);
}

static void handle_event(void *ctx, int cpu, void *data, __u32 size)
{
	const struct event *e = data;

	printf("%-16s pid %-7u ppid %-7u %s\n", e->comm, e->pid, e->ppid, e->filename);
}

static void handle_lost(void *ctx, int cpu, __u64 count)
{
	fprintf(stderr, "lost %llu events on CPU %d\n", (unsigned long long)count, cpu);
}

static void usage(const char *prog)
{
	fprintf(stderr,
		"Usage: %s [-b <btf file>] [-o <bpf object>] [-v]\n"
		"\n"
		"  -b  BTF file describing the running kernel, for kernels without\n"
		"      /sys/kernel/btf/vmlinux (default: the kernel's own BTF)\n"
		"  -o  eBPF object to load (default: example.bpf.o)\n"
		"  -v  print libbpf debug messages\n",
		prog);
}

int main(int argc, char **argv)
{
	const char *obj_path = "example.bpf.o";
	const char *btf_path = NULL;
	struct perf_buffer *pb = NULL;
	struct bpf_program *prog;
	struct bpf_link *link = NULL;
	struct bpf_object *obj;
	struct bpf_map *map;
	int opt, err;

	while ((opt = getopt(argc, argv, "b:o:vh")) != -1) {
		switch (opt) {
		case 'b':
			btf_path = optarg;
			break;
		case 'o':
			obj_path = optarg;
			break;
		case 'v':
			verbose = true;
			break;
		default:
			usage(argv[0]);
			return opt == 'h' ? 0 : 1;
		}
	}

	libbpf_set_print(print_fn);

	LIBBPF_OPTS(bpf_object_open_opts, opts, .btf_custom_path = btf_path);

	obj = bpf_object__open_file(obj_path, &opts);
	if (!obj) {
		fprintf(stderr, "opening %s: %s\n", obj_path, strerror(errno));
		return 1;
	}

	err = bpf_object__load(obj);
	if (err) {
		fprintf(stderr, "loading %s: %s\n", obj_path, strerror(-err));
		if (!btf_path)
			fprintf(stderr, "if this kernel has no BTF, pass one with -b\n");
		goto cleanup;
	}

	prog = bpf_object__find_program_by_name(obj, "sys_enter_openat");
	link = bpf_program__attach(prog);
	if (!link) {
		err = -errno;
		fprintf(stderr, "attaching: %s\n", strerror(errno));
		goto cleanup;
	}

	map = bpf_object__find_map_by_name(obj, "events");
	pb = perf_buffer__new(bpf_map__fd(map), 16, handle_event, handle_lost, NULL, NULL);
	if (!pb) {
		err = -errno;
		fprintf(stderr, "creating perf buffer: %s\n", strerror(errno));
		goto cleanup;
	}

	signal(SIGINT, sig_handler);
	signal(SIGTERM, sig_handler);

	printf("Tracing openat() with %s BTF, Ctrl-C to stop.\n",
	       btf_path ? btf_path : "the kernel's");

	while (!exiting) {
		err = perf_buffer__poll(pb, 100);
		if (err < 0 && err != -EINTR) {
			fprintf(stderr, "polling perf buffer: %s\n", strerror(-err));
			break;
		}
		err = 0;
	}

cleanup:
	perf_buffer__free(pb);
	bpf_link__destroy(link);
	bpf_object__close(obj);

	return err ? 1 : 0;
}
