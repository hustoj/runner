package runner

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

type RunningTask struct {
	setting           *TaskConfig
	process           *Process
	Result            *Result
	timeLimit         int64
	memoryLimit       int64
	outputFileFD      int
	hasOutputFileFD   bool
	taskCtrl          taskController
	wallClockTimedOut atomic.Bool
}

func (task *RunningTask) Init(setting *TaskConfig) {
	task.setting = setting
	task.timeLimit = int64(setting.CPU) * microsPerSecond
	task.memoryLimit = int64(setting.Memory) * 1024

	task.Result = &Result{}
	task.Result.Init()
	task.wallClockTimedOut.Store(false)

	log.Debugf("load case config %#v", task.setting)
	log.Debugf("CPU time limit: %d, wall-clock limit: %d, PeakMemory limit: %d", task.timeLimit, task.wallClockLimitMicros(), task.memoryLimit)
}

func (task *RunningTask) Run() error {
	if task.setting == nil {
		return fmt.Errorf("task setting is nil")
	}
	if err := task.setting.ValidateLaunchSafety(); err != nil {
		return fmt.Errorf("unsafe configuration: %w", err)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := task.runProcess(); err != nil {
		return err
	}
	defer task.cleanupRuntimeResources()
	stopWatchdog := task.startWallClockWatchdog()
	defer stopWatchdog()
	return task.trace()
}

func (task *RunningTask) startWallClockWatchdog() func() {
	timeout := time.Duration(task.setting.effectiveWallClockLimitSeconds()) * time.Second
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	var stopRequested atomic.Bool

	go func() {
		defer close(doneCh)

		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-timer.C:
			if stopRequested.Load() {
				return
			}
			task.wallClockTimedOut.Store(true)
			log.Infof("wall-clock TLE: limit %s reached, killing task", timeout)
			task.killProcessTree()
		case <-stopCh:
			return
		}
	}()

	return func() {
		if stopRequested.CompareAndSwap(false, true) {
			close(stopCh)
		}
		<-doneCh
	}
}

func (task *RunningTask) killProcessTree() {
	if task.process == nil || task.process.Pid <= 0 {
		return
	}

	if task.taskCtrl != nil {
		if err := task.taskCtrl.Kill(); err != nil {
			log.Infof("kill task cgroup failed: %v", err)
		}
	}

	// The launcher places the child into its own process group before exec.
	// The direct PID kill is a fallback for early startup failures or setpgid races.
	_ = syscall.Kill(-task.process.Pid, syscall.SIGKILL)
	_ = syscall.Kill(task.process.Pid, syscall.SIGKILL)
}

func (task *RunningTask) GetResult() *Result {
	log.Debug(task.Result.String())

	return task.Result
}

// abortTrace marks the task as RUNTIME_ERROR, kills the process, and refreshes
// resource stats for the final result check. Callers log their specific reason
// before calling this, then break out of the trace loop or return.
func (task *RunningTask) abortTrace() {
	task.Result.RetCode = RUNTIME_ERROR
	task.process.Kill()
	task.parseRunningInfo()
}

type traceContext struct {
	process      *Process
	tracer       *TracerDetect
	traceSeccomp bool
	resumeMode   traceResumeMode
}

type traceDecision uint8

const (
	traceStop traceDecision = iota
	traceContinue
)

func (task *RunningTask) trace() error {
	process := task.process
	process.IsKilled = false

	ctx, err := task.prepareTraceContext()
	if err != nil {
		return err
	}
	if task.prepareInitialTraceStop(ctx) {
		task.runTraceLoop(ctx)
	}
	return task.finalizeTraceResult()
}

func (task *RunningTask) prepareTraceContext() (traceContext, error) {
	process := task.process
	tracer := &TracerDetect{}
	tracer.RegisterTracee(process.Pid, false)
	syscallPolicy, err := task.setting.compileSyscallPolicy()
	if err != nil {
		process.Kill()
		return traceContext{}, fmt.Errorf("compile syscall policy: %w", err)
	}
	log.Debugf("allowed syscall is: %s", syscallPolicy.Ptrace.AllowedCalls)
	policy, err := makeCallPolicy(syscallPolicy.Ptrace)
	if err != nil {
		process.Kill()
		return traceContext{}, fmt.Errorf("build call policy: %w", err)
	}
	tracer.setCallPolicy(policy)
	tracer.consumeBootstrapCall(syscall.SYS_EXECVE)

	return traceContext{
		process:      process,
		tracer:       tracer,
		traceSeccomp: task.traceSeccompEvents(),
		resumeMode:   task.traceResumeMode(),
	}, nil
}

