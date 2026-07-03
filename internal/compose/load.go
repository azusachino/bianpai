package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var projectNameRe = regexp.MustCompile(`[^a-z0-9_-]+`)

type Project struct {
	Name      string
	FilePath  string
	WorkDir   string
	Services  map[string]Service
	Networks  map[string]Network
	Volumes   map[string]Volume
	Warnings  []string
	RawConfig File
}

func DiscoverFile(workDir string) (string, error) {
	names := []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
	for _, name := range names {
		path := filepath.Join(workDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no compose file found in %s", workDir)
}

func Load(path, projectName string) (*Project, error) {
	if path == "" {
		var err error
		path, err = DiscoverFile(".")
		if err != nil {
			return nil, err
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if len(file.Services) == 0 {
		return nil, fmt.Errorf("compose file has no services")
	}

	workDir := filepath.Dir(abs)
	if projectName == "" {
		projectName = file.Name
	}
	if projectName == "" {
		projectName = filepath.Base(workDir)
	}
	projectName = NormalizeProjectName(projectName)
	if file.Networks == nil {
		file.Networks = map[string]Network{}
	}
	if file.Volumes == nil {
		file.Volumes = map[string]Volume{}
	}

	p := &Project{
		Name:      projectName,
		FilePath:  abs,
		WorkDir:   workDir,
		Services:  file.Services,
		Networks:  file.Networks,
		Volumes:   file.Volumes,
		RawConfig: file,
	}
	p.Warnings = unsupportedWarnings(file.Services)
	return p, nil
}

func NormalizeProjectName(name string) string {
	name = strings.ToLower(name)
	name = projectNameRe.ReplaceAllString(name, "")
	name = strings.Trim(name, "_-")
	if name == "" {
		return "bianpai"
	}
	return name
}

func (p *Project) ServiceNames(selected []string) ([]string, error) {
	if len(selected) == 0 {
		names := make([]string, 0, len(p.Services))
		for name := range p.Services {
			names = append(names, name)
		}
		sort.Strings(names)
		return topoSort(names, p.Services), nil
	}

	seen := map[string]bool{}
	var names []string
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		svc, ok := p.Services[name]
		if !ok {
			return fmt.Errorf("unknown service %q", name)
		}
		seen[name] = true
		for _, dep := range svc.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		names = append(names, name)
		return nil
	}
	for _, name := range selected {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return names, nil
}

func (p *Project) ContainerName(service string) string {
	return p.Name + "_" + service + "_1"
}

func (p *Project) ImageName(service string) string {
	return p.Name + "_" + service
}

func (p *Project) NetworkName(network string) string {
	return p.Name + "_" + network
}

func (p *Project) VolumeName(volume string) string {
	return p.Name + "_" + volume
}

func (p *Project) ServiceNetworks(service string) []ServiceNet {
	svc := p.Services[service]
	if len(svc.Networks) == 0 {
		return []ServiceNet{{Name: "default"}}
	}
	return svc.Networks
}

func (p *Project) UsedNetworks(serviceNames []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, service := range serviceNames {
		for _, network := range p.ServiceNetworks(service) {
			if !seen[network.Name] {
				seen[network.Name] = true
				out = append(out, network.Name)
			}
		}
	}
	return out
}

func (p *Project) UsedNamedVolumes(serviceNames []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, service := range serviceNames {
		for _, mount := range p.Services[service].Volumes {
			if _, ok := p.Volumes[mount.Source]; ok && !seen[mount.Source] {
				seen[mount.Source] = true
				out = append(out, mount.Source)
			}
		}
	}
	return out
}

func unsupportedWarnings(services map[string]Service) []string {
	seen := map[string]bool{}
	var warnings []string
	for name, service := range services {
		if service.Deploy.Kind != 0 && !seen["deploy"] {
			warnings = append(warnings, "service "+name+" uses deploy; swarm deploy semantics are ignored")
			seen["deploy"] = true
		}
		if len(service.Profiles) > 0 && !seen["profiles"] {
			warnings = append(warnings, "service "+name+" uses profiles; profiles are ignored")
			seen["profiles"] = true
		}
		if service.Healthcheck.Kind != 0 && !seen["healthcheck"] {
			warnings = append(warnings, "service "+name+" uses healthcheck; health state is not waited on")
			seen["healthcheck"] = true
		}
		if service.Restart != "" && !seen["restart"] {
			warnings = append(warnings, "service "+name+" uses restart; restart policies are not applied")
			seen["restart"] = true
		}
		if service.Privileged && !seen["privileged"] {
			warnings = append(warnings, "service "+name+" uses privileged; privileged mode is not applied")
			seen["privileged"] = true
		}
		if service.NetworkMode != "" && !seen["network_mode"] {
			warnings = append(warnings, "service "+name+" uses network_mode; it is ignored (each container gets its own network)")
			seen["network_mode"] = true
		}
		if service.ExtraHosts.Kind != 0 && !seen["extra_hosts"] {
			warnings = append(warnings, "service "+name+" uses extra_hosts; host entries are not applied")
			seen["extra_hosts"] = true
		}
	}
	return warnings
}

func topoSort(names []string, services map[string]Service) []string {
	seen := map[string]bool{}
	var out []string
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, dep := range services[name].DependsOn {
			if _, ok := services[dep]; ok {
				visit(dep)
			}
		}
		out = append(out, name)
	}
	for _, name := range names {
		visit(name)
	}
	return out
}

func StringifyScalar(v any) string {
	return scalarToString(v)
}
