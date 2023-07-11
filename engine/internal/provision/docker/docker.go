/*
2020 © Postgres.ai
*/

// Package docker provides an interface to work with Docker containers.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/host"

	"gitlab.com/postgres-ai/database-lab/v3/internal/provision/resources"
	"gitlab.com/postgres-ai/database-lab/v3/internal/provision/runners"
	"gitlab.com/postgres-ai/database-lab/v3/internal/retrieval/engine/postgres/tools"
	"gitlab.com/postgres-ai/database-lab/v3/pkg/log"
)

const (
	// LabelClone specifies the container label used to identify clone containers.
	LabelClone = "dblab_clone"

	// referenceKey uses as a filtering key to identify image tag.
	referenceKey = "reference"
)

var systemVolumes = []string{"/sys", "/lib", "/proc"}

// imagePullProgress describes the progress of pulling the container image.
type imagePullProgress struct {
	Status   string `json:"status"`
	Progress string `json:"progress"`
}

// RunContainer runs specified container.
func RunContainer(r runners.Runner, c *resources.AppConfig) error {
	hostInfo, err := host.Info()
	if err != nil {
		return errors.Wrap(err, "failed to get host info")
	}

	unixSocketCloneDir, volumes := createDefaultVolumes(c)

	log.Dbg(fmt.Sprintf("Host info: %#v", hostInfo))

	if hostInfo.VirtualizationRole == "guest" {
		// Build custom mounts rely on mounts of the Database Lab instance if it's running inside Docker container.
		// We cannot use --volumes-from because it removes the ZFS mount point.
		volumes, err = getMountVolumes(r, c, hostInfo.Hostname)
		if err != nil {
			return errors.Wrap(err, "failed to detect container volumes")
		}
	}

	if err := createSocketCloneDir(unixSocketCloneDir); err != nil {
		return errors.Wrap(err, "failed to create socket clone directory")
	}

	containerFlags := make([]string, 0, len(c.ContainerConf))
	for flagName, flagValue := range c.ContainerConf {
		containerFlags = append(containerFlags, fmt.Sprintf("--%s=%s", flagName, flagValue))
	}

	// TODO (akartasov): use Docker client instead of command execution.
	instancePort := strconv.Itoa(int(c.Port))
	dockerRunCmd := strings.Join([]string{
		"docker run",
		"--name", c.CloneName,
		"--detach",
		"--publish", fmt.Sprintf("%[1]s:%[1]s", instancePort),
		"--env", "PGDATA=" + c.DataDir(),
		"--env", "PG_UNIX_SOCKET_DIR=" + unixSocketCloneDir,
		"--env", "PG_SERVER_PORT=" + instancePort,
		strings.Join(volumes, " "),
		fmt.Sprintf("--label %s='%s'", LabelClone, c.Pool.Name),
		strings.Join(containerFlags, " "),
		c.DockerImage,
	}, " ")

	if _, err := r.Run(dockerRunCmd, true); err != nil {
		return errors.Wrap(err, "failed to run command")
	}

	dockerConnectCmd := strings.Join([]string{"docker network connect", c.NetworkID, c.CloneName}, " ")

	if _, err := r.Run(dockerConnectCmd, true); err != nil {
		return errors.Wrap(err, "failed to connect container to the internal DLE network")
	}

	return nil
}

func createDefaultVolumes(c *resources.AppConfig) (string, []string) {
	unixSocketCloneDir := c.Pool.SocketCloneDir(c.CloneName)

	// Directly mount PGDATA if Database Lab is running without any virtualization.
	volumes := []string{
		fmt.Sprintf("--volume %s:%s", c.CloneDir(), c.CloneDir()),
		fmt.Sprintf("--volume %s:%s", unixSocketCloneDir, unixSocketCloneDir),
	}

	return unixSocketCloneDir, volumes
}

func getMountVolumes(r runners.Runner, c *resources.AppConfig, containerID string) ([]string, error) {
	inspectCmd := "docker inspect -f '{{ json .Mounts }}' " + containerID

	var mountPoints []types.MountPoint

	out, err := r.Run(inspectCmd, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container mounts")
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &mountPoints); err != nil {
		return nil, errors.Wrap(err, "failed to interpret mount paths")
	}

	return buildVolumesFromMountPoints(c, mountPoints), nil
}

func buildVolumesFromMountPoints(c *resources.AppConfig, mountPoints []types.MountPoint) []string {
	unixSocketCloneDir := c.Pool.SocketCloneDir(c.CloneName)
	mounts := tools.GetMountsFromMountPoints(c.CloneDir(), mountPoints)
	volumes := make([]string, 0, len(mounts))

	for _, mountPoint := range mountPoints {
		// Add an extra mount for socket directories.
		if strings.HasPrefix(unixSocketCloneDir, mountPoint.Destination) {
			volumes = append(volumes, buildSocketMount(unixSocketCloneDir, mountPoint.Source, mountPoint.Destination))
			break
		}
	}

	for _, mount := range mounts {
		// Exclude system and non-data volumes from a clone container.
		if isSystemVolume(mount.Source) || !strings.HasPrefix(mount.Source, c.Pool.MountDir) {
			continue
		}

		volume := fmt.Sprintf("--volume %s:%s", mount.Source, mount.Target)

		if mount.BindOptions != nil && mount.BindOptions.Propagation != "" {
			volume += ":" + string(mount.BindOptions.Propagation)
		}

		volumes = append(volumes, volume)
	}

	return volumes
}

