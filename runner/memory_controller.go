package runner

type memoryStatus struct {
	PeakMemoryKB int64
	OOMCount     int64
	OOMKillCount int64
}

func (status memoryStatus) Exceeded() bool {
	return status.OOMCount > 0 || status.OOMKillCount > 0
}

type memoryController interface {
	Status() (memoryStatus, error)
	MovePID(pid int) error
	Cleanup() error
}
