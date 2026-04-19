//go:build linux && (amd64 || arm64)

package runner

import (
	"fmt"
	"syscall"

	"github.com/hustoj/runner/sec"
)

func (tracer *TracerDetect) setInSyscall(pid int, inSyscall bool) {
	state := tracer.ensureTracee(pid)
	state.inSyscall = inSyscall
}

func (tracer *TracerDetect) inSyscall(pid int) bool {
	state, ok := tracer.getTracee(pid)
	return ok && state.inSyscall
}

func getName(syscallID uint64) string {
	name, err := sec.SCTbl.GetName(int(syscallID))
	if err != nil {
		return fmt.Sprintf("unknown_%d", syscallID)
	}
	return name
}

func (tracer *TracerDetect) checkSyscall(pid int) bool {
	state, ok := tracer.getTracee(pid)
	if !ok {
		log.Warnf("checkSyscall called for unregistered pid %d", pid)
		return true
	}
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(pid, &regs)
	if err != nil {
		log.Debugf("trace failed: %v", err)
		return true
	}

	callID := getSyscallNumber(&regs)

	if !tracer.inSyscall(pid) {
		log.Debugf(">>Name %16v", getName(callID))

		if !tracer.callPolicy.CheckID(callID) {
			log.Infof("not allowed syscall %d: %16v ", callID, getName(callID))
			return true
		}
		if callID != syscall.SYS_WRITE && callID != syscall.SYS_READ {
			log.Info(getName(callID))
		}
	} else {
		if callID != syscall.SYS_WRITE && callID != syscall.SYS_READ {
			if state.prevSyscall != callID {
				log.Debugf(">>Name %16v", getName(callID))
			}
			log.Infof("%16X", getSyscallReturn(&regs))
		}
	}
	state.prevSyscall = callID
	if state.prevSyscall == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.setInSyscall(pid, !tracer.inSyscall(pid))
	return false
}
