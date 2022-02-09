#include "vmlinux.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#include "example.h"

extern u32 LINUX_KERNEL_VERSION __kconfig;	// will be solved load-time by libbpf

#define MAX_PERCPU_BUFSIZE	(1 << 15)	// value is set by the kernel as an upper bound
#define O_DIRECTORY		00200000

#define READ_KERN(ptr) ({ typeof(ptr) _val;		\
	__builtin_memset(&_val, 0, sizeof(_val));	\
	bpf_core_read(&_val, sizeof(_val), &ptr);	\
	_val;						\
	})

#define READ_USER(ptr) ({ typeof(ptr) _val;		\
	__builtin_memset(&_val, 0, sizeof(_val));	\
	bpf_core_read_user(&_val, sizeof(_val), &ptr);	\
	_val;						\
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

static __always_inline u32
get_task_ppid(struct task_struct *task)
{
	struct task_struct *parent = READ_KERN(task->real_parent);
	return READ_KERN(parent->pid);
}

// inline helper function

static __always_inline int
init_context(context_t *context, struct task_struct *task)
{

	u64 id = bpf_get_current_pid_tgid();
	context->tid = id;
	context->pid = id >> 32;
	context->ppid = get_task_ppid(task);
	context->uid = bpf_get_current_uid_gid();
	bpf_get_current_comm(&context->comm, sizeof(context->comm));
	context->ts = bpf_ktime_get_ns();

	return 0;
}

// tracepoint sys_enter_openat

SEC("tracepoint/syscalls/sys_enter_openat")
int sys_enter_openat(struct trace_event_raw_sys_enter *ctx)
{
	context_t context = {};
	char *fn_ptr;

	init_context(&context, (struct task_struct *) bpf_get_current_task());

	fn_ptr = (char *) (ctx->args[1]);
	bpf_core_read_user_str(&context.filename, sizeof(context.filename), fn_ptr);

	// BUG <= 5.4.0 (https://bugs.launchpad.net/bugs/1944756)
	// needs: "bpf: Track contents of read-only maps as scalars"
	// if kernel does not contain that fix, the branch bellow will fail
	if (LINUX_KERNEL_VERSION > KERNEL_VERSION(5, 9, 0)) {
		// Recent kernels use struct open_how
		struct open_how *ohow_ptr = (struct open_how *) (&ctx->args[2]);
		context.flags = READ_KERN(ohow_ptr->flags);
		context.mode = READ_KERN(ohow_ptr->mode);
	} else {
		// Ubuntu Bionic and Focal (up to kernel: 5.8)
		// Older kernels have different signature
		bpf_core_read(&context.flags, sizeof(context.flags), &ctx->args[2]);
		bpf_core_read(&context.mode, sizeof(context.flags), &ctx->args[3]);
	}

	if (context.flags & O_DIRECTORY)
		return 0;

	return bpf_perf_event_output(ctx, &events, 0xffffffffULL, &context, sizeof(context));
}

// global license var
char LICENSE[] SEC("license") = "GPL";
