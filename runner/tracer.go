package runner

type TracerDetect struct {
	Exit       bool
	prevRax    uint64
	Pid        int
	callPolicy *CallPolicy
}

func (tracer *TracerDetect) setCallPolicy(policy *CallPolicy) {
	tracer.callPolicy = policy
}
