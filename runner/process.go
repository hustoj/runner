package runner

import (
	"errors"
	"syscall"
)

type processStop struct {
	status syscall.WaitStatus
	rusage syscall.Rusage
}

type Process struct {
	Pid          int
	CurrentPid   int
	Status       syscall.WaitStatus
	Rusage       syscall.Rusage
	IsKilled     bool
	tracees      map[int]struct{}
	rusageByPid  map[int]syscall.Rusage
	rusageOffset map[int]int64
	threadGroups map[int]int
	pendingStops map[int]processStop
}

func (process *Process) ensureStateMaps() {
	if process.tracees == nil {
		process.tracees = make(map[int]struct{})
	}
	if process.rusageByPid == nil {
		process.rusageByPid = make(map[int]syscall.Rusage)
	}
	if process.rusageOffset == nil {
		process.rusageOffset = make(map[int]int64)
	}
	if process.threadGroups == nil {
		process.threadGroups = make(map[int]int)
	}
	if process.pendingStops == nil {
		process.pendingStops = make(map[int]processStop)
	}
}

func NewProcess(pid int) *Process {
	process := &Process{
		Pid:          pid,
		CurrentPid:   pid,
		tracees:      make(map[int]struct{}),
		rusageByPid:  make(map[int]syscall.Rusage),
		rusageOffset: make(map[int]int64),
		threadGroups: make(map[int]int),
		pendingStops: make(map[int]processStop),
	}
	process.AddTracee(pid)
	return process
}

func (process *Process) Wait() (bool, error) {
	if process.IsKilled || !process.HasActiveTracees() {
		return false, nil
	}
	if pid, stop, ok := process.takePendingTrackedStop(); ok {
		process.CurrentPid = pid
		process.Status = stop.status
		process.Rusage = stop.rusage
		process.recordRusage(pid, stop.rusage)
		return true, nil
	}
	for {
		pid1, err := syscall.Wait4(-1, &process.Status, waitOptions(), &process.Rusage)
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.ECHILD {
			process.clearPendingStops()
			return false, nil
		}
		if pid1 == 0 {
			return false, errors.New("wait4 returned pid 0")
		}
		if err != nil {
			return false, err
		}
		if process.HasTracee(pid1) {
			process.CurrentPid = pid1
			process.recordRusage(pid1, process.Rusage)
			return true, nil
		}
		process.rememberPendingStop(pid1, process.Status, process.Rusage)
	}
}

// AddTracee keeps Process zero-value safe even though production code should
// construct it via NewProcess.
func (process *Process) AddTracee(pid int) {
	process.ensureStateMaps()
	process.tracees[pid] = struct{}{}
}

func (process *Process) RemoveTracee(pid int) {
	delete(process.tracees, pid)
	delete(process.pendingStops, pid)
}

func (process *Process) HasTracee(pid int) bool {
	_, ok := process.tracees[pid]
	return ok
}

func (process *Process) HasActiveTracees() bool {
	return len(process.tracees) > 0
}

func (process *Process) HasPendingStops() bool {
	return len(process.pendingStops) > 0
}

func (process *Process) ActivePids() []int {
	ret := make([]int, 0, len(process.tracees))
	for pid := range process.tracees {
		ret = append(ret, pid)
	}
	return ret
}

func (process *Process) SetThreadGroup(pid int, tgid int) {
	process.ensureStateMaps()
	process.threadGroups[pid] = tgid
}

func (process *Process) ThreadGroup(pid int) (int, bool) {
	tgid, ok := process.threadGroups[pid]
	return tgid, ok
}

func (process *Process) SetRusageOffset(groupID int, offset int64) {
	process.ensureStateMaps()
	if _, exists := process.rusageOffset[groupID]; exists {
		return
	}
	process.rusageOffset[groupID] = offset
}

func (process *Process) takePendingTrackedStop() (int, processStop, bool) {
	for pid, stop := range process.pendingStops {
		if !process.HasTracee(pid) {
			continue
		}
		delete(process.pendingStops, pid)
		return pid, stop, true
	}
	return 0, processStop{}, false
}

func (process *Process) rememberPendingStop(pid int, status syscall.WaitStatus, rusage syscall.Rusage) {
	process.ensureStateMaps()
	process.pendingStops[pid] = processStop{
		status: status,
		rusage: rusage,
	}
}

func (process *Process) clearPendingStops() {
	process.ensureStateMaps()
	process.pendingStops = make(map[int]processStop)
}

func (process *Process) recordRusage(pid int, rusage syscall.Rusage) {
	process.ensureStateMaps()
	process.rusageByPid[pid] = rusage
}

func (process *Process) Memory() int64 {
	groupMaxRSS := make(map[int]int64)
	for pid, ru := range process.rusageByPid {
		groupID := pid
		if tgid, ok := process.threadGroups[pid]; ok && tgid > 0 {
			groupID = tgid
		}
		if ru.Maxrss > groupMaxRSS[groupID] {
			groupMaxRSS[groupID] = ru.Maxrss
		}
	}
	var total int64
	for groupID, memory := range groupMaxRSS {
		if offset, ok := process.rusageOffset[groupID]; ok {
			memory -= offset
			if memory < 0 {
				memory = 0
			}
		}
		total += memory
	}
	return total
}

func (process *Process) Exited() bool {
	if process.IsKilled {
		return true
	}
	if process.Status.Exited() {
		log.Debugf("Exited: %#v", process.Rusage)
		return true
	}
	return false
}

func (process *Process) GetTimeCost() int64 {
	// CPU time is intentionally cumulative across all waited tracees. We do not
	// deduplicate by thread group here, because wait4 accounts usage for the
	// specific waited child/task rather than reporting a shared wall-clock value.
	var total int64
	for _, ru := range process.rusageByPid {
		uSec := int64(ru.Utime.Usec) + int64(ru.Stime.Usec)
		total += uSec + (int64(ru.Utime.Sec)+int64(ru.Stime.Sec))*1e6
	}
	return total
}

func (process *Process) Kill() {
	if process.IsKilled {
		return
	}
	log.Debugf("kill, %#v", process.Rusage)
	process.IsKilled = true
	// The launcher places the child into its own process group before exec, so
	// a negative PID targets the entire sandboxed tree in one shot.
	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
	for pid := range process.tracees {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	for pid := range process.pendingStops {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	for process.HasActiveTracees() || process.HasPendingStops() {
		pid, err := syscall.Wait4(-1, &process.Status, waitOptions(), &process.Rusage)
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.ECHILD {
			process.tracees = make(map[int]struct{})
			process.clearPendingStops()
			break
		}
		if err != nil {
			log.Infof("wait kill cleanup failed: %v", err)
			break
		}
		process.CurrentPid = pid
		process.recordRusage(pid, process.Rusage)
		process.RemoveTracee(pid)
	}
}
