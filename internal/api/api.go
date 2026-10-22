// Package api holds the types shared by the CLI and the daemon. They are
// encoded as newline-delimited JSON over the control socket.
package api

import "time"

// Commands understood by the daemon.
const (
	CmdPing     = "ping"
	CmdUp       = "up"
	CmdDown     = "down"
	CmdReload   = "reload"
	CmdStart    = "start"
	CmdStop     = "stop"
	CmdRestart  = "restart"
	CmdPS       = "ps"
	CmdList     = "ls"
	CmdLogs     = "logs"
	CmdShutdown = "shutdown"
)

// Request is sent by the CLI. One request per connection.
type Request struct {
	Cmd string `json:"cmd"`

	// Project selection. Project wins over ConfigPath. When both are empty
	// the daemon falls back to the only loaded project, if there is one.
	Project    string `json:"project,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`

	// Apps to act on. Entries may be "app" or "project/app".
	Apps []string `json:"apps,omitempty"`

	// All targets every loaded project (ps, down).
	All bool `json:"all,omitempty"`

	// Env is the CLI's environment, used as the base environment for apps
	// started by up/reload.
	Env []string `json:"env,omitempty"`

	// Logs options.
	Follow bool `json:"follow,omitempty"`
	Lines  int  `json:"lines,omitempty"`
}

// Message is sent by the daemon. A response is one or more messages, the
// last of which has Done set (or Error, which implies Done).
type Message struct {
	Error    string          `json:"error,omitempty"`
	Text     string          `json:"text,omitempty"`
	Apps     []AppStatus     `json:"apps,omitempty"`
	Projects []ProjectStatus `json:"projects,omitempty"`
	Log      *LogLine        `json:"log,omitempty"`
	Done     bool            `json:"done,omitempty"`
}

// App states.
const (
	StateStopped = "stopped"
	StateWaiting = "waiting" // waiting for dependencies
	StateRunning = "running"
	StateBackoff = "backoff" // crashed, restart scheduled
	StateExited  = "exited"  // exited and will not be restarted
	StateFailed  = "failed"  // could not start, or gave up restarting
)

// Health states (only for apps with a healthcheck).
const (
	HealthStarting  = "starting"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

type AppStatus struct {
	Project   string    `json:"project"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	Health    string    `json:"health,omitempty"`
	PID       int       `json:"pid,omitempty"`
	StartedAt time.Time `json:"started_at,omitzero"`
	Restarts  int       `json:"restarts"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	LastError string    `json:"last_error,omitempty"`
}

type ProjectStatus struct {
	Name       string    `json:"name"`
	ConfigPath string    `json:"config_path"`
	LoadedAt   time.Time `json:"loaded_at"`
	Apps       int       `json:"apps"`
	Running    int       `json:"running"`
}

// Log streams.
const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
	StreamSystem = "watchd" // lifecycle events and Lua print() output
)

type LogLine struct {
	Time    time.Time `json:"time"`
	Project string    `json:"project"`
	App     string    `json:"app"`
	Stream  string    `json:"stream"`
	Text    string    `json:"text"`
}
