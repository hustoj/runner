package runner

import (
	"fmt"
	"strings"

	"github.com/koding/multiconfig"
)

type TaskConfig struct {
	// Resource limits
	CPU    int `default:"3"`   // CPU time limit in seconds
	Memory int `default:"256"` // Memory limit in MB
	Output int `default:"16"`  // Output size limit in MB
	Stack  int `default:"8"`   // Stack size limit in MB

	// Sandbox security settings
	RunUID int `default:"-1"` // UID to run as (-1 = no privilege drop)
	RunGID int `default:"-1"` // GID to run as (-1 = no privilege drop)

	Command    string `default:"./main"`
	Language   int    `default:"2"`
	WorkDir    string `default:""`      // Working directory (empty = current dir)
	ChrootDir  string `default:""`      // Chroot jail directory (empty = no chroot)
	NoNewPrivs bool   `default:"true"`  // Prevent privilege escalation via setuid binaries
	UseMountNS bool   `default:"false"` // Isolate mount points
	UsePIDNS   bool   `default:"false"` // Isolate process IDs
	UseIPCNS   bool   `default:"false"` // Isolate IPC resources
	UseUTSNS   bool   `default:"false"` // Isolate hostname/domainname
	UseNetNS   bool   `default:"false"` // Isolate network stack

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

// Validate checks if the configuration is valid.
// Returns an error if any required constraints are violated.
func (tc *TaskConfig) Validate() error {
	// Check UID/GID pairing
	if (tc.RunUID >= 0 && tc.RunGID < 0) || (tc.RunUID < 0 && tc.RunGID >= 0) {
		return fmt.Errorf("RunUID and RunGID must both be set or both be -1 (got uid=%d, gid=%d)", tc.RunUID, tc.RunGID)
	}

	// Warn if namespace is enabled without privilege drop
	if tc.RunUID < 0 && (tc.UseMountNS || tc.UsePIDNS || tc.UseIPCNS || tc.UseUTSNS || tc.UseNetNS) {
		log.Warn("Namespaces are enabled but no privilege drop configured - namespace isolation may fail")
	}

	return nil
}

func (tc *TaskConfig) GetCommand() string {
	tc.parseCommand()
	return tc.commands[0]
}

func (tc *TaskConfig) parseCommand() {
	if len(tc.commands) == 0 {
		tc.commands = strings.Split(tc.Command, " ")
	}
}

func (tc *TaskConfig) GetArgs() []string {
	tc.parseCommand()
	return tc.commands[1:]
}

func LoadConfig() *TaskConfig {
	m := multiconfig.NewWithPath("case.json")
	setting = new(TaskConfig)
	m.MustLoad(setting)

	// Validate configuration after loading
	if err := setting.Validate(); err != nil {
		log.Panicf("Invalid configuration: %v", err)
	}

	return setting
}
