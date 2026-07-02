package backend

import (
	"context"
	"io"
)

type WSLC struct {
	runner Runner
	binary string
}

func NewWSLC(runner Runner) *WSLC {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &WSLC{runner: runner, binary: "wslc"}
}

func (b *WSLC) Check(ctx context.Context, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "--version"}, nil, stdout, stderr)
}

func (b *WSLC) Build(ctx context.Context, req BuildRequest, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, b.BuildArgv(req), nil, stdout, stderr)
}

func (b *WSLC) Pull(ctx context.Context, image string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "pull", image}, nil, stdout, stderr)
}

func (b *WSLC) Run(ctx context.Context, req RunRequest, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, b.RunArgv(req), nil, stdout, stderr)
}

func (b *WSLC) Stop(ctx context.Context, container string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "stop", container}, nil, stdout, stderr)
}

func (b *WSLC) Remove(ctx context.Context, container string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "remove", container}, nil, stdout, stderr)
}

func (b *WSLC) Exec(ctx context.Context, container string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	argv := append([]string{b.binary, "exec", container}, command...)
	return b.runner.Run(ctx, argv, stdin, stdout, stderr)
}

func (b *WSLC) Logs(ctx context.Context, container string, follow bool, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "logs"}
	if follow {
		argv = append(argv, "--follow")
	}
	argv = append(argv, container)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *WSLC) List(ctx context.Context, project string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "list", "--all", "--format", "json", "--filter", "label=com.bianpai.project=" + project}, nil, stdout, stderr)
}

func (b *WSLC) CreateNetwork(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "network", "create"}
	for _, label := range labels {
		argv = append(argv, "--label", label)
	}
	argv = append(argv, name)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *WSLC) RemoveNetwork(ctx context.Context, name string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "network", "remove", name}, nil, stdout, stderr)
}

func (b *WSLC) CreateVolume(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "volume", "create"}
	for _, label := range labels {
		argv = append(argv, "--label", label)
	}
	argv = append(argv, name)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *WSLC) RemoveVolume(ctx context.Context, name string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "volume", "remove", name}, nil, stdout, stderr)
}

func (b *WSLC) BuildArgv(req BuildRequest) []string {
	argv := []string{b.binary, "build"}
	if req.Tag != "" {
		argv = append(argv, "--tag", req.Tag)
	}
	if req.Dockerfile != "" {
		argv = append(argv, "--file", req.Dockerfile)
	}
	if req.Pull {
		argv = append(argv, "--pull")
	}
	if req.NoCache {
		argv = append(argv, "--no-cache")
	}
	if req.Target != "" {
		argv = append(argv, "--target", req.Target)
	}
	for _, arg := range req.Args {
		argv = append(argv, "--build-arg", arg)
	}
	for _, label := range req.Labels {
		argv = append(argv, "--label", label)
	}
	argv = append(argv, req.Context)
	return argv
}

func (b *WSLC) RunArgv(req RunRequest) []string {
	argv := []string{b.binary, "run"}
	if req.Detach {
		argv = append(argv, "--detach")
	}
	if req.RemoveOnExit {
		argv = append(argv, "--rm")
	}
	if req.Name != "" {
		argv = append(argv, "--name", req.Name)
	}
	for _, env := range req.Env {
		argv = append(argv, "--env", env)
	}
	for _, envFile := range req.EnvFiles {
		argv = append(argv, "--env-file", envFile)
	}
	for _, label := range req.Labels {
		argv = append(argv, "--label", label)
	}
	for _, port := range req.Ports {
		argv = append(argv, "--publish", port)
	}
	for _, volume := range req.Volumes {
		argv = append(argv, "--volume", volume)
	}
	for _, network := range req.Networks {
		argv = append(argv, "--network", network.Name)
		for _, alias := range network.Aliases {
			argv = append(argv, "--network-alias", alias)
		}
	}
	if req.Hostname != "" {
		argv = append(argv, "--hostname", req.Hostname)
	}
	if req.Workdir != "" {
		argv = append(argv, "--workdir", req.Workdir)
	}
	if req.User != "" {
		argv = append(argv, "--user", req.User)
	}
	if req.Memory != "" {
		argv = append(argv, "--memory", req.Memory)
	}
	if req.CPUs != "" {
		argv = append(argv, "--cpus", req.CPUs)
	}
	if len(req.Entrypoint) > 0 {
		argv = append(argv, "--entrypoint", req.Entrypoint[0])
	}
	if req.Interactive {
		argv = append(argv, "--interactive")
	}
	if req.TTY {
		argv = append(argv, "--tty")
	}
	argv = append(argv, req.Image)
	argv = append(argv, req.Command...)
	return argv
}
