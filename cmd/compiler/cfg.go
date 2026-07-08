package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
	"github.com/koding/multiconfig"
)

const compilerBootstrapConfigEnv = "RUNNER_COMPILER_BOOTSTRAP_CONFIG"

type CompileArgs struct {
	values []string
}

func newCompileArgs(values ...string) *CompileArgs {
	return &CompileArgs{values: append([]string(nil), values...)}
}

func (args CompileArgs) Values() []string {
	values := make([]string, len(args.values))
	copy(values, args.values)
	return values
}

func (args CompileArgs) Len() int {
	return len(args.values)
}

func (args CompileArgs) String() string {
	return strings.Join(args.values, " ")
}

func (args *CompileArgs) Set(value string) error {
	parsed, err := parseCompileArgsValue(value)
	if err != nil {
		return err
	}
	args.values = append(args.values[:0], parsed...)
	return nil
}

func (args CompileArgs) MarshalJSON() ([]byte, error) {
	return json.Marshal(args.values)
}

func (args *CompileArgs) UnmarshalJSON(data []byte) error {
	switch trimmed := strings.TrimSpace(string(data)); trimmed {
	case "null":
		args.values = nil
		return nil
	default:
		var values []string
		if err := json.Unmarshal(data, &values); err == nil {
			args.values = append(args.values[:0], values...)
			return nil
		}

		var legacy string
		if err := json.Unmarshal(data, &legacy); err == nil {
			return args.Set(legacy)
		}
	}

	return fmt.Errorf("compile args must be a JSON string or array of strings")
}

func parseCompileArgsValue(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	return shlex.Split(value)
}

type CompileConfig struct {
	CPU      int          `default:"3"`
	Memory   int          `default:"128"`
	Output   int          `default:"16"`
	Stack    int          `default:"8"`
	MaxProcs int          `default:"32"`
	Command  string       `default:"gcc"`
	Verbose  bool         `default:"false"`
	LogPath  string       `default:""`
	Args     *CompileArgs `default:"main.c -o main -O2 -fmax-errors=10 -Wall --static -lm --std=c99"`

	// Sandbox security settings for the compiler child process.
	// When RunUID/RunGID are both -1 (default), no privilege drop is applied.
	// Root launchers must configure a positive RunUID/RunGID pair instead.
	RunUID     int    `default:"-1"`    // Target UID (-1 = no privilege drop)
	RunGID     int    `default:"-1"`    // Target GID (-1 = no privilege drop)
	ChrootDir  string `default:""`      // Chroot jail directory (empty = no chroot)
	WorkDir    string `default:""`      // Working directory inside chroot (empty = /; must be absolute with chroot)
	NoNewPrivs bool   `default:"true"`  // Required PR_SET_NO_NEW_PRIVS before exec
	UseMountNS bool   `default:"false"` // Isolate mount points
	UseIPCNS   bool   `default:"false"` // Isolate IPC resources
	UseUTSNS   bool   `default:"false"` // Isolate hostname/domainname
	UseNetNS   bool   `default:"false"` // Isolate network stack when supported by launcher privileges

	commands []string
	parseErr error
}

var compilerEffectiveUID = os.Geteuid

const (
	compilerNoNewPrivsRequiredError      = "compiler no_new_privs must be enabled"
	compilerPrivilegedChildRequiredError = "running compiler as root without dropping to unprivileged RunUID/RunGID is unsafe"
)

