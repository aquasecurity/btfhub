package job

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeKABIStrings(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		changed int
		found   int
	}{
		{
			name:    "rh_kabi and uek_kabi names",
			in:      "\x00__UNIQUE_ID_rh_kabi_hide42\x00task_struct\x00__UNIQUE_ID_uek_kabi_hide7\x00",
			want:    "\x00__UNIQUE_ID_rh_kabi_hide\x00\x00\x00task_struct\x00__UNIQUE_ID_uek_kabi_hide\x00\x00",
			changed: 2,
			found:   2,
		},
		{
			name:  "already normalized",
			in:    "\x00__UNIQUE_ID_rh_kabi_hide\x00\x00\x00",
			want:  "\x00__UNIQUE_ID_rh_kabi_hide\x00\x00\x00",
			found: 1,
		},
		{
			name:    "one normalized, one not",
			in:      "\x00__UNIQUE_ID_rh_kabi_hide\x00\x00\x00__UNIQUE_ID_rh_kabi_hide9\x00",
			want:    "\x00__UNIQUE_ID_rh_kabi_hide\x00\x00\x00__UNIQUE_ID_rh_kabi_hide\x00\x00",
			changed: 1,
			found:   2,
		},
		{
			name: "other unique ids and kabi names are kept",
			in:   "__UNIQUE_ID_ddebug123\x00rh_kabi_hide5\x00__UNIQUE_ID_rh_kabi_hide12x\x00UCOUNT_KABI_RESERVE_1\x00",
			want: "__UNIQUE_ID_ddebug123\x00rh_kabi_hide5\x00__UNIQUE_ID_rh_kabi_hide12x\x00UCOUNT_KABI_RESERVE_1\x00",
		},
		{
			name: "name without terminator is kept",
			in:   "__UNIQUE_ID_rh_kabi_hide42",
			want: "__UNIQUE_ID_rh_kabi_hide42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := []byte(tt.in)
			changed, found := normalizeKABIStrings(b)
			if changed != tt.changed || found != tt.found {
				t.Errorf("got %d changed and %d found, want %d and %d", changed, found, tt.changed, tt.found)
			}
			if string(b) != tt.want {
				t.Errorf("got %q, want %q", b, tt.want)
			}
		})
	}
}

