package job

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// kabiHideName matches the name that the kABI macros of some distribution
// kernels give to the struct member hiding a replaced one, such as
// __UNIQUE_ID_rh_kabi_hide42 (RHEL's rh_kabi.h) or __UNIQUE_ID_uek_kabi_hide42
// (Oracle UEK's uek_kabi.h), with its NUL terminator. The number comes from
// __COUNTER__, so it differs between compilation units. Group 1 is the number.
var kabiHideName = regexp.MustCompile(`__UNIQUE_ID_[A-Za-z0-9_]*?_kabi_hide([0-9]+)\x00`)

// errCompressedDebugStr is returned for a .debug_str section that is
// compressed, which NormalizeKABINames does not handle.
var errCompressedDebugStr = errors.New("compressed .debug_str")

// normalizeKABIStrings removes the numbers from the kABI hide names in a
// string table, in place, by overwriting them with NUL bytes. Each string only
// ends earlier, so no offset into the table changes. It returns the number of
// names changed.
func normalizeKABIStrings(strtab []byte) int {
	matches := kabiHideName.FindAllSubmatchIndex(strtab, -1)
	for _, m := range matches {
		clear(strtab[m[2]:m[3]])
	}
	return len(matches)
}

// NormalizeKABINames removes the numbers from the kABI hide names in the
// .debug_str section of a vmlinux file, in place, and returns the number of
// names changed.
//
// With a different name in each compilation unit, pahole sees a different
// version of each struct holding such a member, and of each type referring to
// those structs, in each compilation unit, and BTF deduplication cannot merge
// them. The BTF of Oracle Linux UEK 5.4.17-2122, for instance, ends up with
// about a million types and 881 copies of task_struct, instead of about
// 100,000 types and 2 copies.
func NormalizeKABINames(vmlinux string) (int, error) {
	f, err := os.OpenFile(vmlinux, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	ef, err := elf.NewFile(f)
	if err != nil {
		return 0, fmt.Errorf("elf open: %s", err)
	}

	sec := ef.Section(".debug_str")
	if sec == nil || sec.Type == elf.SHT_NOBITS {
		return 0, nil
	}
	if sec.Flags&elf.SHF_COMPRESSED != 0 {
		return 0, errCompressedDebugStr
	}

	strtab := make([]byte, sec.FileSize)
	if _, err := f.ReadAt(strtab, int64(sec.Offset)); err != nil {
		return 0, fmt.Errorf("read .debug_str: %s", err)
	}

	n := normalizeKABIStrings(strtab)
	if n == 0 {
		return 0, nil
	}

	if _, err := f.WriteAt(strtab, int64(sec.Offset)); err != nil {
		return 0, fmt.Errorf("write .debug_str: %s", err)
	}

	return n, nil
}

// kabiHideMember matches a kABI hide member name once NormalizeKABINames has
// cut its number off.
var kabiHideMember = regexp.MustCompile(`^__UNIQUE_ID_[A-Za-z0-9_]*_kabi_hide$`)

// BTF kinds, from include/uapi/linux/btf.h
const (
	btfKindInt       = 1
	btfKindPtr       = 2
	btfKindArray     = 3
	btfKindStruct    = 4
	btfKindUnion     = 5
	btfKindEnum      = 6
	btfKindFwd       = 7
	btfKindTypedef   = 8
	btfKindVolatile  = 9
	btfKindConst     = 10
	btfKindRestrict  = 11
	btfKindFunc      = 12
	btfKindFuncProto = 13
	btfKindVar       = 14
	btfKindDatasec   = 15
	btfKindFloat     = 16
	btfKindDeclTag   = 17
	btfKindTypeTag   = 18
	btfKindEnum64    = 19
)

// btfTypeExtra returns how many bytes follow the common btf_type header for a
// type of the given kind and vlen, and false for an unknown kind.
func btfTypeExtra(kind uint32, vlen int) (int, bool) {
	switch kind {
	case btfKindPtr, btfKindFwd, btfKindTypedef, btfKindVolatile, btfKindConst,
		btfKindRestrict, btfKindFunc, btfKindFloat, btfKindTypeTag:
		return 0, true
	case btfKindInt, btfKindVar, btfKindDeclTag:
		return 4, true
	case btfKindArray:
		return 12, true
	case btfKindStruct, btfKindUnion, btfKindDatasec, btfKindEnum64:
		return 12 * vlen, true
	case btfKindEnum, btfKindFuncProto:
		return 8 * vlen, true
	}
	return 0, false
}

// renameKABIHideMembers appends to each normalized kABI hide member name the ID
// of the member's type, adding the new names to the end of the string section.
// It returns the new BTF and the number of members renamed.
func renameKABIHideMembers(data []byte) ([]byte, int, error) {
	const btfMagic = 0xeb9f
	const hdrSize = 24

	if len(data) < hdrSize {
		return nil, 0, errors.New("BTF too short")
	}

	var order binary.ByteOrder
	switch {
	case binary.LittleEndian.Uint16(data) == btfMagic:
		order = binary.LittleEndian
	case binary.BigEndian.Uint16(data) == btfMagic:
		order = binary.BigEndian
	default:
		return nil, 0, errors.New("not a BTF file")
	}

	hdrLen := uint64(order.Uint32(data[4:]))
	typeOff := hdrLen + uint64(order.Uint32(data[8:]))
	typeEnd := typeOff + uint64(order.Uint32(data[12:]))
	strOff := hdrLen + uint64(order.Uint32(data[16:]))
	strLen := uint64(order.Uint32(data[20:]))
	strEnd := strOff + strLen

	if typeEnd > uint64(len(data)) || strEnd != uint64(len(data)) {
		return nil, 0, errors.New("unexpected BTF layout")
	}
	strs := data[strOff:strEnd]

	name := func(off uint32) (string, error) {
		if uint64(off) >= strLen {
			return "", fmt.Errorf("string offset %d out of range", off)
		}
		end := bytes.IndexByte(strs[off:], 0)
		if end < 0 {
			return "", fmt.Errorf("string at offset %d not terminated", off)
		}
		return string(strs[off : int(off)+end]), nil
	}

	out := bytes.Clone(data)
	var added []byte
	newOffs := make(map[string]uint32)
	renamed := 0

	for p := typeOff; p < typeEnd; {
		if p+12 > typeEnd {
			return nil, 0, errors.New("truncated BTF type")
		}
		info := order.Uint32(data[p+4:])
		kind, vlen := (info>>24)&0x1f, int(info&0xffff)
		p += 12

		extra, ok := btfTypeExtra(kind, vlen)
		if !ok {
			return nil, 0, fmt.Errorf("unknown BTF kind %d", kind)
		}
		if p+uint64(extra) > typeEnd {
			return nil, 0, errors.New("truncated BTF type")
		}

		if kind == btfKindStruct || kind == btfKindUnion {
			for m := p; m < p+uint64(extra); m += 12 {
				member, err := name(order.Uint32(data[m:]))
				if err != nil {
					return nil, 0, err
				}
				if !kabiHideMember.MatchString(member) {
					continue
				}
				newName := member + strconv.FormatUint(uint64(order.Uint32(data[m+4:])), 10)
				off, ok := newOffs[newName]
				if !ok {
					off = uint32(strLen) + uint32(len(added))
					added = append(append(added, newName...), 0)
					newOffs[newName] = off
				}
				order.PutUint32(out[m:], off)
				renamed++
			}
		}

		p += uint64(extra)
	}

	if renamed == 0 {
		return data, 0, nil
	}

	order.PutUint32(out[20:], uint32(strLen)+uint32(len(added)))

	return append(out, added...), renamed, nil
}

// RenameKABIHideMembers gives each kABI hide member of a BTF file a name of
// its own, by appending the ID of its type to the name NormalizeKABINames
// left, and returns the number of members renamed.
//
// A struct with two replaced members otherwise holds two members with the
// same name, in sibling anonymous unions, and C headers generated from the
// BTF (bpftool btf dump format c) don't compile. The type of a hide member
// is the anonymous struct holding the replaced member, and two replaced
// members of a struct differ, so their names do too. Run after pahole, the
// renaming does not change what deduplication merged.
func RenameKABIHideMembers(btfPath string) (int, error) {
	data, err := os.ReadFile(btfPath)
	if err != nil {
		return 0, err
	}

	out, n, err := renameKABIHideMembers(data)
	if err != nil || n == 0 {
		return n, err
	}

	info, err := os.Stat(btfPath)
	if err != nil {
		return 0, err
	}

	tmp := btfPath + ".tmp"
	if err := os.WriteFile(tmp, out, info.Mode().Perm()); err != nil {
		os.Remove(tmp)
		return 0, err
	}

	return n, os.Rename(tmp, btfPath)
}