func (task *RunningTask) prepareInitialTraceStop(ctx traceContext) bool {
	process := ctx.process
	alive, err := process.Wait()
	if err != nil {
		log.Infof("initial wait failed: %v", err)
		task.abortTrace()
		return false
	}
	if !alive {
		return false
	}
	if process.Exited() {
		log.Infof("program exited before tracing loop")
		ctx.tracer.RemoveTracee(process.CurrentPid)
		process.RemoveTracee(process.CurrentPid)
		task.parseRunningInfo()
		task.applyExitCode(process.Status)
		return false
	}
	if !process.IsInitialTraceStop() {
		task.handleBrokenTraceStop("unexpected initial ptrace stop")
		return false
	}
	process.SetThreadGroup(process.Pid, process.Pid)
	process.SetRusageOffset(process.Pid, process.Rusage.Maxrss)
	if err := process.SetPtraceOptions(ctx.traceSeccomp); err != nil {
		log.Infof("PtraceSetOptions: err %v", err)
		task.abortTrace()
		return false
	}
	if !process.ContinueWithMode(ctx.resumeMode, 0) {
		log.Infof("Program not alive after ptrace setup")
		task.parseRunningInfo()
		return false
	}
	return true
}

func (task *RunningTask) runTraceLoop(ctx traceContext) {
	process := ctx.process
	for {
		alive, err := process.Wait()
		if err != nil {
			log.Infof("wait in trace loop failed: %v", err)
			task.abortTrace()
			return
		}
		if !alive {
			return
		}

		if task.handleTraceStop(ctx) == traceStop {
			return
		}
	}
}

func (task *RunningTask) handleTraceStop(ctx traceContext) traceDecision {
	process := ctx.process
	switch {
	case process.Exited():
		return task.handleExitedStop(ctx)
	case ctx.tracer.ConsumeAttachStop(process.CurrentPid, process.Status):
		return task.handleAttachStop(ctx)
	case process.IsPtraceEventStop():
		return task.handlePtraceEventStop(ctx)
	case !process.IsSyscallStop():
		return task.handleSignalOrBrokenStop(ctx)
	default:
		return task.handleSyscallStop(ctx)
	}
}

func (task *RunningTask) handleExitedStop(ctx traceContext) traceDecision {
	process := ctx.process
	log.Infof("program exited! pid=%d", process.CurrentPid)
	ctx.tracer.RemoveTracee(process.CurrentPid)
	process.RemoveTracee(process.CurrentPid)
	task.parseRunningInfo()
	// Only the root process exit code determines the result; child threads
	// (e.g. JVM daemon threads) may legitimately exit with non-zero codes.
	if process.CurrentPid == process.Pid {
		task.applyExitCode(process.Status)
	}
	if !process.HasActiveTracees() {
		return traceStop
	}
	return traceContinue
}

func (task *RunningTask) handleAttachStop(ctx traceContext) traceDecision {
	process := ctx.process
	if err := process.SetPtraceOptions(ctx.traceSeccomp); err != nil {
		log.Infof("PtraceSetOptions(new child): err %v", err)
		task.abortTrace()
		return traceStop
	}
	if !process.ContinueWithMode(ctx.resumeMode, 0) {
		log.Infof("Program not alive after child attach stop")
		task.parseRunningInfo()
		return traceStop
	}
	return traceContinue
}

func (task *RunningTask) handlePtraceEventStop(ctx traceContext) traceDecision {
	process := ctx.process
	if !task.handlePtraceEvent(ctx) {
		return traceStop
	}
	if !process.ContinueWithMode(ctx.resumeMode, 0) {
		log.Infof("Program not alive after ptrace event")
		task.parseRunningInfo()
		return traceStop
	}
	return traceContinue
}

func (task *RunningTask) handleSignalOrBrokenStop(ctx traceContext) traceDecision {
	process := ctx.process
	if !process.Status.Stopped() {
		task.handleBrokenTraceStop("unexpected non-syscall ptrace stop")
		return traceStop
	}

	// Signal-delivery stop: forward the signal so the process can handle it.
	// This is required for runtimes like the JVM that use SIGSEGV internally.
	// If the process has no handler, it will be killed by the signal and the
	// subsequent Signaled() status is caught on the next Wait iteration.
	sig := process.Status.StopSignal()
	task.parseRunningInfo()
	if task.applyOutputLimitSignal(sig) {
		log.Debugf("kill by output limit signal: %v", sig)
		process.Kill()
		return traceStop
	}
	task.checkLimit()
	if process.IsKilled {
		return traceStop
	}
	log.Debugf("forwarding signal %v to pid=%d", sig, process.CurrentPid)
	if !process.ContinueWithMode(ctx.resumeMode, int(sig)) {
		log.Infof("Program not alive after signal forward")
		task.parseRunningInfo()
		return traceStop
	}
	return traceContinue
}

