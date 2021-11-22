#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <signal.h>
#include <getopt.h>
#include <unistd.h>
#include <time.h>
#include <pwd.h>
#include <fcntl.h>
#include <syslog.h>
#include <errno.h>
#include <sys/types.h>
#include <sys/time.h>
#include <sys/resource.h>
#include <sys/stat.h>
#include <sys/types.h>

#include <sys/types.h>
#include <sys/time.h>
#include <sys/resource.h>

#include <arpa/inet.h>
#include <netinet/in.h>

#include <linux/perf_event.h>
#include <linux/hw_breakpoint.h>

#include <bpf/libbpf.h>
#include <bpf/bpf.h>

#include "example.h"

static int bpfverbose = 0;
static volatile bool exiting;

char *get_currtime(void)
{
	char *datetime = malloc(100);
	time_t t = time(NULL);
	struct tm *tmp;

	memset(datetime, 0, 100);

	if ((tmp = localtime(&t)) == NULL)
		exiterr("could not get localtime");

	if ((strftime(datetime, 100, "%Y/%m/%d_%H:%M", tmp)) == 0)
		exiterr("could not parse localtime");

	return datetime;
}

static int get_pid_max(void)
{
	FILE *f;
	int pid_max = 0;

	if ((f = fopen("/proc/sys/kernel/pid_max", "r")) == NULL)
		exiterr("failed to open proc_sys pid_max");

	if (fscanf(f, "%d\n", &pid_max) != 1)
		exiterr("failed to read proc_sys pid_max");

	fclose(f);

	return pid_max;
}

int bump_memlock_rlimit(void)
{
	struct rlimit rlim_new = {
		.rlim_cur = RLIM_INFINITY,
		.rlim_max = RLIM_INFINITY,
	};

	return setrlimit(RLIMIT_MEMLOCK, &rlim_new);
}

static int output(context_t *e)
{
	char *currtime = get_currtime();

	wrapout("(%s) %s (pid: %u) opened: %s (flags: 0x%08llx, mode: 0x%08llx)",
			currtime, e->comm, e->pid, e->filename, e->flags, e->mode);

	free(currtime);

	return 0;
}

int libbpf_print_fn(enum libbpf_print_level level, const char *format, va_list args)
{
	if (level == LIBBPF_INFO && !bpfverbose)
		return 0;

	return vfprintf(stderr, format, args);
}

int usage(int argc, char **argv)
{
	fprintf(stdout,
		"\n"
		"Syntax: %s [options]\n"
		"\n"
		"\t[options]:\n"
		"\n"
		"\t-v: bpf verbose mode\n"
		"\n"
		"Check https://rafaeldtinoco.github.io/portablebpf/\n"
		"\n",
		argv[0]);

	exit(0);
}

void handle_event(void *ctx, int cpu, void *evdata, __u32 data_sz)
{
	output((context_t *) evdata);
}

void handle_lost_events(void *ctx, int cpu, __u64 lost_cnt)
{
	fprintf(stderr, "lost %llu events on CPU #%d\n", lost_cnt, cpu);
}

void trap(int what)
{
	exiting = 1;
}

int main(int argc, char **argv)
{
	char *btf_file;
	int map_fd, opt, pid_max, err = 0;

	struct bpf_object *obj = NULL;
	struct bpf_program *program = NULL;
	struct bpf_map *map = NULL;
	struct bpf_link *link = NULL;
	struct perf_buffer *pb = NULL;

	struct bpf_object_open_opts openopts = {};
	struct perf_buffer_opts pb_opts = {};

	while ((opt = getopt(argc, argv, "hvd")) != -1) {
		switch(opt) {
		case 'v':
			bpfverbose = 1;
			break;
		case 'h':
		default:
			usage(argc, argv);
		}
	}

	if ((pid_max = get_pid_max()) < 0)
		exiterr("failed to get pid_max");

	fprintf(stdout, "Foreground mode...<Ctrl-C> or or SIG_TERM to end it.\n");

	signal(SIGINT, trap);
	signal(SIGTERM, trap);

	umask(022);

	libbpf_set_print(libbpf_print_fn);

	if ((err = bump_memlock_rlimit()))
		exiterr("failed to increase rlimit: %d", err);

	// bpf object open options

	openopts.sz = sizeof(struct bpf_object_open_opts);
	btf_file = getenv("EXAMPLE_BTF_FILE");
	if (btf_file != NULL)
		openopts.btf_custom_path = strdup(btf_file);

	// create bpf object from file

	obj = bpf_object__open_file("example.bpf.o", &openopts);
	err = libbpf_get_error(obj);
	if (err) {
		fprintf(stderr, "ERROR: failed to open bpf object file: %d\n", err);
		goto cleanup;
	}

	// create bpf programs from bpf object

	program = bpf_object__find_program_by_name(obj, "sys_enter_openat");
	err = libbpf_get_error(program);
	if (err) {
		fprintf(stderr, "ERROR: failed to find ebpf program: %d\n", err);
		goto cleanup;
	}

	// enable/disable program autoload

	bpf_program__set_autoload(program, 1);

	// load program(s)

	err = bpf_object__load(obj);
	if (err) {
		fprintf(stderr, "ERROR: failed to load bpf object file: %d\n", err);
		goto cleanup;
	}

	// create maps from ebpf oject

	map = bpf_object__find_map_by_name(obj, "events");
	err = libbpf_get_error(map);
	if (err) {
		fprintf(stderr, "ERROR: failed to find events map: %d\n", err);
		goto cleanup;
	}
	map_fd = bpf_map__fd(map);

	// links

	link = bpf_program__attach(program);
	err = libbpf_get_error(link);
	if (err) {
		fprintf(stderr, "ERROR: failed to attach program to kprobe: %d\n", err);
		goto cleanup;
	}

	// events

	pb_opts.sample_cb = handle_event;
	pb_opts.lost_cb = handle_lost_events;

	pb = perf_buffer__new(map_fd, 16, &pb_opts);
	err = libbpf_get_error(pb);
	if (err) {
		fprintf(stderr, "ERROR: failed to create perf event: %d\n", err);
		goto cleanup;
	}

	printf("Tracing... Hit Ctrl-C to end.\n");
	while (1) {
		err = perf_buffer__poll(pb, 100);
		if (err < 0 || exiting)
			break;
	}

cleanup:
	if (pb)
		perf_buffer__free(pb);
	if (obj)
		bpf_object__close(obj);

	return 0;
}
