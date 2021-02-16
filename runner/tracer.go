package runner

import (
	"syscall"
)

type TracerDetect struct {
	Exit       bool
	prevRax    uint64
	Pid        int
	callPolicy *CallPolicy
}

func (tracer *TracerDetect) setCallPolicy(policy *CallPolicy) {
	tracer.callPolicy = policy
}

func (tracer *TracerDetect) checkSyscall() bool {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(tracer.Pid, &regs)
	if err != nil {
		log.Debugf("trace failed: %v", err)
		return true
	}

	if tracer.Exit {
		if regs.Orig_rax != syscall.SYS_WRITE && regs.Orig_rax != syscall.SYS_READ {
			if tracer.prevRax != regs.Orig_rax {
				log.Debugf(">>Name %16v", getName(regs.Orig_rax))
			}
			log.Infof("%16X", regs.Rax)
		}
	} else {
		log.Debugf(">>Name %16v", getName(regs.Orig_rax))

		if !tracer.callPolicy.CheckID(regs.Orig_rax) {
			log.Infof("not allowed syscall %d: %16v ", regs.Orig_rax, getName(regs.Orig_rax))
			return true
		}
		if regs.Orig_rax != syscall.SYS_WRITE && regs.Orig_rax != syscall.SYS_READ {
			log.Info(getName(regs.Orig_rax))
		}
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.Exit = !tracer.Exit
	return false
}
