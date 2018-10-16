package runner

import (
	"github.com/sirupsen/logrus"
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
			logrus.Infoln("trace failed:", err)
			return true
		}
		if tracer.prevRax != regs.Orig_rax {
			logrus.Debugf(">>Name %16v", getName(regs.Orig_rax))
		}
		logrus.Debugf(">>Value %40X", regs.Rax)
	} else {
		logrus.Debugf(">>Name %16v", getName(regs.Orig_rax))
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		logrus.Debugf("SYS_EXIT_GROUP")
	}
	tracer.Exit = !tracer.Exit
	return false
}
