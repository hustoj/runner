package runner

import "syscall"

type traceeState struct {
	inSyscall         bool
	prevRax           uint64 //nolint:unused // accessed in tracer_linux.go
	pendingAttachStop bool
}

type TracerDetect struct {
	callPolicy *CallPolicy
	tracees    map[int]*traceeState
}

func (tracer *TracerDetect) setCallPolicy(policy *CallPolicy) {
	tracer.callPolicy = policy
}

func (tracer *TracerDetect) getTracee(pid int) (*traceeState, bool) {
	if tracer.tracees == nil {
		return nil, false
	}
	state, ok := tracer.tracees[pid]
	return state, ok
}

func (tracer *TracerDetect) ensureTracee(pid int) *traceeState {
	if tracer.tracees == nil {
		tracer.tracees = make(map[int]*traceeState)
	}
	state, ok := tracer.tracees[pid]
	if !ok {
		state = &traceeState{}
		tracer.tracees[pid] = state
	}
	return state
}

func (tracer *TracerDetect) RegisterTracee(pid int, pendingAttachStop bool) {
	state := tracer.ensureTracee(pid)
	state.pendingAttachStop = pendingAttachStop
}

func (tracer *TracerDetect) RemoveTracee(pid int) {
	delete(tracer.tracees, pid)
}

func (tracer *TracerDetect) HasTracee(pid int) bool {
	_, ok := tracer.getTracee(pid)
	return ok
}

func (tracer *TracerDetect) ConsumeAttachStop(pid int, status syscall.WaitStatus) bool {
	state, ok := tracer.getTracee(pid)
	if !ok || !state.pendingAttachStop {
		return false
	}
	if !status.Stopped() || status.StopSignal() != syscall.SIGSTOP {
		return false
	}
	state.pendingAttachStop = false
	return true
}

func (tracer *TracerDetect) FinishPtraceEvent(pid int) {
	if state, ok := tracer.getTracee(pid); ok && state.inSyscall {
		state.inSyscall = false
	}
}

func (tracer *TracerDetect) consumeBootstrapCall(callID uint64) {
	if tracer.callPolicy == nil {
		return
	}
	tracer.callPolicy.Consume(callID)
}
