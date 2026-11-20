/* SPDX-License-Identifier: GPL-2.0 */
#ifndef EXAMPLE_H
#define EXAMPLE_H

#define TASK_COMM_LEN 16
#define FILENAME_LEN 128

/* sent from the eBPF program to the loader for each openat() */
struct event {
	unsigned int pid;
	unsigned int ppid;
	char comm[TASK_COMM_LEN];
	char filename[FILENAME_LEN];
};

#endif /* EXAMPLE_H */
