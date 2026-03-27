//go:build linux && (amd64 || arm64)

package runner

import (
	"fmt"
	"syscall"

	"github.com/hustoj/runner/sec"
)

func getName(syscallID uint64) string {
	name, err := sec.SCTbl.GetName(int(syscallID))
	if err != nil {
		return fmt.Sprintf("unknown_%d", syscallID)
	}
	return name
}

func (tracer *TracerDetect) checkSyscall() bool {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(tracer.Pid, &regs)
	if err != nil {
		log.Debugf("trace failed: %v", err)
		return true
	}

	callID := getSyscallNumber(&regs)

	if !tracer.inSyscall {
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
			if tracer.prevSyscall != callID {
				log.Debugf(">>Name %16v", getName(callID))
			}
			log.Infof("%16X", getSyscallReturn(&regs))
		}
	}
	tracer.prevSyscall = callID
	if tracer.prevSyscall == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.inSyscall = !tracer.inSyscall
	return false
}
