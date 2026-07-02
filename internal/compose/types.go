package compose

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Name     string             `yaml:"name"`
	Services map[string]Service `yaml:"services"`
	Networks map[string]Network `yaml:"networks"`
	Volumes  map[string]Volume  `yaml:"volumes"`
}

type Service struct {
	Image       string       `yaml:"image"`
	Build       *Build       `yaml:"build"`
	Command     Command      `yaml:"command"`
	Entrypoint  Command      `yaml:"entrypoint"`
	Environment Environment  `yaml:"environment"`
	EnvFile     StringList   `yaml:"env_file"`
	Ports       []Port       `yaml:"ports"`
	Volumes     []Mount      `yaml:"volumes"`
	WorkingDir  string       `yaml:"working_dir"`
	User        string       `yaml:"user"`
	Hostname    string       `yaml:"hostname"`
	DependsOn   DependsOn    `yaml:"depends_on"`
	Networks    ServiceNets  `yaml:"networks"`
	Labels      Labels       `yaml:"labels"`
	StdinOpen   bool         `yaml:"stdin_open"`
	TTY         bool         `yaml:"tty"`
	MemLimit    string       `yaml:"mem_limit"`
	CPUs        ScalarString `yaml:"cpus"`
	Deploy      yaml.Node    `yaml:"deploy"`
	Profiles    []string     `yaml:"profiles"`
	Healthcheck yaml.Node    `yaml:"healthcheck"`
}

type Network struct {
	Driver string `yaml:"driver"`
}

type Volume struct {
	Driver string `yaml:"driver"`
}

type Build struct {
	Context    string
	Dockerfile string
	Args       []string
	Target     string
}

func (b *Build) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		b.Context = value.Value
		return nil
	case yaml.MappingNode:
		type rawBuild struct {
			Context    string    `yaml:"context"`
			Dockerfile string    `yaml:"dockerfile"`
			Args       BuildArgs `yaml:"args"`
			Target     string    `yaml:"target"`
		}
		var raw rawBuild
		if err := value.Decode(&raw); err != nil {
			return err
		}
		b.Context = raw.Context
		b.Dockerfile = raw.Dockerfile
		b.Args = raw.Args
		b.Target = raw.Target
		return nil
	default:
		return fmt.Errorf("build must be a string or mapping")
	}
}

type BuildArgs []string

func (a *BuildArgs) UnmarshalYAML(value *yaml.Node) error {
	var out []string
	switch value.Kind {
	case 0:
		return nil
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			out = append(out, value.Content[i].Value+"="+value.Content[i+1].Value)
		}
	case yaml.SequenceNode:
		for _, item := range value.Content {
			out = append(out, item.Value)
		}
	default:
		return fmt.Errorf("build args must be a map or list")
	}
	*a = out
	return nil
}

type Command []string

func (c *Command) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		return nil
	case yaml.ScalarNode:
		*c = strings.Fields(value.Value)
		return nil
	case yaml.SequenceNode:
		var out []string
		for _, item := range value.Content {
			out = append(out, item.Value)
		}
		*c = out
		return nil
	default:
		return fmt.Errorf("command must be a string or list")
	}
}

func (c Command) Args() []string {
	return append([]string(nil), c...)
}

type Environment []string

func (e *Environment) UnmarshalYAML(value *yaml.Node) error {
	var out []string
	switch value.Kind {
	case 0:
		return nil
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			out = append(out, value.Content[i].Value+"="+value.Content[i+1].Value)
		}
	case yaml.SequenceNode:
		for _, item := range value.Content {
			out = append(out, item.Value)
		}
	default:
		return fmt.Errorf("environment must be a map or list")
	}
	*e = out
	return nil
}

type Labels []string

func (l *Labels) UnmarshalYAML(value *yaml.Node) error {
	var out []string
	switch value.Kind {
	case 0:
		return nil
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			out = append(out, value.Content[i].Value+"="+value.Content[i+1].Value)
		}
	case yaml.SequenceNode:
		for _, item := range value.Content {
			out = append(out, item.Value)
		}
	default:
		return fmt.Errorf("labels must be a map or list")
	}
	*l = out
	return nil
}

type StringList []string

func (l *StringList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		return nil
	case yaml.ScalarNode:
		*l = []string{value.Value}
	case yaml.SequenceNode:
		for _, item := range value.Content {
			*l = append(*l, item.Value)
		}
	default:
		return fmt.Errorf("value must be a string or list")
	}
	return nil
}

type DependsOn []string

func (d *DependsOn) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		return nil
	case yaml.SequenceNode:
		for _, item := range value.Content {
			*d = append(*d, item.Value)
		}
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			*d = append(*d, value.Content[i].Value)
		}
	default:
		return fmt.Errorf("depends_on must be a list or map")
	}
	return nil
}

type ServiceNets []ServiceNet

type ServiceNet struct {
	Name    string
	Aliases []string
}

func (n *ServiceNets) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		return nil
	case yaml.SequenceNode:
		for _, item := range value.Content {
			*n = append(*n, ServiceNet{Name: item.Value})
		}
	case yaml.MappingNode:
		for i := 0; i < len(value.Content); i += 2 {
			net := ServiceNet{Name: value.Content[i].Value}
			var raw struct {
				Aliases []string `yaml:"aliases"`
			}
			if err := value.Content[i+1].Decode(&raw); err == nil {
				net.Aliases = raw.Aliases
			}
			*n = append(*n, net)
		}
	default:
		return fmt.Errorf("networks must be a list or map")
	}
	return nil
}

type Port struct {
	Value string
}

func (p *Port) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		p.Value = value.Value
	case yaml.MappingNode:
		var raw struct {
			Target    ScalarString `yaml:"target"`
			Published ScalarString `yaml:"published"`
			HostIP    string       `yaml:"host_ip"`
			Protocol  string       `yaml:"protocol"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		p.Value = formatPort(raw.Target.String(), raw.Published.String(), raw.HostIP, raw.Protocol)
	default:
		return fmt.Errorf("port must be a string or mapping")
	}
	return nil
}

type Mount struct {
	Value  string
	Source string
	Target string
}

func (m *Mount) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		m.Value = value.Value
		parts := strings.Split(value.Value, ":")
		if len(parts) >= 2 {
			m.Source = parts[0]
			m.Target = parts[1]
		}
	case yaml.MappingNode:
		var raw struct {
			Source   string `yaml:"source"`
			Target   string `yaml:"target"`
			ReadOnly bool   `yaml:"read_only"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		m.Source = raw.Source
		m.Target = raw.Target
		m.Value = raw.Source + ":" + raw.Target
		if raw.ReadOnly {
			m.Value += ":ro"
		}
	default:
		return fmt.Errorf("volume must be a string or mapping")
	}
	return nil
}

type ScalarString string

func (s *ScalarString) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case 0:
		return nil
	case yaml.ScalarNode:
		*s = ScalarString(value.Value)
	default:
		return fmt.Errorf("value must be a scalar")
	}
	return nil
}

func (s ScalarString) String() string {
	return string(s)
}

func formatPort(target, published, hostIP, protocol string) string {
	var out string
	if hostIP != "" {
		out += hostIP + ":"
	}
	if published != "" {
		out += published + ":"
	}
	out += target
	if protocol != "" && protocol != "tcp" {
		out += "/" + protocol
	}
	return out
}

func scalarToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}
