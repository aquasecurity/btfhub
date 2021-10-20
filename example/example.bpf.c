#include "vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#include "example.h"

struct {
	__uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(u32));
	__uint(value_size, sizeof(u32));
} events SEC(".maps");

static __always_inline int dealwithit(void *args)
{
	struct events_t evdata = {};
	struct task_struct *task = (void *) bpf_get_current_task();

	u64 id1 = bpf_get_current_pid_tgid();
	u64 id2 = bpf_get_current_uid_gid();
	u32 tgid = id1 >> 32, pid = id1;
	u32 gid = id2 >> 32, uid = id2;

	evdata.pid = tgid;
        evdata.uid = uid;
        evdata.gid = gid;

	bpf_probe_read_kernel(&evdata.loginuid, sizeof(int), &task->loginuid.val);
	bpf_probe_read_kernel_str(&evdata.comm, 16, task->comm);

	bpf_perf_event_output(args, &events, 0xffffffffULL, &evdata, sizeof(evdata));

	return 0;
}

SEC("kprobe/ksys_sync")
int BPF_KPROBE(ksys_sync)
{
	return dealwithit(ctx);
}

SEC("tracepoint/syscalls/sys_enter_sync")
int tracepoint__sys_enter_sync(struct trace_event_raw_sys_enter *args)
{
	return dealwithit(args);
}

char LICENSE[] SEC("license") = "GPL";
