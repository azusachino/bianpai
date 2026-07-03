package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aisk/bianpai/internal/backend"
	"github.com/aisk/bianpai/internal/compose"
	"github.com/spf13/cobra"
)

type app struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	be     backend.Backend

	file        string
	projectName string
	backendName string
}

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	cmd := a.rootCommand(ctx)
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(stderr, "bianpai:", err)
		return 1
	}
	return 0
}

func newBackend(name string) (backend.Backend, error) {
	switch name {
	case "", "wslc":
		return backend.NewWSLC(nil), nil
	case "container":
		return backend.NewContainer(nil), nil
	default:
		return nil, fmt.Errorf("unsupported backend %q", name)
	}
}

func (a *app) rootCommand(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Use:           "bianpai",
		Short:         "Run a Compose-compatible project on container CLIs",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			be, err := newBackend(a.backendName)
			if err != nil {
				return err
			}
			a.be = be
			return nil
		},
	}
	root.PersistentFlags().StringVarP(&a.file, "file", "f", "", "compose file path")
	root.PersistentFlags().StringVarP(&a.projectName, "project-name", "p", "", "project name")
	root.PersistentFlags().StringVar(&a.backendName, "backend", "wslc", "container backend")

	root.AddCommand(a.versionCommand())
	root.AddCommand(a.upCommand(ctx))
	root.AddCommand(a.downCommand(ctx))
	root.AddCommand(a.psCommand(ctx))
	root.AddCommand(a.logsCommand(ctx))
	root.AddCommand(a.buildCommand(ctx))
	root.AddCommand(a.pullCommand(ctx))
	root.AddCommand(a.execCommand(ctx))
	return root
}

func (a *app) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show backend version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.be.Check(cmd.Context(), a.stdout, a.stderr)
		},
	}
}

func (a *app) upCommand(ctx context.Context) *cobra.Command {
	var detach bool
	var withBuild bool
	cmd := &cobra.Command{
		Use:   "up [SERVICE...]",
		Short: "Create and start project containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			if !detach {
				fmt.Fprintln(a.stderr, "warning: foreground attach is not implemented; running detached")
			}
			services, err := project.ServiceNames(args)
			if err != nil {
				return err
			}
			if c, ok := a.be.(*backend.Container); ok && len(services) > 1 && !c.SupportsNetworks(cmd.Context()) {
				fmt.Fprintln(a.stderr, "warning: Apple container needs macOS 26 for container-to-container networking; services may not reach each other")
			}
			if withBuild {
				if err := a.buildServices(cmd.Context(), project, services, false, false); err != nil {
					return err
				}
			}
			labels := projectLabels(project.Name)
			for _, network := range project.UsedNetworks(services) {
				if err := a.be.CreateNetwork(cmd.Context(), project.NetworkName(network), labels, a.stdout, a.stderr); err != nil {
					return err
				}
			}
			for _, volume := range project.UsedNamedVolumes(services) {
				if err := a.be.CreateVolume(cmd.Context(), project.VolumeName(volume), labels, a.stdout, a.stderr); err != nil {
					return err
				}
			}
			for _, service := range services {
				req, err := runRequest(project, service)
				if err != nil {
					return err
				}
				req.Detach = true
				_ = a.be.Stop(cmd.Context(), req.Name, io.Discard, io.Discard)
				_ = a.be.Remove(cmd.Context(), req.Name, io.Discard, io.Discard)
				if err := a.be.Run(cmd.Context(), req, a.stdout, a.stderr); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "run containers in the background")
	cmd.Flags().BoolVar(&withBuild, "build", false, "build images before starting")
	return cmd
}

func (a *app) downCommand(ctx context.Context) *cobra.Command {
	var removeVolumes bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove project containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			services, err := project.ServiceNames(nil)
			if err != nil {
				return err
			}
			for i := len(services) - 1; i >= 0; i-- {
				name := project.ContainerName(services[i])
				_ = a.be.Stop(cmd.Context(), name, a.stdout, a.stderr)
				_ = a.be.Remove(cmd.Context(), name, a.stdout, a.stderr)
			}
			for _, network := range project.UsedNetworks(services) {
				_ = a.be.RemoveNetwork(cmd.Context(), project.NetworkName(network), a.stdout, a.stderr)
			}
			if removeVolumes {
				for _, volume := range project.UsedNamedVolumes(services) {
					_ = a.be.RemoveVolume(cmd.Context(), project.VolumeName(volume), a.stdout, a.stderr)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&removeVolumes, "volumes", "v", false, "remove named volumes")
	return cmd
}

func (a *app) psCommand(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List project containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			return a.be.List(cmd.Context(), project.Name, a.stdout, a.stderr)
		},
	}
}

func (a *app) logsCommand(ctx context.Context) *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [SERVICE...]",
		Short: "Show service logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			services, err := project.ServiceNames(args)
			if err != nil {
				return err
			}
			for _, service := range services {
				if err := a.be.Logs(cmd.Context(), project.ContainerName(service), follow, a.stdout, a.stderr); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow logs")
	return cmd
}

