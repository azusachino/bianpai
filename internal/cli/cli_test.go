package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bianpai/bianpai/internal/backend"
	"github.com/spf13/cobra"
)

func TestNewBackendSelectsByName(t *testing.T) {
	if b, err := newBackend("container"); err != nil {
		t.Fatalf("container: %v", err)
	} else if _, ok := b.(*backend.Container); !ok {
		t.Fatalf("container -> %T, want *backend.Container", b)
	}

	if b, err := newBackend("wslc"); err != nil {
		t.Fatalf("wslc: %v", err)
	} else if _, ok := b.(*backend.WSLC); !ok {
		t.Fatalf("wslc -> %T, want *backend.WSLC", b)
	}

	b, err := newBackend("")
	switch runtime.GOOS {
	case "windows":
		if err != nil {
			t.Fatalf("default: %v", err)
		}
		if _, ok := b.(*backend.WSLC); !ok {
			t.Fatalf("default -> %T, want *backend.WSLC", b)
		}
	case "darwin":
		if err != nil {
			t.Fatalf("default: %v", err)
		}
		if _, ok := b.(*backend.Container); !ok {
			t.Fatalf("default -> %T, want *backend.Container", b)
		}
	default:
		if err == nil {
			t.Fatalf("default -> %T, want error on %s", b, runtime.GOOS)
		}
	}

	if _, err := newBackend("bogus"); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestRootCommandParsesPersistentFlags(t *testing.T) {
	a := &app{}
	cmd := a.rootCommand(context.Background())

	if err := cmd.ParseFlags([]string{
		"--file", "compose.yaml",
		"--project-name", "demo",
		"--backend", "wslc",
	}); err != nil {
		t.Fatal(err)
	}

	if a.file != "compose.yaml" || a.projectName != "demo" || a.backendName != "wslc" {
		t.Fatalf("persistent flags = file %q, project %q, backend %q", a.file, a.projectName, a.backendName)
	}
}

func TestCommandOptions(t *testing.T) {
	buildCompose := writeCompose(t, `name: demo
services:
  web:
    image: nginx:alpine
    build: .
`)
	volumeCompose := writeCompose(t, `name: demo
services:
  web:
    image: nginx:alpine
    volumes:
      - data:/data
volumes:
  data:
`)

	tests := []struct {
		name  string
		args  []string
		make  func(*optionRecordingBackend) *cobra.Command
		check func(*testing.T, *optionRecordingBackend)
	}{
		{
			name: "up build",
			args: []string{"-d", "--build"},
			make: func(be *optionRecordingBackend) *cobra.Command {
				a := testApp(be, buildCompose)
				return a.upCommand(context.Background())
			},
			check: func(t *testing.T, be *optionRecordingBackend) {
				if len(be.builds) != 1 {
					t.Fatalf("build calls = %d, want 1", len(be.builds))
				}
			},
		},
		{
			name: "down volumes",
			args: []string{"-v"},
			make: func(be *optionRecordingBackend) *cobra.Command {
				a := testApp(be, volumeCompose)
				return a.downCommand(context.Background())
			},
			check: func(t *testing.T, be *optionRecordingBackend) {
				if len(be.removedVolumes) != 1 || be.removedVolumes[0] != "demo_data" {
					t.Fatalf("removed volumes = %#v, want [demo_data]", be.removedVolumes)
				}
			},
		},
		{
			name: "logs follow",
			args: []string{"--follow", "web"},
			make: func(be *optionRecordingBackend) *cobra.Command {
				a := testApp(be, buildCompose)
				return a.logsCommand(context.Background())
			},
			check: func(t *testing.T, be *optionRecordingBackend) {
				if len(be.logsFollow) != 1 || !be.logsFollow[0] {
					t.Fatalf("logs follow = %#v, want [true]", be.logsFollow)
				}
			},
		},
		{
			name: "build flags",
			args: []string{"--no-cache", "--pull", "web"},
			make: func(be *optionRecordingBackend) *cobra.Command {
				a := testApp(be, buildCompose)
				return a.buildCommand(context.Background())
			},
			check: func(t *testing.T, be *optionRecordingBackend) {
				if len(be.builds) != 1 {
					t.Fatalf("build calls = %d, want 1", len(be.builds))
				}
				if !be.builds[0].NoCache || !be.builds[0].Pull {
					t.Fatalf("build request flags = no-cache:%v pull:%v, want true/true", be.builds[0].NoCache, be.builds[0].Pull)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := &optionRecordingBackend{}
			cmd := tt.make(be)
			cmd.SetArgs(tt.args)
			if err := cmd.ExecuteContext(context.Background()); err != nil {
				t.Fatal(err)
			}
			tt.check(t, be)
		})
	}
}

func TestExecRequiresServiceAndCommand(t *testing.T) {
	cmd := testApp(&optionRecordingBackend{}, writeTestCompose(t)).execCommand(context.Background())
	cmd.SetArgs([]string{"web"})
	if err := cmd.ExecuteContext(context.Background()); err == nil || err.Error() != "exec requires SERVICE and COMMAND" {
		t.Fatalf("error = %v, want missing command error", err)
	}
}

func testApp(be backend.Backend, composePath string) *app {
	return &app{
		stdin:  bytes.NewReader(nil),
		stdout: io.Discard,
		stderr: io.Discard,
		be:     be,
		file:   composePath,
	}
}

func TestDownSuppressesBestEffortCleanupErrors(t *testing.T) {
	composePath := writeTestCompose(t)

	var stdout, stderr bytes.Buffer
	a := &app{
		stdin:  bytes.NewReader(nil),
		stdout: &stdout,
		stderr: &stderr,
		be:     noisyCleanupBackend{},
		file:   composePath,
	}
	cmd := a.downCommand(context.Background())
	cmd.SetArgs(nil)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := "Stopping container demo_web_1 ...\nNot running\nRemoving container demo_web_1 ...\nNot found\nRemoving network demo_default ...\nNot found\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestServiceCommandsOwnNotRunningMessage(t *testing.T) {
	composePath := writeTestCompose(t)
	var stdout, stderr bytes.Buffer
	a := &app{
		stdin:  bytes.NewReader(nil),
		stdout: &stdout,
		stderr: &stderr,
		be:     noisyCleanupBackend{},
		file:   composePath,
	}

	cmd := a.logsCommand(context.Background())
	cmd.SetArgs([]string{"web"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected logs error")
	}
	if got, want := err.Error(), `service "web" is not running`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if strings.Contains(stderr.String(), "internalError") {
		t.Fatalf("stderr leaked backend internals: %q", stderr.String())
	}
}

func TestContainerUpRejectsServiceDNSProject(t *testing.T) {
	composePath := writeCompose(t, `name: demo
services:
  api:
    image: app
    networks: [appnet]
  db:
    image: postgres
    networks: [appnet]
networks:
  appnet:
`)
	runner := &recordingRunner{}
	err := executeUpWithContainer(t, composePath, runner)
	if err == nil {
		t.Fatal("expected service DNS error")
	}
	if !strings.Contains(err.Error(), "does not support Compose service DNS") {
		t.Fatalf("error = %q", err.Error())
	}
	if runner.calls != 0 {
		t.Fatalf("backend was called %d times before capability validation failed", runner.calls)
	}
}

func TestContainerUpRejectsHostname(t *testing.T) {
	composePath := writeCompose(t, `name: demo
services:
  web:
    image: nginx
    hostname: web
`)
	runner := &recordingRunner{}
	err := executeUpWithContainer(t, composePath, runner)
	if err == nil {
		t.Fatal("expected hostname error")
	}
	if !strings.Contains(err.Error(), "does not support hostname") {
		t.Fatalf("error = %q", err.Error())
	}
	if runner.calls != 0 {
		t.Fatalf("backend was called %d times before capability validation failed", runner.calls)
	}
}

func TestContainerUpRejectsNetworkAliases(t *testing.T) {
	composePath := writeCompose(t, `name: demo
services:
  web:
    image: nginx
    networks:
      appnet:
        aliases: [frontend]
networks:
  appnet:
`)
	runner := &recordingRunner{}
	err := executeUpWithContainer(t, composePath, runner)
	if err == nil {
		t.Fatal("expected network alias error")
	}
	if !strings.Contains(err.Error(), "does not support network aliases") {
		t.Fatalf("error = %q", err.Error())
	}
	if runner.calls != 0 {
		t.Fatalf("backend was called %d times before capability validation failed", runner.calls)
	}
}

func TestBackendErrorCleansRuntimeInternals(t *testing.T) {
	err := backendError("remove service web", `Error: internalError: "failed to delete container" (cause: "notFound: container missing")`, errors.New("exit status 1"))
	got := err.Error()
	if strings.Contains(got, "internalError") || strings.Contains(got, `"`) {
		t.Fatalf("error leaked backend internals: %q", got)
	}
	if !strings.Contains(got, "remove service web failed:") || !strings.Contains(got, "notFound") {
		t.Fatalf("unexpected cleaned error: %q", got)
	}
}

func writeTestCompose(t *testing.T) string {
	t.Helper()
	return writeCompose(t, `name: demo
services:
  web:
    image: nginx:alpine
    networks:
      - default
networks:
  default:
`)
}

func writeCompose(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	composePath := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return composePath
}

func executeUpWithContainer(t *testing.T, composePath string, runner *recordingRunner) error {
	t.Helper()
	var stdout, stderr bytes.Buffer
	a := &app{
		stdin:  bytes.NewReader(nil),
		stdout: &stdout,
		stderr: &stderr,
		be:     backend.NewContainer(runner),
		file:   composePath,
	}
	cmd := a.upCommand(context.Background())
	cmd.SetArgs([]string{"--detach"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd.ExecuteContext(context.Background())
}

type recordingRunner struct {
	calls int
}

func (r *recordingRunner) Run(context.Context, []string, io.Reader, io.Writer, io.Writer) error {
	r.calls++
	return nil
}

type noisyCleanupBackend struct{}

type optionRecordingBackend struct {
	noisyCleanupBackend
	builds         []backend.BuildRequest
	logsFollow     []bool
	removedVolumes []string
}

func (b *optionRecordingBackend) Build(_ context.Context, req backend.BuildRequest, _, _ io.Writer) error {
	b.builds = append(b.builds, req)
	return nil
}

func (b *optionRecordingBackend) Logs(_ context.Context, _ string, follow bool, _, _ io.Writer) error {
	b.logsFollow = append(b.logsFollow, follow)
	return nil
}

func (b *optionRecordingBackend) RemoveVolume(_ context.Context, name string, _, _ io.Writer) error {
	b.removedVolumes = append(b.removedVolumes, name)
	return nil
}

func (noisyCleanupBackend) Check(context.Context, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) Build(context.Context, backend.BuildRequest, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) Pull(context.Context, string, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) Run(context.Context, backend.RunRequest, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) Stop(_ context.Context, _ string, _ io.Writer, stderr io.Writer) error {
	_, _ = io.WriteString(stderr, "internalError\n")
	return errors.New("missing container")
}

func (noisyCleanupBackend) Remove(_ context.Context, _ string, _ io.Writer, stderr io.Writer) error {
	_, _ = io.WriteString(stderr, "internalError\n")
	return errors.New("missing container")
}

func (noisyCleanupBackend) Exec(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) Logs(_ context.Context, _ string, _ bool, _ io.Writer, stderr io.Writer) error {
	_, _ = io.WriteString(stderr, `Error: internalError: "failed to read logs" (cause: "notFound: container missing")`)
	return errors.New("missing container")
}

func (noisyCleanupBackend) List(context.Context, string, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) CreateNetwork(context.Context, string, []string, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) RemoveNetwork(_ context.Context, _ string, _ io.Writer, stderr io.Writer) error {
	_, _ = io.WriteString(stderr, "internalError\n")
	return errors.New("missing network")
}

func (noisyCleanupBackend) CreateVolume(context.Context, string, []string, io.Writer, io.Writer) error {
	return nil
}

func (noisyCleanupBackend) RemoveVolume(_ context.Context, _ string, _ io.Writer, stderr io.Writer) error {
	_, _ = io.WriteString(stderr, "internalError\n")
	return errors.New("missing volume")
}

func TestExecPassesFlagsUnmodified(t *testing.T) {
	composePath := writeTestCompose(t)
	var stdout, stderr bytes.Buffer

	execBackend := &recordingExecBackend{}

	a := &app{
		stdin:  bytes.NewReader(nil),
		stdout: &stdout,
		stderr: &stderr,
		be:     execBackend,
		file:   composePath,
	}

	cmd := a.execCommand(context.Background())
	cmd.SetArgs([]string{"web", "pg_isready", "-U", "app", "-d", "app"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if got, want := execBackend.lastCommand, []string{"pg_isready", "-U", "app", "-d", "app"}; !equalSlice(got, want) {
		t.Fatalf("exec command = %v, want %v", got, want)
	}
}

type recordingExecBackend struct {
	noisyCleanupBackend
	lastCommand []string
}

func (b *recordingExecBackend) Exec(ctx context.Context, container string, command []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b.lastCommand = command
	return nil
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAlreadyExistsErrorsAreIgnoredDuringUp(t *testing.T) {
	composePath := writeTestCompose(t)
	var stdout, stderr bytes.Buffer

	errBackend := &alreadyExistsErrorBackend{}

	a := &app{
		stdin:  bytes.NewReader(nil),
		stdout: &stdout,
		stderr: &stderr,
		be:     errBackend,
		file:   composePath,
	}

	cmd := a.upCommand(context.Background())
	cmd.SetArgs([]string{"--detach"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("expected up to succeed, got: %v", err)
	}
}

type alreadyExistsErrorBackend struct {
	noisyCleanupBackend
}

func (alreadyExistsErrorBackend) CreateNetwork(context.Context, string, []string, io.Writer, io.Writer) error {
	return errors.New("network already exists")
}

func (alreadyExistsErrorBackend) CreateVolume(context.Context, string, []string, io.Writer, io.Writer) error {
	return errors.New("volume already exists")
}
