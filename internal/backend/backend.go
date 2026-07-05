package backend

import (
	"context"
	"io"
	"os"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(argv) == 0 {
		return nil
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

type Backend interface {
	Check(ctx context.Context, stdout, stderr io.Writer) error
	Build(ctx context.Context, req BuildRequest, stdout, stderr io.Writer) error
	Pull(ctx context.Context, image string, stdout, stderr io.Writer) error
	Run(ctx context.Context, req RunRequest, stdout, stderr io.Writer) error
	Stop(ctx context.Context, container string, stdout, stderr io.Writer) error
	Remove(ctx context.Context, container string, stdout, stderr io.Writer) error
	Exec(ctx context.Context, container string, command []string, stdin io.Reader, stdout, stderr io.Writer) error
	Logs(ctx context.Context, container string, follow bool, stdout, stderr io.Writer) error
	List(ctx context.Context, project string, stdout, stderr io.Writer) error
	CreateNetwork(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error
	RemoveNetwork(ctx context.Context, name string, stdout, stderr io.Writer) error
	CreateVolume(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error
	RemoveVolume(ctx context.Context, name string, stdout, stderr io.Writer) error
}

type BuildRequest struct {
	Context    string
	Dockerfile string
	Tag        string
	Args       []string
	Target     string
	Pull       bool
	NoCache    bool
	Labels     []string
}

type RunRequest struct {
	Image        string
	Name         string
	Command      []string
	Entrypoint   []string
	Env          []string
	EnvFiles     []string
	Labels       []string
	Ports        []string
	Volumes      []string
	Networks     []NetworkAttachment
	Hostname     string
	Workdir      string
	User         string
	Memory       string
	CPUs         string
	Interactive  bool
	TTY          bool
	Detach       bool
	RemoveOnExit bool
}

type NetworkAttachment struct {
	Name    string
	Aliases []string
}

func runCommandArgs(req RunRequest) []string {
	if len(req.Entrypoint) <= 1 {
		return append([]string(nil), req.Command...)
	}
	args := append([]string(nil), req.Entrypoint[1:]...)
	return append(args, req.Command...)
}
