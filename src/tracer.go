package runner

import (
	"syscall"
)

type TracerDetect struct {
	Exit    bool
	prevRax uint64
	Pid     int
}

func (tracer *TracerDetect) detect() bool {
	var regs syscall.PtraceRegs
	err := syscall.PtraceGetRegs(tracer.Pid, &regs)
	if tracer.Exit {
		if err != nil {
			log.Debugln("trace failed:", err)
			return true
		}
		if tracer.prevRax != regs.Orig_rax {
			log.Debugf(">>Name %16v", getName(regs.Orig_rax))
		}
		log.Debugf(">>Value %40X", regs.Rax)
	} else {
		log.Debugf(">>Name %16v", getName(regs.Orig_rax))
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		log.Debugf("SYS_EXIT_GROUP")
	}
	tracer.Exit = !tracer.Exit
	return false
}
