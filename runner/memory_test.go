package runner

import (
	"testing"

	"go.uber.org/zap"
)

func Test_parseSize(t *testing.T) {
	ret, _ := parseSize("   145164 kB")
	if ret != 145164 {
		t.Errorf("PeakMemory parse failed, %d", ret)
	}
}

func Test_parseLine(t *testing.T) {
	s1, s2 := parseLine("VmPeak:   145164 kB")
	expect1 := "VmPeak"
	if s1 != expect1 {
		t.Errorf("parse prefix failed, expect %s, actual: %s", expect1, s1)
	}
	expect2 := "145164 kB"
	if s2 != expect2 {
		t.Errorf("parse prefix failed, expect %s, actual: %s", expect2, s2)
	}
}

func Test_parseMemory(t *testing.T) {
	fileContent := `
VmPeak:   145164 kB
VmSize:   144372 kB
VmLck:         0 kB
VmPin:         0 kB
VmHWM:      7308 kB
VmRSS:      5640 kB
`
	expect := int64(7308)
	ret, _ := parseMemory(fileContent)
	if ret != expect {
		t.Errorf("PeakMemory parse failed, %d", ret)
	}
}

func TestOutOfMemoryUsesPeakMemoryOnly(t *testing.T) {
	log = zap.NewNop().Sugar()
	defer func() {
		log = nil
	}()

	task := &RunningTask{
		memoryLimit: 4096,
		Result: &Result{
			PeakMemory:   488,
			RusageMemory: 7144,
		},
	}

	if task.outOfMemory() {
		t.Fatal("outOfMemory() = true, want false when only RusageMemory exceeds limit")
	}

	task.Result.PeakMemory = 5000
	if !task.outOfMemory() {
		t.Fatal("outOfMemory() = false, want true when PeakMemory exceeds limit")
	}
}
