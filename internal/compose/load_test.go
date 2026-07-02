package compose

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadComposeSubset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	data := []byte(`name: Demo App
services:
  db:
    image: postgres:16
  web:
    build:
      context: ./web
      dockerfile: Dockerfile.dev
      args:
        MODE: dev
      target: runtime
    command: ["serve", "--debug"]
    environment:
      APP_ENV: dev
      DEBUG: "1"
    env_file: .env
    ports:
      - "8080:80"
      - target: 443
        published: 8443
        protocol: tcp
    volumes:
      - data:/var/lib/app
      - ./src:/app/src:ro
    depends_on:
      db:
        condition: service_started
    networks:
      appnet:
        aliases: [frontend]
    labels:
      com.example.role: web
    stdin_open: true
    tty: true
    mem_limit: 512M
    cpus: "0.5"
volumes:
  data: {}
networks:
  appnet: {}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "demoapp" {
		t.Fatalf("project name = %q", project.Name)
	}
	services, err := project.ServiceNames([]string{"web"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(services, []string{"db", "web"}) {
		t.Fatalf("service order = %#v", services)
	}
	web := project.Services["web"]
	if web.Build.Context != "./web" || web.Build.Dockerfile != "Dockerfile.dev" || web.Build.Target != "runtime" {
		t.Fatalf("build parse mismatch: %#v", web.Build)
	}
	if !reflect.DeepEqual(web.Build.Args, []string{"MODE=dev"}) {
		t.Fatalf("build args = %#v", web.Build.Args)
	}
	if !reflect.DeepEqual([]string(web.Environment), []string{"APP_ENV=dev", "DEBUG=1"}) {
		t.Fatalf("environment = %#v", web.Environment)
	}
	if got := []string{web.Ports[0].Value, web.Ports[1].Value}; !reflect.DeepEqual(got, []string{"8080:80", "8443:443"}) {
		t.Fatalf("ports = %#v", got)
	}
	if got := project.UsedNamedVolumes([]string{"web"}); !reflect.DeepEqual(got, []string{"data"}) {
		t.Fatalf("named volumes = %#v", got)
	}
	if got := project.UsedNetworks([]string{"web"}); !reflect.DeepEqual(got, []string{"appnet"}) {
		t.Fatalf("networks = %#v", got)
	}
}
