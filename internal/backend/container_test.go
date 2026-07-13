package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

// container network subcommands only exist on macOS 26; probing them is how we
// detect whether container-to-container networking is available.
func TestContainerSupportsNetworks(t *testing.T) {
	ok := &captureRunner{}
	if !NewContainer(ok).SupportsNetworks(context.Background()) {
		t.Error("expected true when 'container network ls' succeeds")
	}
	if !reflect.DeepEqual(ok.argv, []string{"container", "network", "ls"}) {
		t.Errorf("argv = %#v", ok.argv)
	}

	failing := &captureRunner{err: errors.New("unknown subcommand 'network'")}
	if NewContainer(failing).SupportsNetworks(context.Background()) {
		t.Error("expected false when 'container network ls' errors (macOS 15)")
	}
}

// captureRunner records the argv it was invoked with and, if set, writes
// canned data to the stdout writer it is given.
type captureRunner struct {
	argv       []string
	stdoutData string
	err        error
}

func (r *captureRunner) Run(ctx context.Context, argv []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r.argv = argv
	if r.stdoutData != "" {
		io.WriteString(stdout, r.stdoutData)
	}
	return r.err
}

func TestContainerRunArgv(t *testing.T) {
	b := NewContainer(nil)
	got := b.RunArgv(RunRequest{
		Image:       "nginx:alpine",
		Name:        "demo_web_1",
		Env:         []string{"APP_ENV=dev"},
		Labels:      []string{"com.bianpai.project=demo", "com.bianpai.service=web"},
		Ports:       []string{"8080:80"},
		Volumes:     []string{"demo_data:/data"},
		Networks:    []NetworkAttachment{{Name: "demo_default", Aliases: []string{"web"}}},
		Hostname:    "web",
		Workdir:     "/app",
		CPUs:        "0.5",
		Memory:      "256M",
		Detach:      true,
		Command:     []string{"nginx", "-g", "daemon off;"},
		Interactive: true,
		TTY:         true,
	})
	want := []string{
		"container", "run", "--detach", "--name", "demo_web_1",
		"--env", "APP_ENV=dev",
		"--label", "com.bianpai.project=demo", "--label", "com.bianpai.service=web",
		"--publish", "8080:80",
		"--volume", "demo_data:/data",
		"--network", "demo_default",
		"--workdir", "/app", "--memory", "256M", "--cpus", "0.5",
		"--interactive", "--tty",
		"nginx:alpine", "nginx", "-g", "daemon off;",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestContainerBuildArgv(t *testing.T) {
	b := NewContainer(nil)
	got := b.BuildArgv(BuildRequest{
		Context:    ".",
		Dockerfile: "Dockerfile.dev",
		Tag:        "demo_api",
		Args:       []string{"MODE=dev"},
		Target:     "runtime",
		Pull:       true,
		NoCache:    true,
		Labels:     []string{"com.bianpai.project=demo"},
	})
	want := []string{"container", "build", "--tag", "demo_api", "--file", "Dockerfile.dev", "--pull", "--no-cache", "--target", "runtime", "--build-arg", "MODE=dev", "--label", "com.bianpai.project=demo", "."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

// TestContainerCommandArgv pins the subcommands that diverge from WSLC.
func TestContainerCommandArgv(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		call func(b *Container, r Runner)
		want []string
	}{
		{
			name: "check",
			call: func(b *Container, r Runner) { b.Check(ctx, io.Discard, io.Discard) },
			want: []string{"container", "--version"},
		},
		{
			name: "pull",
			call: func(b *Container, r Runner) { b.Pull(ctx, "nginx:alpine", io.Discard, io.Discard) },
			want: []string{"container", "image", "pull", "nginx:alpine"},
		},
		{
			name: "remove",
			call: func(b *Container, r Runner) { b.Remove(ctx, "demo_web_1", io.Discard, io.Discard) },
			want: []string{"container", "delete", "demo_web_1"},
		},
		{
			name: "stop",
			call: func(b *Container, r Runner) { b.Stop(ctx, "demo_web_1", io.Discard, io.Discard) },
			want: []string{"container", "stop", "demo_web_1"},
		},
		{
			name: "create network",
			call: func(b *Container, r Runner) {
				b.CreateNetwork(ctx, "demo_default", []string{"com.bianpai.project=demo"}, io.Discard, io.Discard)
			},
			want: []string{"container", "network", "create", "--label", "com.bianpai.project=demo", "demo_default"},
		},
		{
			name: "remove network",
			call: func(b *Container, r Runner) { b.RemoveNetwork(ctx, "demo_default", io.Discard, io.Discard) },
			want: []string{"container", "network", "delete", "demo_default"},
		},
		{
			name: "create volume",
			call: func(b *Container, r Runner) {
				b.CreateVolume(ctx, "demo_data", []string{"com.bianpai.project=demo"}, io.Discard, io.Discard)
			},
			want: []string{"container", "volume", "create", "--label", "com.bianpai.project=demo", "demo_data"},
		},
		{
			name: "remove volume",
			call: func(b *Container, r Runner) { b.RemoveVolume(ctx, "demo_data", io.Discard, io.Discard) },
			want: []string{"container", "volume", "delete", "demo_data"},
		},
		{
			name: "exec",
			call: func(b *Container, r Runner) {
				b.Exec(ctx, "demo_web_1", []string{"sh", "-c", "echo hi"}, nil, io.Discard, io.Discard)
			},
			want: []string{"container", "exec", "demo_web_1", "sh", "-c", "echo hi"},
		},
		{
			name: "logs follow",
			call: func(b *Container, r Runner) { b.Logs(ctx, "demo_web_1", true, io.Discard, io.Discard) },
			want: []string{"container", "logs", "--follow", "demo_web_1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &captureRunner{}
			b := NewContainer(r)
			tc.call(b, r)
			if !reflect.DeepEqual(r.argv, tc.want) {
				t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", r.argv, tc.want)
			}
		})
	}
}

// containerListJSON is a real `container list --all --format json` payload shape
// (container v1.0.0) carrying two containers from different projects.
const containerListJSON = `[
  {"id":"demo_web_1","configuration":{"id":"demo_web_1","labels":{"com.bianpai.project":"demo","com.bianpai.service":"web"},"image":{"reference":"docker.io/library/nginx:alpine"}},"status":{"state":"running"}},
  {"id":"other_db_1","configuration":{"id":"other_db_1","labels":{"com.bianpai.project":"other","com.bianpai.service":"db"},"image":{"reference":"docker.io/library/postgres:16"}},"status":{"state":"stopped"}}
]`

func TestContainerListAcceptsStringStatus(t *testing.T) {
	r := &captureRunner{stdoutData: `[
  {"id":"demo_web_1","configuration":{"labels":{"com.bianpai.project":"demo"},"image":{"reference":"nginx:alpine"}},"status":"running"}
]`}

	var out bytes.Buffer
	if err := NewContainer(r).List(context.Background(), "demo", &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "demo_web_1") || !strings.Contains(out.String(), "running") {
		t.Fatalf("expected string status to render, got:\n%s", out.String())
	}
}

func TestContainerListRejectsInvalidStatus(t *testing.T) {
	r := &captureRunner{stdoutData: `[
  {"id":"demo_web_1","configuration":{"labels":{"com.bianpai.project":"demo"}},"status":true}
]`}

	var out bytes.Buffer
	err := NewContainer(r).List(context.Background(), "demo", &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "parsing container status") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}

// container list has no --filter, so List must filter by the project label itself.
func TestContainerListFiltersByProjectLabel(t *testing.T) {
	r := &captureRunner{stdoutData: containerListJSON}
	b := NewContainer(r)

	var out bytes.Buffer
	if err := b.List(context.Background(), "demo", &out, io.Discard); err != nil {
		t.Fatal(err)
	}

	// It must fetch the full JSON list (no --filter flag exists).
	wantArgv := []string{"container", "list", "--all", "--format", "json"}
	if !reflect.DeepEqual(r.argv, wantArgv) {
		t.Fatalf("argv = %#v, want %#v", r.argv, wantArgv)
	}

	got := out.String()
	if !strings.Contains(got, "demo_web_1") {
		t.Errorf("expected matching container demo_web_1 in output:\n%s", got)
	}
	if !strings.Contains(got, "docker.io/library/nginx:alpine") || !strings.Contains(got, "running") {
		t.Errorf("expected image and state in output:\n%s", got)
	}
	if strings.Contains(got, "other_db_1") {
		t.Errorf("non-matching project container leaked into output:\n%s", got)
	}
}
