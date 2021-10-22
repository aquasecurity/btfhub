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
	Pid      uint32   // 0 4
	Tid      uint32   // 4 4
	PPid     uint32   // 8 4
	Uid      uint32   // 12 4
	Flags    uint64   // 16 8
	Mode     uint64   // 24 8
	TS       uint64   // 32 8
	Comm     [16]byte // 40 16
	FileName [64]byte // 56 64
}

type gdata struct {
	Pid      uint
	Tid      uint
	PPid     uint
	Uid      uint
	Flags    uint
	Mode     uint
	TS       uint
	Comm     string
	FileName string
}

func main() {

	var err error

	var bpfModule *bpf.Module
	var bpfMapEvents *bpf.BPFMap
	var bpfProgKsysSync *bpf.BPFProg
	var perfBuffer *bpf.PerfBuffer

	var eventsChannel chan []byte
	var lostChannel chan uint64

	// create BPF module using BPF object file
	bpfModule, err = bpf.NewModuleFromFile("example.bpf.o")
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

	// get BPF program from BPF object
	bpfProgKsysSync, err = bpfModule.GetProgram("do_sys_openat2")
	if err != nil {
		errExit(err)
	}

	// attach to BPF program to kprobe
	_, err = bpfProgKsysSync.AttachKprobe("do_sys_openat2")
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
				Pid:      uint(dt.Pid),
				Tid:      uint(dt.Tid),
				PPid:     uint(dt.PPid),
				Uid:      uint(dt.Uid),
				Flags:    uint(dt.Flags),
				Mode:     uint(dt.Mode),
				TS:       uint(dt.TS),
				Comm:     string(bytes.TrimRight(dt.Comm[:], "\x00")),
				FileName: string(bytes.TrimRight(dt.FileName[:], "\x00")),
			}
			_, _ = fmt.Fprintf(os.Stdout, "%s (pid: %d) opened: %s (flags: %08x, mode: %08x)\n", goData.Comm, goData.Pid, goData.FileName, goData.Flags, goData.Mode)
		}
	}()

	select {
	case <-allGood:
		okExit()
	case <-timeout:
		errTimeout()
	}
}
