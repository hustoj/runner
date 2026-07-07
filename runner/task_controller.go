package runner

type memoryStatus struct {
	PeakMemoryKB int64
	OOMCount     int64
	OOMKillCount int64
}

func (status memoryStatus) Exceeded() bool {
	return status.OOMCount > 0 || status.OOMKillCount > 0
}

// TaskController controls a per-task process container used by launchers that
// need to move, kill, and clean up a process tree without exposing runner-only
// memory accounting details.
type TaskController interface {
	MovePID(pid int) error
	Kill() error
	Cleanup() error
}

type taskController interface {
	TaskController
	MemoryStatus() (memoryStatus, error)
}
