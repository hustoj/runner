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
			return true
		}
		if tracer.prevRax != regs.Orig_rax {
			logrus.Infof(">>Name %16v", getName(regs.Orig_rax))
		}
		logrus.Infof(">>Value %40X", regs.Rax)
	} else {
		logrus.Infof(">>Name %16v", getName(regs.Orig_rax))
	}
	tracer.prevRax = regs.Orig_rax
	if tracer.prevRax == syscall.SYS_EXIT_GROUP {
		logrus.Infof("SYS_EXIT_GROUP")
	}
	tracer.Exit = !tracer.Exit
	return false
}
