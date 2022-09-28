package containerd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml"
)

const (
	criPluginURI = "io.containerd.grpc.v1.cri"
)

func LoadConfig(path string) (*toml.Tree, error) {
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, err
	}

	imports := tree.GetArray("imports")
	if imports != nil && len(imports.([]string)) > 0 {
		return nil, errors.New("containerd config imports aren't supported yet")
	}

	return tree, nil
}

func SetRegistryPath(cfg *toml.Tree, path string) (bool, error) {
	val := cfg.GetPath([]string{"plugins", criPluginURI, "registry", "config_path"})
	if val != nil && val.(string) == path {
		return false, nil
	}

	registry, _ := toml.TreeFromMap(map[string]interface{}{
		"config_path": path,
	})
	cfg.SetPath([]string{"plugins", criPluginURI, "registry"}, registry)

	return true, nil
}

func WriteConfig(cfg *toml.Tree, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = cfg.WriteTo(file)
	return err
}

func getPidOf(ctx context.Context, name string) (int, error) {
	b, err := exec.CommandContext(ctx, "pidof", "-s", name).CombinedOutput()
	out := strings.TrimSpace(string(b))
	if err != nil {
		if len(out) != 0 {
			return 0, fmt.Errorf("getting pid of %q: error: %v, msg: %q", name, err, out)
		}
		return 0, nil
	}
	return strconv.Atoi(out)
}

func killPid(ctx context.Context, pid int) error {
	out, err := exec.CommandContext(ctx, "kill", strconv.Itoa(pid)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("killing process %d: error: %v, msg: %q", pid, err, out)
	}
	return nil
}

func waitFor(ctx context.Context, interval, timeout time.Duration, check func(ctx context.Context) (bool, error)) error {
	intervalTimer, timeoutTimer := time.NewTimer(interval), time.NewTimer(timeout)
	defer intervalTimer.Stop()
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTimer.C:
			return errors.New("timeout exceeded")
		case <-intervalTimer.C:
			done, err := check(ctx)
			if err != nil {
				return err
			}

			if done {
				return nil
			}
		}
	}
}

func RestartProcess(ctx context.Context, binary string, timeout time.Duration) error {
	pid, err := getPidOf(ctx, binary)
	if err != nil {
		return err
	}

	if err := killPid(ctx, pid); err != nil {
		return err
	}

	return waitFor(ctx, time.Second, timeout, func(ctx context.Context) (bool, error) {
		newPid, err := getPidOf(ctx, binary)
		if err != nil {
			return false, err
		}

		if newPid != 0 && newPid != pid {
			return true, nil
		}

		return false, nil
	})
}

func copy(src, dst string) (int64, error) {
	s, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer s.Close()

	d, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer d.Close()

	return io.Copy(d, s)
}

func CopyRegistryHosts(path string, files []string) error {
	err := os.Mkdir(path, 0750)
	if err != nil && !os.IsExist(err) {
		return err
	}

	for _, file := range files {
		name := filepath.Base(file)
		if _, err := copy(file, filepath.Join(path, name)); err != nil {
			return err
		}
	}

	return nil
}