func (task *RunningTask) handleSyscallStop(ctx traceContext) traceDecision {
	process := ctx.process
	checkResult := ctx.tracer.checkSyscall(process.CurrentPid)
	if checkResult == syscallCheckViolation {
		log.Debugf("------- check syscall failed")
		task.abortTrace()
		return traceStop
	}
	if checkResult == syscallCheckTraceeGone {
		log.Debugf("skip syscall inspection for pid=%d because tracee is already gone", process.CurrentPid)
		task.parseRunningInfo()
		task.checkLimit()
		return traceContinue
	}
	if checkResult == syscallCheckTracerError {
		log.Warnf("ptrace register read failed for pid=%d", process.CurrentPid)
		task.abortTrace()
		return traceStop
	}
	// before next ptrace, get result, always pass
	task.parseRunningInfo()
	task.checkLimit()

	if !process.ContinueWithMode(ctx.resumeMode, 0) {
		log.Infof("Program not alive! break")
		return traceStop
	}
	return traceContinue
}

func (task *RunningTask) traceSeccompEvents() bool {
	return task.setting.effectiveSyscallBackend() == syscallBackendHybrid
}

func (task *RunningTask) traceResumeMode() traceResumeMode {
	if task.traceSeccompEvents() {
		return traceResumeEventStops
	}
	return traceResumeSyscallStops
}

func (task *RunningTask) handlePtraceEvent(ctx traceContext) bool {
	process := ctx.process
	switch process.PtraceEvent() {
	case ptraceEventClone, ptraceEventFork, ptraceEventVFork:
		return task.handleForkLikePtraceEvent(ctx)
	case ptraceEventSeccomp:
		return task.handleSeccompEvent(ctx)
	default:
		log.Warnf("unhandled ptrace event %d on pid=%d", process.PtraceEvent(), process.CurrentPid)
	}
	return true
}

func (task *RunningTask) handleForkLikePtraceEvent(ctx traceContext) bool {
	process := ctx.process
	event := process.PtraceEvent()
	newPid, err := process.GetEventPid()
	if err != nil {
		log.Infof("PtraceGetEventMsg failed: %v", err)
		task.abortTrace()
		return false
	}
	process.AddTracee(newPid)
	if event != ptraceEventClone {
		process.SetThreadGroup(newPid, newPid)
	}
	ctx.tracer.RegisterTracee(newPid, true)
	log.Infof("registered traced child pid=%d from event=%d", newPid, event)
	return true
}

func (task *RunningTask) handleSeccompEvent(ctx traceContext) bool {
	process := ctx.process
	checkResult := ctx.tracer.checkSeccompTrace(process.CurrentPid)
	if checkResult == syscallCheckViolation {
		log.Debugf("------- check seccomp-traced syscall failed")
		task.abortTrace()
		return false
	}
	if checkResult == syscallCheckTraceeGone {
		log.Debugf("skip seccomp trace inspection for pid=%d because tracee is already gone", process.CurrentPid)
		return true
	}
	if checkResult == syscallCheckTracerError {
		log.Warnf("ptrace register read failed for seccomp event pid=%d", process.CurrentPid)
		task.abortTrace()
		return false
	}
	return true
}

func (task *RunningTask) handleBrokenTraceStop(reason string) {
	process := task.process
	if process.Status.Stopped() {
		log.Infof("%s: stop signal %v", reason, process.Status.StopSignal())
	} else if process.Status.Signaled() {
		log.Infof("%s: exit signal %v", reason, process.Status.Signal())
	} else {
		log.Infof("%s", reason)
	}
	task.parseRunningInfo()
	if process.Status.Stopped() {
		if !task.applyOutputLimitSignal(process.Status.StopSignal()) {
			task.Result.detectSignal(process.Status.StopSignal())
		}
	} else if process.Status.Signaled() {
		task.applyTerminationSignal(process.Status.Signal())
	} else if !task.promoteResourceLimitResult() {
		log.Warnf("process broken, but cause can't detect")
	}

	log.Debugf("Process broken, will kill process")
	process.Kill()
}

