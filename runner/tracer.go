package runner

import (
	"syscall"
)

type TracerDetect struct {
	Exit    bool
	prevRax uint64
	Pid     int
}

func (tracer *TracerDetect) checkSyscall() bool {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(tracer.Pid, &regs)
	if err != nil {
		log.Debugln("trace failed:", err)
		return true
	}

	if tracer.Exit {
		if regs.Orig_rax != syscall.SYS_WRITE && regs.Orig_rax != syscall.SYS_READ {
			if tracer.prevRax != regs.Orig_rax {
				log.Debugf(">>Name %16v", getName(regs.Orig_rax))
			}
			log.Infof("%16X\n", regs.Rax)
		}
	} else {
		log.Debugf(">>Name %16v", getName(regs.Orig_rax))
		// todo:here check
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.Exit = !tracer.Exit
	return false
}
