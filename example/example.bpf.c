#include "vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#include "example.h"

#define MAX_PERCPU_BUFSIZE	(1 << 15)	// value is set by the kernel as an upper bound
#define O_DIRECTORY		00200000

#define READ_KERN(ptr) ({ typeof(ptr) _val;				\
	__builtin_memset(&_val, 0, sizeof(_val));			\
	bpf_core_read(&_val, sizeof(_val), &ptr);			\
	_val;								\
	})

#define BPF_MAP(_name, _type, _key_type, _value_type, _max_entries)	\
struct bpf_map_def SEC("maps") _name = {				\
	.type = _type,							\
	.key_size = sizeof(_key_type),					\
	.value_size = sizeof(_value_type),				\
	.max_entries = _max_entries,					\
};

// maps macros

#define BPF_PERF_OUTPUT(_name) \
	BPF_MAP(_name, BPF_MAP_TYPE_PERF_EVENT_ARRAY, int, __u32, 1024)

// maps

BPF_PERF_OUTPUT(events);

// helper functions

static __always_inline u32 get_task_ppid(struct task_struct *task)
{
	struct task_struct *parent = READ_KERN(task->real_parent);
	return READ_KERN(parent->pid);
}

// inline helper function

static __always_inline int init_context(context_t *context, struct task_struct *task) {

	u64 id = bpf_get_current_pid_tgid();
	context->tid = id;
	context->pid = id >> 32;
	context->ppid = get_task_ppid(task);
	context->uid = bpf_get_current_uid_gid();
	bpf_get_current_comm(&context->comm, sizeof(context->comm));
	context->ts = bpf_ktime_get_ns();

	return 0;
}

// kprobe function (eBPF program: do_sys_openat2)

SEC("kprobe/do_sys_openat2")
int BPF_KPROBE(do_sys_openat2)
{
	context_t context = {};
	char *fn_ptr;
	struct open_how *ohow_ptr;

	init_context(&context, (struct task_struct *) bpf_get_current_task());

	fn_ptr = (char *) PT_REGS_PARM2(ctx);
	ohow_ptr = (struct open_how *) PT_REGS_PARM3(ctx);

	bpf_core_read_user_str(&context.filename, sizeof(context.filename), fn_ptr);
	context.flags = READ_KERN(ohow_ptr->flags);
	context.mode = READ_KERN(ohow_ptr->mode);

	if (context.flags & O_DIRECTORY)
		return 0;

	return bpf_perf_event_output(ctx, &events, 0xffffffffULL, &context, sizeof(context));
}

// global license var
char LICENSE[] SEC("license") = "GPL";
