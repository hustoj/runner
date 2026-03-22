package runner

type TracerDetect struct {
	inSyscall  bool
	prevRax    uint64
	Pid        int
	callPolicy *CallPolicy
}

func (tracer *TracerDetect) setCallPolicy(policy *CallPolicy) {
	tracer.callPolicy = policy
}

func (tracer *TracerDetect) consumeBootstrapCall(callID uint64) {
	if tracer.callPolicy == nil {
		return
	}
	tracer.callPolicy.Consume(callID)
}
