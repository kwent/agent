package executors

import (
	"encoding/base64"
	"fmt"

	api "github.com/semaphoreci/agent/pkg/api"
	"github.com/semaphoreci/agent/pkg/config"
)

type DockerComposeFile struct {
	configuration   api.Compose
	exposeKvmDevice bool
	fileInjections  []config.FileInjection
}

func ConstructDockerComposeFile(conf api.Compose, exposeKvmDevice bool, fileInjections []config.FileInjection) string {
	f := DockerComposeFile{
		configuration:   conf,
		exposeKvmDevice: exposeKvmDevice,
		fileInjections:  fileInjections,
	}
	return f.Construct()
}

func (f *DockerComposeFile) Construct() string {
	// `version:` is obsolete in modern docker compose (v2.x) and emits a
	// noisy WARN line on every job. Drop it — schema validates without.
	dockerCompose := "services:\n"

	main, rest := f.configuration.Containers[0], f.configuration.Containers[1:]

	// main service links up all the services
	dockerCompose += f.ServiceWithLinks(main, rest)
	dockerCompose += "\n"

	for _, c := range rest {
		dockerCompose += f.Service(c)
		dockerCompose += "\n"
	}

	return dockerCompose
}

func (f *DockerComposeFile) Service(container api.Container) string {
	result := ""
	result += fmt.Sprintf("  %s:\n", container.Name)
	result += fmt.Sprintf("    image: %s\n", container.Image)

	if f.exposeKvmDevice {
		result += "    devices:\n"
		result += "      - \"/dev/kvm:/dev/kvm\"\n"
	}

	if container.Command != "" {
		result += fmt.Sprintf("    command: %s\n", container.Command)
	}

	if container.User != "" {
		result += fmt.Sprintf("    user: %s\n", container.User)
	}

	if container.Entrypoint != "" {
		result += fmt.Sprintf("    entrypoint: %s\n", container.Entrypoint)
	}

	if len(container.EnvVars) > 0 {
		result += "    environment:\n"

		for _, e := range container.EnvVars {
			value, _ := base64.StdEncoding.DecodeString(e.Value)

			result += fmt.Sprintf("      - %s=%s\n", e.Name, value)
		}
	}

	return result
}

// ServiceWithLinks builds the main service entry plus any side-cars.
//
// The legacy compose `links:` field is intentionally NOT emitted:
// Podman rejects `HostConfig.Links` with "bad parameter: link is not
// supported", and on Docker the sibling-service DNS that `links:`
// used to provide is already covered by Compose's default project
// network (every service gets a DNS-resolvable hostname for free).
// Dropping `links:` makes the generated YAML runtime-agnostic.
//
// Pin upstream DNS on every service so the container can resolve
// public hosts (github.com, gem fetch, etc.) without depending on
// the host's systemd-resolved stub at 127.0.0.53, which is
// unreachable from the container netns on Hetzner Robot images.
func (f *DockerComposeFile) ServiceWithLinks(c api.Container, links []api.Container) string {
	result := f.Service(c)

	result += "    dns:\n"
	result += "      - 1.1.1.1\n"
	result += "      - 8.8.8.8\n"
	result += "      - 9.9.9.9\n"

	if len(f.fileInjections) > 0 {
		result += "    volumes:\n"
		for _, fileInjection := range f.fileInjections {
			result += fmt.Sprintf("      - %s:%s\n", fileInjection.HostPath, fileInjection.Destination)
		}
	}

	return result
}