func (a *app) buildCommand(ctx context.Context) *cobra.Command {
	var noCache bool
	var pull bool
	cmd := &cobra.Command{
		Use:   "build [SERVICE...]",
		Short: "Build service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			services, err := project.ServiceNames(args)
			if err != nil {
				return err
			}
			return a.buildServices(cmd.Context(), project, services, noCache, pull)
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "build without cache")
	cmd.Flags().BoolVar(&pull, "pull", false, "always pull newer base images")
	return cmd
}

func (a *app) pullCommand(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "pull [SERVICE...]",
		Short: "Pull service images",
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			services, err := project.ServiceNames(args)
			if err != nil {
				return err
			}
			for _, service := range services {
				image := project.Services[service].Image
				if image == "" {
					continue
				}
				if err := a.be.Pull(cmd.Context(), image, a.stdout, a.stderr); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func (a *app) execCommand(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "exec SERVICE COMMAND...",
		Short: "Run a command in a service container",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("exec requires SERVICE and COMMAND")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := a.loadProject()
			if err != nil {
				return err
			}
			service := args[0]
			if _, ok := project.Services[service]; !ok {
				return fmt.Errorf("unknown service %q", service)
			}
			return a.be.Exec(cmd.Context(), project.ContainerName(service), args[1:], a.stdin, a.stdout, a.stderr)
		},
	}
}

func (a *app) loadProject() (*compose.Project, error) {
	project, err := compose.Load(a.file, a.projectName)
	if err != nil {
		return nil, err
	}
	for _, warning := range project.Warnings {
		fmt.Fprintln(a.stderr, "warning:", warning)
	}
	return project, nil
}

func (a *app) buildServices(ctx context.Context, project *compose.Project, services []string, noCache, pull bool) error {
	for _, service := range services {
		svc := project.Services[service]
		if svc.Build == nil {
			continue
		}
		contextPath := resolvePath(project.WorkDir, svc.Build.Context)
		dockerfile := svc.Build.Dockerfile
		if dockerfile != "" && !filepath.IsAbs(dockerfile) {
			dockerfile = filepath.Join(contextPath, dockerfile)
		}
		image := svc.Image
		if image == "" {
			image = project.ImageName(service)
		}
		req := backend.BuildRequest{
			Context:    contextPath,
			Dockerfile: dockerfile,
			Tag:        image,
			Args:       svc.Build.Args,
			Target:     svc.Build.Target,
			Pull:       pull,
			NoCache:    noCache,
			Labels:     serviceLabels(project.Name, service),
		}
		if err := a.be.Build(ctx, req, a.stdout, a.stderr); err != nil {
			return err
		}
	}
	return nil
}

func runRequest(project *compose.Project, service string) (backend.RunRequest, error) {
	svc := project.Services[service]
	image := svc.Image
	if image == "" {
		if svc.Build == nil {
			return backend.RunRequest{}, fmt.Errorf("service %q must define image or build", service)
		}
		image = project.ImageName(service)
	}

	req := backend.RunRequest{
		Image:       image,
		Name:        project.ContainerName(service),
		Command:     svc.Command.Args(),
		Entrypoint:  svc.Entrypoint.Args(),
		Env:         append([]string(nil), svc.Environment...),
		EnvFiles:    resolvePaths(project.WorkDir, svc.EnvFile),
		Labels:      append(serviceLabels(project.Name, service), svc.Labels...),
		Ports:       portValues(svc.Ports),
		Hostname:    svc.Hostname,
		Workdir:     svc.WorkingDir,
		User:        svc.User,
		Memory:      svc.MemLimit,
		CPUs:        svc.CPUs.String(),
		Interactive: svc.StdinOpen,
		TTY:         svc.TTY,
	}
	for _, mount := range svc.Volumes {
		req.Volumes = append(req.Volumes, volumeValue(project, mount))
	}
	for _, network := range project.ServiceNetworks(service) {
		req.Networks = append(req.Networks, backend.NetworkAttachment{
			Name:    project.NetworkName(network.Name),
			Aliases: append([]string{service}, network.Aliases...),
		})
	}
	return req, nil
}

func projectLabels(project string) []string {
	return []string{"com.bianpai.project=" + project}
}

func serviceLabels(project, service string) []string {
	return []string{"com.bianpai.project=" + project, "com.bianpai.service=" + service}
}

func portValues(ports []compose.Port) []string {
	out := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.Value != "" {
			out = append(out, port.Value)
		}
	}
	return out
}

func volumeValue(project *compose.Project, mount compose.Mount) string {
	if _, ok := project.Volumes[mount.Source]; ok {
		return strings.Replace(mount.Value, mount.Source, project.VolumeName(mount.Source), 1)
	}
	if mount.Source == "" || filepath.IsAbs(mount.Source) || strings.HasPrefix(mount.Source, "~") {
		return mount.Value
	}
	if strings.HasPrefix(mount.Source, ".") {
		abs := filepath.Join(project.WorkDir, mount.Source)
		return strings.Replace(mount.Value, mount.Source, abs, 1)
	}
	return mount.Value
}

func resolvePaths(base string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, resolvePath(base, path))
	}
	return out
}

func resolvePath(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}
