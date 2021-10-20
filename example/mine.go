package main

import "C"

// #cgo LDFLAGS: -lz -lelf -lbpf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	bpf "github.com/aquasecurity/libbpfgo"
	"log"
	"os"
	"runtime"
	"time"
)

func okExit() {
	_, _ = fmt.Fprintf(os.Stdout, "success\n")
	os.Exit(0)
}

func errExit(why error) {
	_, fn, line, _ := runtime.Caller(1)
	log.Printf("error: %s:%d %v\n", fn, line, why)
	os.Exit(1)
}

func errTimeout() {
	_, _ = fmt.Fprintf(os.Stdout, "timeout\n")
	os.Exit(3)
}

type data struct {
	Comm     [16]byte // 00 - 16 : command (task_comm_len)
	Pid      uint32   // 16 - 20 : process id
	Uid      uint32   // 20 - 24 : user id
	Gid      uint32   // 24 - 28 : group id
	LoginUid uint32   // 28 - 32 : real user (login/terminal)
}

type gdata struct {
	Comm     string
	Pid      uint
	Uid      uint
	Gid      uint
	LoginUid uint
}

func main() {

	var err error

	var bpfModule *bpf.Module
	var bpfMapEvents *bpf.BPFMap
	var bpfProgKsysSync *bpf.BPFProg
	//var bpfLinkKsysSync *bpf.BPFLink
	var perfBuffer *bpf.PerfBuffer

	var eventsChannel chan []byte
	var lostChannel chan uint64

	// create BPF module using BPF object file
	bpfModule, err = bpf.NewModuleFromFile("mine.bpf.o")
	if err != nil {
		errExit(err)
	}
	defer bpfModule.Close()

	// BPF map "events": resize it before object is loaded
	bpfMapEvents, err = bpfModule.GetMap("events")
	err = bpfMapEvents.Resize(8192)
	if err != nil {
		errExit(err)
	}

	// load BPF object from BPF module
	if err = bpfModule.BPFLoadObject(); err != nil {
		errExit(err)
	}

	// when using ksys_sync kprobe:

	// get BPF program from BPF object
	bpfProgKsysSync, err = bpfModule.GetProgram("ksys_sync")
	if err != nil {
		errExit(err)
	}

	// attach to BPF program to kprobe
	_, err = bpfProgKsysSync.AttachKprobe("ksys_sync")
	if err != nil {
		errExit(err)
	}

	// when using ksys_sync syscall tracepoint

	// get BPF program from BPF object
	bpfProgKsysSync, err = bpfModule.GetProgram("tracepoint__sys_enter_sync")
	if err != nil {
		errExit(err)
	}

	// attach to BPF program to kprobe
	_, err = bpfProgKsysSync.AttachTracepoint("syscalls:sys_enter_sync")
	if err != nil {
		errExit(err)
	}

	// channel for events (and lost events)
	eventsChannel = make(chan []byte)
	lostChannel = make(chan uint64)

	perfBuffer, err = bpfModule.InitPerfBuf("events", eventsChannel, lostChannel, 1)
	if err != nil {
		errExit(err)
	}

	// start perf event polling (will receive events through eventChannel)
	perfBuffer.Start()

	fmt.Println("Listening for events, <Ctrl-C> or or SIG_TERM to end it.")

	timeout := make(chan bool)
	allGood := make(chan bool)

	go func() {
		time.Sleep(60 * time.Second)
		timeout <- true
	}()

	go func() {
		// receive events until channel is closed
		for dataRaw := range eventsChannel {
			var dt data
			var dataBuffer *bytes.Buffer

			dataBuffer = bytes.NewBuffer(dataRaw)

			err = binary.Read(dataBuffer, binary.LittleEndian, &dt)
			if err != nil {
				fmt.Println(err)
				continue
			}

			goData := gdata{
				Comm:     string(bytes.TrimRight(dt.Comm[:], "\x00")),
				Pid:      uint(dt.Pid),
				Uid:      uint(dt.Uid),
				Gid:      uint(dt.Gid),
				LoginUid: uint(dt.LoginUid),
			}

			_, _ = fmt.Fprintf(os.Stdout, "%s (pid: %d) (loginuid: %d)\n", goData.Comm, goData.Pid, goData.LoginUid)
		}
	}()

	select {
	case <-allGood:
		okExit()
	case <-timeout:
		errTimeout()
	}
}
