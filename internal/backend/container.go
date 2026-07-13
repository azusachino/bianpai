package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Container drives Apple's `container` CLI (macOS, Apple Silicon).
//
// It mirrors WSLC's argv-rendering approach. Build/Run and most flags are
// identical; only a handful of subcommands differ (image pull, delete,
// network/volume delete) and List has to filter client-side because
// `container list` has no --filter flag.
type Container struct {
	runner Runner
	binary string
}

var _ Backend = (*Container)(nil)

func NewContainer(runner Runner) *Container {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Container{runner: runner, binary: "container"}
}

func (b *Container) Check(ctx context.Context, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "--version"}, nil, stdout, stderr)
}

func (b *Container) Build(ctx context.Context, req BuildRequest, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, b.BuildArgv(req), nil, stdout, stderr)
}

func (b *Container) Pull(ctx context.Context, image string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "image", "pull", image}, nil, stdout, stderr)
}

func (b *Container) Run(ctx context.Context, req RunRequest, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, b.RunArgv(req), nil, stdout, stderr)
}

func (b *Container) Stop(ctx context.Context, container string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "stop", container}, nil, stdout, stderr)
}

func (b *Container) Remove(ctx context.Context, container string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "delete", container}, nil, stdout, stderr)
}

func (b *Container) Exec(ctx context.Context, container string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	argv := append([]string{b.binary, "exec", container}, command...)
	return b.runner.Run(ctx, argv, stdin, stdout, stderr)
}

func (b *Container) Logs(ctx context.Context, container string, follow bool, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "logs"}
	if follow {
		argv = append(argv, "--follow")
	}
	argv = append(argv, container)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *Container) CreateNetwork(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "network", "create"}
	for _, label := range labels {
		argv = append(argv, "--label", label)
	}
	argv = append(argv, name)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *Container) RemoveNetwork(ctx context.Context, name string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "network", "delete", name}, nil, stdout, stderr)
}

func (b *Container) CreateVolume(ctx context.Context, name string, labels []string, stdout, stderr io.Writer) error {
	argv := []string{b.binary, "volume", "create"}
	for _, label := range labels {
		argv = append(argv, "--label", label)
	}
	argv = append(argv, name)
	return b.runner.Run(ctx, argv, nil, stdout, stderr)
}

func (b *Container) RemoveVolume(ctx context.Context, name string, stdout, stderr io.Writer) error {
	return b.runner.Run(ctx, []string{b.binary, "volume", "delete", name}, nil, stdout, stderr)
}

// SupportsNetworks reports whether the host supports container networks, which
// gates container-to-container communication. The `container network`
// subcommands only exist on macOS 26+; on older macOS versions like macOS 15,
// this probe will fail.
func (b *Container) SupportsNetworks(ctx context.Context) bool {
	return b.runner.Run(ctx, []string{b.binary, "network", "ls"}, nil, io.Discard, io.Discard) == nil
}

// containerListEntry represents the JSON schema subset returned by `container list --format json`.
type containerListEntry struct {
	ID            string `json:"id"`
	Configuration struct {
		Labels map[string]string `json:"labels"`
		Image  struct {
			Reference string `json:"reference"`
		} `json:"image"`
	} `json:"configuration"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
}

// List renders project containers. Since the Apple `container` CLI does not support
// server-side filtering via `--filter` (unlike WSLC), we fetch the full list of
// containers in JSON format, parse it, and perform client-side filtering using
// the com.bianpai.project label.
func (b *Container) List(ctx context.Context, project string, stdout, stderr io.Writer) error {
	var buf bytes.Buffer
	if err := b.runner.Run(ctx, []string{b.binary, "list", "--all", "--format", "json"}, nil, &buf, stderr); err != nil {
		return err
	}

	var entries []containerListEntry
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		return fmt.Errorf("parsing container list: %w", err)
	}

	w := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tIMAGE\tSTATE"); err != nil {
		return err
	}
	for _, e := range entries {
		// Filter out containers that do not belong to the current bianpai project
		if e.Configuration.Labels[projectLabelKey] != project {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", e.ID, e.Configuration.Image.Reference, e.Status.State); err != nil {
			return err
		}
	}
	return w.Flush()
}

// projectLabelKey is the label key used to group resources belonging to the same project.
const projectLabelKey = "com.bianpai.project"

func (b *Container) BuildArgv(req BuildRequest) []string {
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

func (b *Container) RunArgv(req RunRequest) []string {
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
	argv = append(argv, runCommandArgs(req)...)
	return argv
}