func encodeBootstrapConfig(config *CompileConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadBootstrapConfig() (*CompileConfig, error) {
	encoded, ok := os.LookupEnv(compilerBootstrapConfigEnv)
	if !ok {
		return nil, fmt.Errorf("%s is not set", compilerBootstrapConfigEnv)
	}
	if encoded == "" {
		return nil, fmt.Errorf("%s is empty", compilerBootstrapConfigEnv)
	}
	var config CompileConfig
	if err := json.Unmarshal([]byte(encoded), &config); err != nil {
		return nil, fmt.Errorf("decode bootstrap config: %w", err)
	}
	return &config, nil
}

func (config *CompileConfig) GetArgs() []string {
	if err := config.parseCommand(); err != nil || len(config.commands) <= 1 {
		return nil
	}
	args := make([]string, len(config.commands)-1)
	copy(args, config.commands[1:])
	return args
}

func (config *CompileConfig) parseCommand() error {
	if len(config.commands) > 0 || config.parseErr != nil {
		return config.parseErr
	}

	switch {
	case config.Args != nil && config.Args.Len() > 0:
		config.commands = append([]string{config.Command}, config.Args.Values()...)
		return nil
	default:
		config.commands, config.parseErr = shlex.Split(config.Command)
		if len(config.commands) == 0 {
			config.commands = []string{config.Command}
		}
		return config.parseErr
	}
}

func (config *CompileConfig) ResolveExec() (string, []string, error) {
	if err := config.parseCommand(); err != nil {
		return "", nil, err
	}
	if len(config.commands) == 0 || config.commands[0] == "" {
		return "", nil, errors.New("empty command")
	}

	command := config.commands[0]
	binary := command
	if !strings.Contains(command, "/") {
		resolved, err := exec.LookPath(command)
		if err != nil {
			return "", nil, err
		}
		binary = resolved
	}

	args := make([]string, 0, len(config.commands))
	args = append(args, binary)
	args = append(args, config.commands[1:]...)

	return binary, args, nil
}

func loadConfig() (*CompileConfig, error) {
	m := multiconfig.NewWithPath("compile.json")
	compileConfig := new(CompileConfig)
	if err := m.Load(compileConfig); err != nil {
		return nil, err
	}
	return compileConfig, nil
}

// ValidateSandbox checks sandbox configuration consistency.
// UID/GID <= 0 means no privilege drop; both must be > 0 to drop.
// ChrootDir must exist as a directory if set.
func (config *CompileConfig) ValidateSandbox() error {
	if err := config.validateResourceLimits(); err != nil {
		return err
	}
	if !config.NoNewPrivs {
		return errors.New(compilerNoNewPrivsRequiredError)
	}
	uidSet := config.RunUID > 0
	gidSet := config.RunGID > 0
	if uidSet != gidSet {
		return fmt.Errorf("runUID and runGID must both be positive or both be <= 0 (got uid=%d, gid=%d)", config.RunUID, config.RunGID)
	}
	if err := config.ValidateLaunchSafety(); err != nil {
		return err
	}
	if config.ChrootDir != "" {
		info, err := os.Stat(config.ChrootDir)
		if err != nil {
			return fmt.Errorf("chrootDir %q: %w", config.ChrootDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("chrootDir %q is not a directory", config.ChrootDir)
		}
	}
	if _, err := config.sandboxWorkDir(); err != nil {
		return err
	}
	return nil
}

func (config *CompileConfig) validateResourceLimits() error {
	if config.CPU < 0 {
		return fmt.Errorf("cpu must be >= 0 (got %d)", config.CPU)
	}
	if config.Memory < 0 {
		return fmt.Errorf("memory must be >= 0 (got %d)", config.Memory)
	}
	if config.Output < 0 {
		return fmt.Errorf("output must be >= 0 (got %d)", config.Output)
	}
	if config.Stack < 0 {
		return fmt.Errorf("stack must be >= 0 (got %d)", config.Stack)
	}
	if config.MaxProcs < 1 {
		return fmt.Errorf("maxProcs must be >= 1 so the compiler can start (got %d)", config.MaxProcs)
	}
	return nil
}

func (config *CompileConfig) ValidateLaunchSafety() error {
	if compilerEffectiveUID() != 0 {
		return nil
	}
	if config.RunUID > 0 && config.RunGID > 0 {
		return nil
	}
	return errors.New(compilerPrivilegedChildRequiredError)
}

func (config *CompileConfig) sandboxWorkDir() (string, error) {
	if config.WorkDir != "" {
		if config.ChrootDir != "" && !filepath.IsAbs(config.WorkDir) {
			return "", fmt.Errorf("workDir %q must be absolute when chrootDir is set", config.WorkDir)
		}
		return config.WorkDir, nil
	}
	if config.ChrootDir != "" {
		return "/", nil
	}
	return "", nil
}
