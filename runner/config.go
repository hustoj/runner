package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/koding/multiconfig"
)

type TaskConfig struct {
	// Resource limits
	CPU           int `default:"3"`   // CPU time limit in seconds
	Memory        int `default:"256"` // Judged memory limit in MB
	MemoryReserve int `default:"32"`  // Extra MB reserved for RLIMIT_DATA / RLIMIT_AS
	Output        int `default:"16"`  // Output size limit in MB
	Stack         int `default:"8"`   // Stack size limit in MB

	// Sandbox security settings
	RunUID int `default:"-1"` // UID to run as (-1 = no privilege drop)
	RunGID int `default:"-1"` // GID to run as (-1 = no privilege drop)

	Command    string   `default:"./main"`
	Args       []string `default:""` // Explicit arguments; takes precedence over parsing Command
	Language   int      `default:"2"`
	WorkDir    string   `default:""`      // Working directory (empty = current dir)
	ChrootDir  string   `default:""`      // Chroot jail directory (empty = no chroot)
	NoNewPrivs bool     `default:"true"`  // Prevent privilege escalation via setuid binaries
	UseMountNS bool     `default:"false"` // Isolate mount points
	UsePIDNS   bool     `default:"false"` // Reserved: current launcher cannot realize PID namespaces without an extra fork
	UseIPCNS   bool     `default:"false"` // Isolate IPC resources
	UseUTSNS   bool     `default:"false"` // Isolate hostname/domainname
	UseNetNS   bool     `default:"false"` // Isolate network stack

	OneTimeCalls  []string `default:"execve"`
	AllowedCalls  []string `default:"read,write,brk,fstat,uname,mmap,arch_prctl,exit_group,readlink,access,mprotect"`
	AdditionCalls []string `default:""`
	Verbose       bool     `default:"false"`
	Name          string
	Result        int `default:"4"`

	//LogPath  string `default:"/var/log/runner/runner.log"`
	LogPath  string `default:"/dev/stderr"`
	commands []string
}

const namespacePrivilegeWarning = "Namespaces are enabled but no privilege drop configured - namespace isolation may fail"

// Validate checks if the configuration is valid.
// Returns an error if any required constraints are violated.
func (tc *TaskConfig) Validate() error {
	if tc.CPU < 0 {
		return fmt.Errorf("cpu must be >= 0 (got %d)", tc.CPU)
	}
	if tc.Memory < 0 {
		return fmt.Errorf("memory must be >= 0 (got %d)", tc.Memory)
	}
	if tc.MemoryReserve < 0 {
		return fmt.Errorf("memoryReserve must be >= 0 (got %d)", tc.MemoryReserve)
	}
	if tc.Output < 0 {
		return fmt.Errorf("output must be >= 0 (got %d)", tc.Output)
	}
	if tc.Stack < 0 {
		return fmt.Errorf("stack must be >= 0 (got %d)", tc.Stack)
	}

	// Check UID/GID pairing
	if (tc.RunUID >= 0 && tc.RunGID < 0) || (tc.RunUID < 0 && tc.RunGID >= 0) {
		return fmt.Errorf("runUID and runGID must both be set or both be -1 (got uid=%d, gid=%d)", tc.RunUID, tc.RunGID)
	}

	// Validate syscall names (platform-dependent; only effective on linux/amd64)
	if err := validateSyscallNames("oneTimeCalls", tc.OneTimeCalls); err != nil {
		return err
	}
	if err := validateSyscallNames("allowedCalls", tc.AllowedCalls); err != nil {
		return err
	}
	if err := validateSyscallNames("additionCalls", tc.AdditionCalls); err != nil {
		return err
	}

	return nil
}

// ValidationWarnings returns non-fatal configuration diagnostics.
func (tc *TaskConfig) ValidationWarnings() []string {
	if tc.RunUID < 0 && (tc.UseMountNS || tc.UsePIDNS || tc.UseIPCNS || tc.UseUTSNS || tc.UseNetNS) {
		return []string{namespacePrivilegeWarning}
	}

	return nil
}

// LogValidationWarnings emits non-fatal configuration diagnostics.
func (tc *TaskConfig) LogValidationWarnings() {
	for _, warning := range tc.ValidationWarnings() {
		if log != nil {
			log.Warn(warning)
			continue
		}

		fmt.Fprintf(os.Stderr, "WARN: %s\n", warning)
	}
}

func (tc *TaskConfig) GetCommand() string {
	tc.parseCommand()
	return tc.commands[0]
}

func (tc *TaskConfig) parseCommand() {
	if len(tc.commands) == 0 {
		if len(tc.Args) > 0 {
			tc.commands = append([]string{tc.Command}, tc.Args...)
		} else {
			tc.commands = strings.Fields(tc.Command)
			if len(tc.commands) == 0 {
				tc.commands = []string{tc.Command}
			}
		}
	}
}

func (tc *TaskConfig) GetArgs() []string {
	tc.parseCommand()
	return tc.commands[1:]
}

func LoadConfig() *TaskConfig {
	m := multiconfig.NewWithPath("case.json")
	cfg := new(TaskConfig)
	m.MustLoad(cfg)

	// Validate configuration after loading
	if err := cfg.Validate(); err != nil {
		panic(fmt.Errorf("invalid configuration: %w", err))
	}

	setting = cfg
	return setting
}