// writeELF writes a minimal 64-bit little endian ELF file holding a
// .debug_str section with the given contents and flags, and returns its path.
func writeELF(t *testing.T, debugStr []byte, flags elf.SectionFlag) string {
	t.Helper()

	shstrtab := []byte("\x00.debug_str\x00.shstrtab\x00")
	hdrSize := binary.Size(elf.Header64{})
	shdrSize := binary.Size(elf.Section64{})
	debugStrOff := hdrSize
	shstrtabOff := debugStrOff + len(debugStr)
	shOff := shstrtabOff + len(shstrtab)

	var buf bytes.Buffer
	hdr := elf.Header64{
		Type:      uint16(elf.ET_EXEC),
		Machine:   uint16(elf.EM_X86_64),
		Version:   uint32(elf.EV_CURRENT),
		Shoff:     uint64(shOff),
		Ehsize:    uint16(hdrSize),
		Shentsize: uint16(shdrSize),
		Shnum:     3,
		Shstrndx:  2,
	}
	copy(hdr.Ident[:], elf.ELFMAG)
	hdr.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	hdr.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	hdr.Ident[elf.EI_VERSION] = byte(elf.EV_CURRENT)

	sections := []elf.Section64{
		{},
		{
			Name:      1, // .debug_str
			Type:      uint32(elf.SHT_PROGBITS),
			Flags:     uint64(flags),
			Off:       uint64(debugStrOff),
			Size:      uint64(len(debugStr)),
			Addralign: 1,
			Entsize:   1,
		},
		{
			Name:      12, // .shstrtab
			Type:      uint32(elf.SHT_STRTAB),
			Off:       uint64(shstrtabOff),
			Size:      uint64(len(shstrtab)),
			Addralign: 1,
		},
	}

	for _, v := range []any{hdr, debugStr, shstrtab, sections} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(t.TempDir(), "vmlinux")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNormalizeKABINames(t *testing.T) {
	debugStr := []byte("\x00int\x00__UNIQUE_ID_uek_kabi_hide53\x00__UNIQUE_ID_uek_kabi_hide1234\x00task_struct\x00")
	path := writeELF(t, debugStr, elf.SHF_MERGE|elf.SHF_STRINGS)
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	changed, found, err := NormalizeKABINames(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 2 || found != 2 {
		t.Errorf("got %d changed and %d found, want 2 and 2", changed, found)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := bytes.Replace(orig, []byte("hide53\x00"), []byte("hide\x00\x00\x00"), 1)
	want = bytes.Replace(want, []byte("hide1234\x00"), []byte("hide\x00\x00\x00\x00\x00"), 1)
	if !bytes.Equal(got, want) {
		t.Errorf("file changed outside the names' numbers")
	}

	ef, err := elf.Open(path)
	if err != nil {
		t.Fatalf("not a valid ELF any more: %s", err)
	}
	defer ef.Close()
	data, err := ef.Section(".debug_str").Data()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(debugStr) {
		t.Errorf(".debug_str is %d bytes, want %d", len(data), len(debugStr))
	}

	// a second run, as on a vmlinux kept by a failed job, finds the names
	// but nothing left to change
	changed, found, err = NormalizeKABINames(path)
	if err != nil || changed != 0 || found != 2 {
		t.Errorf("second run: got %d changed and %d found, err %v, want 0 and 2", changed, found, err)
	}
}

func TestNormalizeKABINamesNoNames(t *testing.T) {
	path := writeELF(t, []byte("\x00int\x00task_struct\x00"), elf.SHF_MERGE|elf.SHF_STRINGS)
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	changed, found, err := NormalizeKABINames(path)
	if err != nil || changed != 0 || found != 0 {
		t.Errorf("got %d changed and %d found, err %v, want none", changed, found, err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("file changed")
	}
}

func TestNormalizeKABINamesCompressed(t *testing.T) {
	path := writeELF(t, []byte("\x00__UNIQUE_ID_rh_kabi_hide1\x00"), elf.SHF_COMPRESSED)
	if _, _, err := NormalizeKABINames(path); !errors.Is(err, errCompressedDebugStr) {
		t.Errorf("got err %v, want %v", err, errCompressedDebugStr)
	}
}

// btfBuilder writes a small little endian BTF file, for tests.
type btfBuilder struct {
	types []byte
	strs  []byte
}

func newBTFBuilder() *btfBuilder {
	return &btfBuilder{strs: []byte{0}}
}

func (b *btfBuilder) str(s string) uint32 {
	if s == "" {
		return 0
	}
	off := uint32(len(b.strs))
	b.strs = append(append(b.strs, s...), 0)
	return off
}

func (b *btfBuilder) typ(name string, kind, vlen, sizeType uint32, extra ...uint32) {
	for _, v := range append([]uint32{b.str(name), kind<<24 | vlen, sizeType}, extra...) {
		b.types = binary.LittleEndian.AppendUint32(b.types, v)
	}
}

// member returns the name, type and offset words of a struct or union member.
func (b *btfBuilder) member(name string, typ, bitOff uint32) []uint32 {
	return []uint32{b.str(name), typ, bitOff}
}

func (b *btfBuilder) bytes() []byte {
	hdr := []byte{0x9f, 0xeb, 1, 0}
	for _, v := range []uint32{24, 0, uint32(len(b.types)), uint32(len(b.types)), uint32(len(b.strs))} {
		hdr = binary.LittleEndian.AppendUint32(hdr, v)
	}
	return append(append(hdr, b.types...), b.strs...)
}

// btfMemberNames returns the member names of each struct and union, by type ID.
func btfMemberNames(t *testing.T, data []byte) map[int][]string {
	t.Helper()

	hdrLen := binary.LittleEndian.Uint32(data[4:])
	typeLen := binary.LittleEndian.Uint32(data[12:])
	strs := data[hdrLen+binary.LittleEndian.Uint32(data[16:]):]
	types := data[hdrLen+binary.LittleEndian.Uint32(data[8:]):][:typeLen]
	name := func(off uint32) string {
		return string(strs[off : int(off)+bytes.IndexByte(strs[off:], 0)])
	}

	names := make(map[int][]string)
	for p, id := 0, 1; p < len(types); id++ {
		info := binary.LittleEndian.Uint32(types[p+4:])
		kind, vlen := (info>>24)&0x1f, int(info&0xffff)
		p += 12
		extra, ok := btfTypeExtra(kind, vlen)
		if !ok {
			t.Fatalf("unknown kind %d", kind)
		}
		if kind == btfKindStruct || kind == btfKindUnion {
			for m := p; m < p+extra; m += 12 {
				names[id] = append(names[id], name(binary.LittleEndian.Uint32(types[m:])))
			}
		}
		p += extra
	}
	return names
}

func TestRenameKABIHideMembers(t *testing.T) {
	// struct s {
	//	union { int x; struct { int a; } __UNIQUE_ID_rh_kabi_hide; };
	//	union { int y; struct { int b; } __UNIQUE_ID_rh_kabi_hide; };
	// };
	b := newBTFBuilder()
	b.typ("int", btfKindInt, 0, 4, 32)                                                                           // 1
	b.typ("", btfKindStruct, 1, 4, b.member("a", 1, 0)...)                                                       // 2
	b.typ("", btfKindStruct, 1, 4, b.member("b", 1, 0)...)                                                       // 3
	b.typ("", btfKindUnion, 2, 4, append(b.member("x", 1, 0), b.member("__UNIQUE_ID_rh_kabi_hide", 2, 0)...)...) // 4
	b.typ("", btfKindUnion, 2, 4, append(b.member("y", 1, 0), b.member("__UNIQUE_ID_rh_kabi_hide", 3, 0)...)...) // 5
	b.typ("s", btfKindStruct, 2, 8, append(b.member("", 4, 0), b.member("", 5, 32)...)...)                       // 6
	b.typ("", btfKindPtr, 0, 6)                                                                                  // 7
	orig := b.bytes()

	out, n, err := renameKABIHideMembers(orig)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d members renamed, want 2", n)
	}

	got := btfMemberNames(t, out)
	if want := []string{"x", "__UNIQUE_ID_rh_kabi_hide2"}; !reflect.DeepEqual(got[4], want) {
		t.Errorf("union 4: got %q, want %q", got[4], want)
	}
	if want := []string{"y", "__UNIQUE_ID_rh_kabi_hide3"}; !reflect.DeepEqual(got[5], want) {
		t.Errorf("union 5: got %q, want %q", got[5], want)
	}
	if want := btfMemberNames(t, orig); !reflect.DeepEqual(got[6], want[6]) || !reflect.DeepEqual(got[2], want[2]) {
		t.Errorf("other members changed: got %q, want %q", got, want)
	}

	// Only member name offsets change: the type section keeps its size, and
	// the original strings stay where they were
	typeLen := binary.LittleEndian.Uint32(orig[12:])
	if binary.LittleEndian.Uint32(out[12:]) != typeLen {
		t.Errorf("type section length changed")
	}
	if !bytes.HasPrefix(out[24+typeLen:], orig[24+typeLen:]) {
		t.Errorf("original strings changed")
	}

	// a second run finds nothing left to rename
	again, n, err := renameKABIHideMembers(out)
	if err != nil || n != 0 || !bytes.Equal(again, out) {
		t.Errorf("second run: %d members renamed, err %v", n, err)
	}
}

func TestRenameKABIHideMembersNone(t *testing.T) {
	// numbered names, as without NormalizeKABINames, are left alone
	b := newBTFBuilder()
	b.typ("int", btfKindInt, 0, 4, 32)
	b.typ("", btfKindStruct, 1, 4, b.member("a", 1, 0)...)
	b.typ("", btfKindUnion, 2, 4, append(b.member("x", 1, 0), b.member("__UNIQUE_ID_rh_kabi_hide7", 2, 0)...)...)
	orig := b.bytes()

	out, n, err := renameKABIHideMembers(orig)
	if err != nil || n != 0 || !bytes.Equal(out, orig) {
		t.Errorf("got %d members renamed, err %v, changed %v", n, err, !bytes.Equal(out, orig))
	}

	path := filepath.Join(t.TempDir(), "x.btf")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := RenameKABIHideMembers(path); err != nil || n != 0 {
		t.Errorf("file: got %d members renamed, err %v", n, err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, orig) {
		t.Errorf("file changed")
	}
}

func TestRenameKABIHideMembersBadInput(t *testing.T) {
	if _, _, err := renameKABIHideMembers([]byte("not a btf file, but long enough")); err == nil {
		t.Errorf("no error for a non BTF file")
	}

	b := newBTFBuilder()
	b.typ("x", 31, 0, 0)
	if _, _, err := renameKABIHideMembers(b.bytes()); err == nil {
		t.Errorf("no error for an unknown kind")
	}
}
