package backend

import (
	"reflect"
	"testing"
)

func TestWSLCRunArgv(t *testing.T) {
	b := NewWSLC(nil)
	got := b.RunArgv(RunRequest{
		Image:       "nginx:alpine",
		Name:        "demo_web_1",
		Env:         []string{"APP_ENV=dev"},
		Labels:      []string{"com.bianpai.project=demo", "com.bianpai.service=web"},
		Ports:       []string{"8080:80"},
		Volumes:     []string{"demo_data:/data"},
		Networks:    []NetworkAttachment{{Name: "demo_default", Aliases: []string{"web"}}},
		Workdir:     "/app",
		CPUs:        "0.5",
		Memory:      "256M",
		Detach:      true,
		Command:     []string{"nginx", "-g", "daemon off;"},
		Interactive: true,
		TTY:         true,
	})
	want := []string{
		"wslc", "run", "--detach", "--name", "demo_web_1",
		"--env", "APP_ENV=dev",
		"--label", "com.bianpai.project=demo", "--label", "com.bianpai.service=web",
		"--publish", "8080:80",
		"--volume", "demo_data:/data",
		"--network", "demo_default", "--network-alias", "web",
		"--workdir", "/app", "--memory", "256M", "--cpus", "0.5",
		"--interactive", "--tty",
		"nginx:alpine", "nginx", "-g", "daemon off;",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestWSLCBuildArgv(t *testing.T) {
	b := NewWSLC(nil)
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
	want := []string{"wslc", "build", "--tag", "demo_api", "--file", "Dockerfile.dev", "--pull", "--no-cache", "--target", "runtime", "--build-arg", "MODE=dev", "--label", "com.bianpai.project=demo", "."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}