func isSystemVolume(source string) bool {
	for _, sysVolume := range systemVolumes {
		if strings.HasPrefix(source, sysVolume) {
			return true
		}
	}

	return false
}

// buildSocketMount builds a socket directory mounting rely on dataDir mounting.
func buildSocketMount(socketDir, hostDataDir, destinationDir string) string {
	socketPath := strings.TrimPrefix(socketDir, destinationDir)
	hostSocketDir := path.Join(hostDataDir, socketPath)

	return fmt.Sprintf("--volume %s:%s:rshared", hostSocketDir, socketDir)
}

func createSocketCloneDir(socketCloneDir string) error {
	if err := os.RemoveAll(socketCloneDir); err != nil {
		return err
	}

	if err := os.MkdirAll(socketCloneDir, 0777); err != nil {
		return err
	}

	return os.Chmod(socketCloneDir, 0777)
}

// StopContainer stops specified container.
func StopContainer(r runners.Runner, c *resources.AppConfig) (string, error) {
	dockerStopCmd := "docker container stop " + c.CloneName

	return r.Run(dockerStopCmd, false)
}

// RemoveContainer removes specified container.
func RemoveContainer(r runners.Runner, cloneName string) (string, error) {
	dockerRemoveCmd := "docker container rm --force --volumes " + cloneName

	return r.Run(dockerRemoveCmd, false)
}

// ListContainers lists container names.
func ListContainers(r runners.Runner, clonePool string) ([]string, error) {
	dockerListCmd := fmt.Sprintf(`docker container ls --filter "label=%s" --filter "label=%s" --all --format '{{.Names}}'`,
		LabelClone, clonePool)

	out, err := r.Run(dockerListCmd, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list containers")
	}

	out = strings.TrimSpace(out)
	if len(out) == 0 {
		return []string{}, nil
	}

	return strings.Split(out, "\n"), nil
}

// GetLogs gets logs from specified container.
func GetLogs(r runners.Runner, c *resources.AppConfig, sinceRelMins uint) (string, error) {
	dockerLogsCmd := "docker logs " + c.CloneName + " " +
		"--since " + strconv.FormatUint(uint64(sinceRelMins), 10) + "m " +
		"--timestamps"

	return r.Run(dockerLogsCmd, true)
}

// Exec executes command on specified container.
func Exec(r runners.Runner, c *resources.AppConfig, cmd string) (string, error) {
	dockerExecCmd := "docker exec " + c.CloneName + " " + cmd

	return r.Run(dockerExecCmd, true)
}

// PrepareImage prepares a Docker image to use.
func PrepareImage(ctx context.Context, docker *client.Client, dockerImage string) error {
	imageExists, err := ImageExists(ctx, docker, dockerImage)
	if err != nil {
		return fmt.Errorf("cannot check docker image existence: %w", err)
	}

	if imageExists {
		return nil
	}

	if err := PullImage(ctx, docker, dockerImage); err != nil {
		return fmt.Errorf("cannot pull docker image: %w", err)
	}

	return nil
}

// ImageExists checks existence of Docker image.
func ImageExists(ctx context.Context, docker *client.Client, dockerImage string) (bool, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add(referenceKey, dockerImage)

	list, err := docker.ImageList(ctx, types.ImageListOptions{
		All:     false,
		Filters: filterArgs,
	})

	if err != nil {
		return false, fmt.Errorf("failed to list images: %w", err)
	}

	return len(list) > 0, nil
}

// PullImage pulls Docker image from DockerHub registry.
func PullImage(ctx context.Context, docker *client.Client, dockerImage string) error {
	pullResponse, err := docker.ImagePull(ctx, dockerImage, types.ImagePullOptions{})

	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// reading output of image pulling, without reading pull will not be performed
	decoder := json.NewDecoder(pullResponse)

	for {
		var pullResult imagePullProgress
		if err := decoder.Decode(&pullResult); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return fmt.Errorf("failed to pull image: %w", err)
		}

		log.Dbg("Image pulling progress", pullResult.Status, pullResult.Progress)
	}

	err = pullResponse.Close()

	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	return nil
}

// IsContainerRunning checks if specified container is running.
func IsContainerRunning(ctx context.Context, docker *client.Client, containerName string) (bool, error) {
	inspection, err := docker.ContainerInspect(ctx, containerName)
	if err != nil {
		return false, fmt.Errorf("failed to inpect container: %w", err)
	}

	return inspection.State.Running, nil
}
