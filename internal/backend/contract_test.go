package backend

import (
	"context"
	"io"
	"testing"
)

// backendsUnderTest returns every backend wired to the same capturing runner,
// so the contract below is enforced identically across all of them. New
// backends (Docker, nerdctl, ...) must be added here and satisfy every case.
func backendsUnderTest(r Runner) map[string]Backend {
	return map[string]Backend{
		"wslc":      NewWSLC(r),
		"container": NewContainer(r),
	}
}

// canonicalRunRequest exercises the flags every backend must translate.
func canonicalRunRequest() RunRequest {
	return RunRequest{
		Image:   "nginx:alpine",
		Name:    "demo_web_1",
		Labels:  []string{"com.bianpai.project=demo", "com.bianpai.service=web"},
		Ports:   []string{"8080:80", "9090:90"},
		Volumes: []string{"demo_data:/data"},
		Env:     []string{"APP_ENV=dev"},
		Command: []string{"nginx", "-g", "daemon off;"},
		Detach:  true,
	}
}

// TestBackendRunContract locks invariants that hold for ALL backends, so a new
// backend can't silently drop the image, name, labels, ports, or command.
func TestBackendRunContract(t *testing.T) {
	req := canonicalRunRequest()
	for name := range backendsUnderTest(nil) {
		t.Run(name, func(t *testing.T) {
			r := &captureRunner{}
			b := backendsUnderTest(r)[name]
			if err := b.Run(context.Background(), req, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			argv := r.argv

			// The command must be the trailing tokens, immediately after the image.
			imgIdx := indexOf(argv, req.Image)
			if imgIdx < 0 {
				t.Fatalf("image %q missing from argv: %#v", req.Image, argv)
			}
			if countOf(argv, req.Image) != 1 {
				t.Errorf("image %q appears %d times, want 1", req.Image, countOf(argv, req.Image))
			}
			gotCmd := argv[imgIdx+1:]
			if !equalStrings(gotCmd, req.Command) {
				t.Errorf("command after image = %#v, want %#v", gotCmd, req.Command)
			}

			// --name must carry the container name.
			if v := flagValue(argv, "--name"); v != req.Name {
				t.Errorf("--name = %q, want %q", v, req.Name)
			}

			// Every label must be rendered exactly once.
			for _, label := range req.Labels {
				if countOf(argv, label) != 1 {
					t.Errorf("label %q rendered %d times, want 1 (argv: %#v)", label, countOf(argv, label), argv)
				}
			}

			// Port and volume counts must match the request.
			if got := countOf(argv, "8080:80") + countOf(argv, "9090:90"); got != len(req.Ports) {
				t.Errorf("published ports = %d, want %d", got, len(req.Ports))
			}
			if got := countOf(argv, "demo_data:/data"); got != len(req.Volumes) {
				t.Errorf("mounted volumes = %d, want %d", got, len(req.Volumes))
			}
		})
	}
}

func indexOf(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}

func countOf(ss []string, target string) int {
	n := 0
	for _, s := range ss {
		if s == target {
			n++
		}
	}
	return n
}

// flagValue returns the token following the first occurrence of flag.
func flagValue(ss []string, flag string) string {
	for i, s := range ss {
		if s == flag && i+1 < len(ss) {
			return ss[i+1]
		}
	}
	return ""
}

func equalStrings(a, b []string) bool {
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
