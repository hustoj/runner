//go:build linux && (amd64 || arm64)

package runner

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/hustoj/runner/sec"
)

var ptraceGetRegs = syscall.PtraceGetRegs

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

func (tracer *TracerDetect) checkSyscall(pid int) syscallCheckResult {
	state, ok := tracer.getTracee(pid)
	if !ok {
		log.Warnf("checkSyscall called for unregistered pid %d", pid)
		return syscallCheckTraceeGone
	}
	var regs syscall.PtraceRegs
	err := ptraceGetRegs(pid, &regs)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			log.Debugf("tracee pid=%d disappeared before regs fetch", pid)
			return syscallCheckTraceeGone
		}
		log.Debugf("trace failed for pid=%d: %v", pid, err)
		return syscallCheckTracerError
	}

	callID := getSyscallNumber(&regs)

	if !tracer.inSyscall(pid) {
		log.Debugf(">>Name %16v", getName(callID))

		if state.seccompPrechecked && state.seccompPrecheckedSysno == callID {
			state.seccompPrechecked = false
			state.seccompPrecheckedSysno = 0
			log.Debugf("syscall %s(%d) already checked by seccomp trace event", getName(callID), callID)
		} else if !tracer.callPolicy.CheckID(callID) {
			state.seccompPrechecked = false
			state.seccompPrecheckedSysno = 0
			log.Infof("not allowed syscall %d: %16v ", callID, getName(callID))
			return syscallCheckViolation
		} else {
			state.seccompPrechecked = false
			state.seccompPrecheckedSysno = 0
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
	return syscallCheckOK
}

func (tracer *TracerDetect) checkSeccompTrace(pid int) syscallCheckResult {
	state, ok := tracer.getTracee(pid)
	if !ok {
		log.Warnf("checkSeccompTrace called for unregistered pid %d", pid)
		return syscallCheckTraceeGone
	}

	var regs syscall.PtraceRegs
	err := ptraceGetRegs(pid, &regs)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			log.Debugf("tracee pid=%d disappeared before seccomp trace regs fetch", pid)
			return syscallCheckTraceeGone
		}
		log.Debugf("seccomp trace regs failed for pid=%d: %v", pid, err)
		return syscallCheckTracerError
	}

	callID := getSyscallNumber(&regs)
	log.Debugf("seccomp trace event pid=%d syscall=%s(%d)", pid, getName(callID), callID)
	if tracer.inSyscall(pid) {
		return syscallCheckOK
	}
	if !tracer.callPolicy.CheckID(callID) {
		log.Infof("not allowed seccomp-traced syscall %d: %16v ", callID, getName(callID))
		return syscallCheckViolation
	}
	state.seccompPrechecked = true
	state.seccompPrecheckedSysno = callID
	return syscallCheckOK
}
