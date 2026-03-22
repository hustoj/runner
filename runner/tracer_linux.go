//go:build linux

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

	if !tracer.inSyscall {
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
			if tracer.prevRax != regs.Orig_rax {
				log.Debugf(">>Name %16v", getName(regs.Orig_rax))
			}
			log.Infof("%16X", regs.Rax)
		}
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.inSyscall = !tracer.inSyscall
	return false
}