func (task *RunningTask) applyExitCode(status syscall.WaitStatus) {
	if !status.Exited() || status.ExitStatus() == 0 || !task.Result.isAccept() {
		return
	}
	task.Result.RetCode = RUNTIME_ERROR
}

func (task *RunningTask) applyTerminationSignal(signal os.Signal) {
	if !task.Result.isAccept() {
		return
	}
	if task.applyOutputLimitSignal(signal) {
		return
	}
	if signal == syscall.SIGKILL && task.promoteResourceLimitResult() {
		return
	}
	task.Result.detectSignal(signal)
}

func (task *RunningTask) check() {
	log.Debug(task.Result.String())
	task.promoteResourceLimitResult()
}

func (task *RunningTask) checkLimit() {
	if !task.promoteResourceLimitResult() {
		return
	}

	switch task.Result.RetCode {
	case OUTPUT_LIMIT:
		log.Debugf("kill by output limit")
	case MEMORY_LIMIT:
		log.Debugf("kill by memory limit: peak %d, rusage: %d, limit %d", task.Result.PeakMemory, task.Result.RusageMemory, task.memoryLimit)
	case TIME_LIMIT:
		log.Debugf("kill by time limit: current %d, limit %d", task.Result.TimeCost, task.timeLimit)
	}
	task.process.Kill()
}

func (task *RunningTask) promoteResourceLimitResult() bool {
	if !task.Result.isAccept() {
		return false
	}
	if task.promoteOutputLimitResultIfExceeded() {
		return true
	}
	if task.outOfMemory() {
		task.Result.RetCode = MEMORY_LIMIT
		return true
	}
	if task.outOfWallClockTime() || task.outOfTime() {
		task.Result.RetCode = TIME_LIMIT
		return true
	}
	return false
}

func (task *RunningTask) outOfTime() bool {
	isTLE := task.Result.TimeCost > task.timeLimit
	if isTLE {
		log.Infof("TLE: Time limit: %d, time cost: %d", task.timeLimit, task.Result.TimeCost)
	}
	return isTLE
}

func (task *RunningTask) promoteOutputLimitResultIfExceeded() bool {
	currentSize, limit, exceeded := task.outputFileLimitExceeded()
	if !exceeded {
		return false
	}

	log.Infof("OLE: Output limit: %d. Size %d.", limit, currentSize)
	task.truncateOutputFileToLimit(limit, currentSize)
	if task.Result.isAccept() {
		task.Result.RetCode = OUTPUT_LIMIT
	}
	return true
}

func (task *RunningTask) outputFileLimitExceeded() (int64, int64, bool) {
	currentSize, ok := task.outputFileSize()
	if !ok {
		return 0, 0, false
	}
	limit := task.configuredOutputLimitBytes()
	return currentSize, limit, currentSize > limit
}

func (task *RunningTask) outputFileSize() (int64, bool) {
	if !task.hasOutputFileFD || task.setting == nil {
		return 0, false
	}

	var stat syscall.Stat_t
	if err := syscall.Fstat(task.outputFileFD, &stat); err != nil {
		log.Infof("read output file status failed: %v", err)
		return 0, false
	}
	return stat.Size, true
}

func (task *RunningTask) configuredOutputLimitBytes() int64 {
	if task.setting == nil {
		return 0
	}
	return int64(task.setting.Output) << 20
}

func (task *RunningTask) applyOutputLimitSignal(signal os.Signal) bool {
	if signal != syscall.SIGXFSZ {
		return false
	}
	task.truncateOutputFileToConfiguredLimit()
	if task.Result.isAccept() {
		task.Result.RetCode = OUTPUT_LIMIT
	}
	return true
}

func (task *RunningTask) truncateOutputFileToConfiguredLimit() {
	currentSize, ok := task.outputFileSize()
	if !ok {
		return
	}
	task.truncateOutputFileToLimit(task.configuredOutputLimitBytes(), currentSize)
}

func (task *RunningTask) truncateOutputFileToLimit(limit int64, currentSize int64) {
	if currentSize <= limit {
		return
	}
	if err := syscall.Ftruncate(task.outputFileFD, limit); err != nil {
		log.Infof("truncate output file to limit %d failed: %v", limit, err)
	}
}

func (task *RunningTask) closeOutputFile() {
	if !task.hasOutputFileFD {
		return
	}
	_ = syscall.Close(task.outputFileFD)
	task.outputFileFD = 0
	task.hasOutputFileFD = false
}

