// SPDX-License-Identifier: GPL-2.0
/*
 * Reports each openat() with the caller's pid, parent pid and command. The
 * parent pid comes from task_struct, whose layout differs between kernels:
 * libbpf finds its offsets in the running kernel's BTF, or in the BTF file
 * given to the loader (one from BTFhub) when the kernel has none.
 */
#include "vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

#include "example.h"

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_openat")
int sys_enter_openat(struct trace_event_raw_sys_enter *ctx)
{
	struct task_struct *task = (struct task_struct *)bpf_get_current_task();
	struct event e = {};

	e.pid = bpf_get_current_pid_tgid() >> 32;
	e.ppid = BPF_CORE_READ(task, real_parent, tgid);
	bpf_get_current_comm(&e.comm, sizeof(e.comm));
	/*
	 * bpf_probe_read_user_str() came in Linux 5.5. On older kernels, which
	 * are most of those without BTF, libbpf replaces it with
	 * bpf_probe_read_str(), which reads user memory there.
	 */
	bpf_probe_read_user_str(&e.filename, sizeof(e.filename), (const char *)ctx->args[1]);

	bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &e, sizeof(e));

	return 0;
}

char LICENSE[] SEC("license") = "GPL";
