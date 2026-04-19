//go:build linux

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

	if !tracer.inSyscall(pid) {
		log.Debugf(">>Name %16v", getName(regs.Orig_rax))

		if !tracer.callPolicy.CheckID(regs.Orig_rax) {
			log.Infof("not allowed syscall %d: %16v ", regs.Orig_rax, getName(regs.Orig_rax))
			return true
		}
		if regs.Orig_rax != syscall.SYS_WRITE && regs.Orig_rax != syscall.SYS_READ {
			log.Info(getName(regs.Orig_rax))
		}
	} else {
		if regs.Orig_rax != syscall.SYS_WRITE && regs.Orig_rax != syscall.SYS_READ {
			if state.prevRax != regs.Orig_rax {
				log.Debugf(">>Name %16v", getName(regs.Orig_rax))
			}
			log.Infof("%16X", regs.Rax)
		}
	}
	state.prevRax = regs.Orig_rax
	if state.prevRax == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.setInSyscall(pid, !tracer.inSyscall(pid))
	return false
}