func (task *RunningTask) outOfWallClockTime() bool {
	if !task.wallClockTimedOut.Load() {
		return false
	}
	log.Infof("TLE: wall-clock time limit: %dus", task.wallClockLimitMicros())
	return true
}

func (task *RunningTask) wallClockLimitMicros() int64 {
	if task.setting != nil {
		return int64(task.setting.effectiveWallClockLimitSeconds()) * microsPerSecond
	}
	return task.timeLimit
}

func (task *RunningTask) outOfMemory() bool {
	if task.taskCtrl == nil {
		isMLE := task.Result.PeakMemory > task.memoryLimit
		if isMLE {
			log.Infof("MLE: Memory Limit: %d. Peak %d, Rusage %d.", task.memoryLimit, task.Result.PeakMemory, task.Result.RusageMemory)
		}
		return isMLE
	}

	status, err := task.taskCtrl.MemoryStatus()
	if err != nil {
		log.Infof("read task memory status failed: %v", err)
		return true
	}
	task.Result.PeakMemory = status.PeakMemoryKB

	isMLE := status.Exceeded()
	if isMLE {
		log.Infof(
			"MLE: Memory limit: %d. Peak %d, OOM %d, OOMKill %d, Rusage %d.",
			task.memoryLimit,
			task.Result.PeakMemory,
			status.OOMCount,
			status.OOMKillCount,
			task.Result.RusageMemory,
		)
	}

	return isMLE
}

func (task *RunningTask) refreshTimeCost() {
	task.Result.TimeCost = task.process.GetTimeCost()
	log.Debugf("current time cost: %dus(1e-6s)", task.Result.TimeCost)
}

func (task *RunningTask) refreshMemory() {
	memory := task.process.Memory()
	log.Debugf("adjusted rusage memory is: %d", memory)
	if memory > task.Result.RusageMemory {
		task.Result.RusageMemory = memory
	}
}

// refreshPeakMemoryFromProc 采样 /proc/<pid>/status 的 VmHWM 按线程组求和。
// 当前内存统计已走 cgroup v2 路径(见 refreshFinalMemoryResult),此方法保留作为 fallback/采样入口。
func (task *RunningTask) refreshPeakMemoryFromProc() (int64, bool) { //nolint:unused // 预留:非 cgroup 场景下的采样回退路径
	groupPeaks := make(map[int]int64)
	sampled := false

	for _, pid := range task.process.ActivePids() {
		info, err := GetProcMemoryInfo(pid)
		if err == nil {
			task.process.SetThreadGroup(pid, info.ThreadGroup)
			if info.PeakMemory > groupPeaks[info.ThreadGroup] {
				groupPeaks[info.ThreadGroup] = info.PeakMemory
			}
			sampled = true
			continue
		}

		tgid, ok := task.process.ThreadGroup(pid)
		if !ok || tgid <= 0 {
			log.Infof("Get status memory failed for pid %d: %v", pid, err)
			continue
		}
		if _, seen := groupPeaks[tgid]; seen {
			continue
		}

		info, fallbackErr := GetProcMemoryInfo(tgid)
		if fallbackErr != nil {
			log.Infof("Get status memory failed for pid %d (tgid %d): %v", pid, tgid, fallbackErr)
			continue
		}
		task.process.SetThreadGroup(pid, info.ThreadGroup)
		if info.PeakMemory > groupPeaks[info.ThreadGroup] {
			groupPeaks[info.ThreadGroup] = info.PeakMemory
		}
		sampled = true
	}

	var total int64
	for _, memory := range groupPeaks {
		total += memory
	}
	return total, sampled
}

func (task *RunningTask) parseRunningInfo() {
	task.refreshTimeCost()
	task.refreshMemory()
}

func (task *RunningTask) finalizeTraceResult() error {
	if err := task.refreshFinalMemoryResult(); err != nil {
		return fmt.Errorf("refresh final memory result: %w", err)
	}
	task.check()
	return nil
}

func (task *RunningTask) refreshFinalMemoryResult() error {
	if task.taskCtrl == nil {
		return nil
	}

	status, err := task.taskCtrl.MemoryStatus()
	if err != nil {
		log.Infof("read final task memory status failed: %v", err)
		return nil
	}
	task.Result.PeakMemory = status.PeakMemoryKB
	return nil
}

func (task *RunningTask) cleanupRuntimeResources() {
	task.closeOutputFile()
	if task.taskCtrl == nil {
		return
	}
	if err := task.taskCtrl.Cleanup(); err != nil {
		log.Warnf("cleanup task cgroup failed: %v", err)
	}
	task.taskCtrl = nil
}
