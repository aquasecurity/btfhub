package kernel

import (
	"regexp"
	"strconv"
)

type KernelVersion struct {
	str  string
	ints []int
}

func NewKernelVersion(v string) KernelVersion {
	return KernelVersion{str: v, ints: splitIntoInts(v)}
}

func (k KernelVersion) IsZero() bool {
	return k.str == ""
}

func (k KernelVersion) String() string {
	return k.str
}

func (k KernelVersion) Less(j KernelVersion) bool {
	vi, vj := k.ints, j.ints
	for x, vni := range vi {
		if x > (len(vj) - 1) {
			return false
		}
		if vni == vj[x] {
			continue
		}
		return vni < vj[x]
	}
	return len(vi) < len(vj)
}

func splitIntoInts(s string) []int {
	var nums []int
	for _, n := range regexp.MustCompile(`([.~-])`).Split(s, -1) {
		x, err := strconv.Atoi(n)
		if err != nil {
			return nums
		}
		nums = append(nums, x)
	}
	return nums
}
