#ifndef MINE_H_
#define MINE_H_

typedef signed char		__s8;
typedef unsigned char		__u8;
typedef short unsigned int	__u16;
typedef int			__s32;
typedef unsigned int		__u32;
typedef long long int		__s64;
typedef long long unsigned int	__u64;

typedef __s8  s8 ;
typedef __u8  u8 ;
typedef __u16 u16;
typedef __s32 s32;
typedef __u32 u32;
typedef __s64 s64;
typedef __u64 u64;

#ifndef KERNEL_VERSION
#define KERNEL_VERSION(a,b,c) (((a) << 16) + ((b) << 8) + (c))
#endif

#define READ_KERN(ptr) \
	({ typeof(ptr) _val;				\
	__builtin_memset(&_val, 0, sizeof(_val));	\
	bpf_core_read(&_val, sizeof(_val), &ptr);	\
	_val;						\
	})

#define _wrapout(nl, ...)           \
{                                   \
    fprintf(stdout, __VA_ARGS__);   \
    if (nl)                         \
        fprintf(stdout, "\n");      \
    fflush(stdout);                 \
}

#define _wrapout0(...) _wrapout(0, __VA_ARGS__)
#define _wrapout1(...) _wrapout(1, __VA_ARGS__)

#define wrapout  _wrapout1
#define here     _wrapout1("line %d, file %s, function %s", __LINE__, __FILE__, __func__)
#define debug(a) _wrapout1("%s (line %d, file %s, function %s)", a, __LINE__, __FILE__, __func__)

#define exiterr(...)	\
{			\
	here;		\
        exit(1);	\
}

typedef struct event_context {
	u32 pid;
	u32 tid;
	u32 ppid;
	u32 uid;
	u64 flags;
	u64 mode;
	u64 ts;
	char comm[16];
	char filename[64];
} context_t;

#endif // MINE_H_
