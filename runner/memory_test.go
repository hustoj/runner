package runner

import (
	"testing"

	"go.uber.org/zap"
)

type fakeTaskController struct {
	status memoryStatus
	err    error
}

func (controller fakeTaskController) MemoryStatus() (memoryStatus, error) {
	return controller.status, controller.err
}

func (controller fakeTaskController) MovePID(pid int) error {
	return nil
}

func (controller fakeTaskController) Cleanup() error {
	return nil
}

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

func TestOutOfMemoryFallsBackToPeakMemoryWithoutController(t *testing.T) {
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

func TestOutOfMemoryUsesMemoryControllerEvents(t *testing.T) {
	log = zap.NewNop().Sugar()
	defer func() {
		log = nil
	}()

	task := &RunningTask{
		memoryLimit: 4096,
		taskCtrl: fakeTaskController{
			status: memoryStatus{
				PeakMemoryKB: 2048,
				OOMCount:     0,
				OOMKillCount: 0,
			},
		},
		Result: &Result{
			PeakMemory:   488,
			RusageMemory: 7144,
		},
	}

	if task.outOfMemory() {
		t.Fatal("outOfMemory() = true, want false when task controller has no OOM events")
	}
	if task.Result.PeakMemory != 2048 {
		t.Fatalf("PeakMemory = %d, want 2048 from task controller", task.Result.PeakMemory)
	}

	task.taskCtrl = fakeTaskController{
		status: memoryStatus{
			PeakMemoryKB: 3072,
			OOMCount:     1,
		},
	}
	if !task.outOfMemory() {
		t.Fatal("outOfMemory() = false, want true when task controller reports OOM")
	}
}
