package runner

import (
	"errors"
	"os"
	"syscall"
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

func Test_parseThreadGroupID(t *testing.T) {
	fileContent := `
Name: main
Tgid:  4242
Pid:   4242
`
	ret, err := parseThreadGroupID(fileContent)
	if err != nil {
		t.Fatalf("parseThreadGroupID failed: %v", err)
	}
	if ret != 4242 {
		t.Fatalf("parseThreadGroupID returned %d", ret)
	}
}

func Test_parseMemoryInfo(t *testing.T) {
	fileContent := `
Name: main
Tgid:  4242
VmHWM: 1234 kB
`
	info, err := parseMemoryInfo(fileContent)
	if err != nil {
		t.Fatalf("parseMemoryInfo failed: %v", err)
	}
	if info.ThreadGroup != 4242 {
		t.Fatalf("parseMemoryInfo thread group = %d", info.ThreadGroup)
	}
	if info.PeakMemory != 1234 {
		t.Fatalf("parseMemoryInfo peak memory = %d", info.PeakMemory)
	}
}

func Test_parseStatusFieldRequiresColon(t *testing.T) {
	value, ok := parseStatusField("TgidExtra: 99\nTgid: 42\n", "Tgid")
	if !ok {
		t.Fatal("parseStatusField should find Tgid")
	}
	if value != "42" {
		t.Fatalf("parseStatusField returned %q", value)
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

func TestRefreshMemoryAggregatesActiveTraceesByThreadGroup(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	oldProcStatusReader := procStatusReader
	procStatusReader = func(path string) ([]byte, error) {
		switch path {
		case "/proc/100/status":
			return []byte("Name:\tmain\nTgid:\t100\nVmHWM:\t64 kB\n"), nil
		case "/proc/101/status":
			return []byte("Name:\tworker\nTgid:\t100\nVmHWM:\t64 kB\n"), nil
		case "/proc/200/status":
			return []byte("Name:\thelper\nTgid:\t200\nVmHWM:\t48 kB\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	defer func() {
		procStatusReader = oldProcStatusReader
	}()

	process := NewProcess(100)
	process.AddTracee(101)
	process.AddTracee(200)

	task := &RunningTask{
		process: process,
		Result:  &Result{},
	}
	task.Result.Init()
	task.refreshMemory()

	if task.Result.PeakMemory != 112 {
		t.Fatalf("PeakMemory = %d, want 112", task.Result.PeakMemory)
	}
}

func TestRefreshMemoryUsesAdjustedRusageAsPeakFallback(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())
	defer SetLogger(nil)

	oldProcStatusReader := procStatusReader
	procStatusReader = func(path string) ([]byte, error) {
		return nil, errors.New("proc status unavailable")
	}
	defer func() {
		procStatusReader = oldProcStatusReader
	}()

	process := NewProcess(100)
	process.SetThreadGroup(100, 100)
	process.SetRusageOffset(100, 64)
	process.recordRusage(100, syscall.Rusage{Maxrss: 96})

	task := &RunningTask{
		process: process,
		Result:  &Result{},
	}
	task.Result.Init()
	task.refreshMemory()

	if task.Result.RusageMemory != 32 {
		t.Fatalf("RusageMemory = %d, want 32", task.Result.RusageMemory)
	}
	if task.Result.PeakMemory != 32 {
		t.Fatalf("PeakMemory = %d, want 32", task.Result.PeakMemory)
	}
}
