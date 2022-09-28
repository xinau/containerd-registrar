package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/xinau/containerd-registrar/internal/containerd"
)

type Config struct {
	BinaryName     string
	ConfigFile     string
	RegistryPath   string
	RegistryHosts  []string
	RestartTimeout time.Duration
}

type Manager struct {
	cfg Config
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

func (mgr *Manager) copyRegistryHosts() {
	logfields := logrus.Fields{"registry.hosts": mgr.cfg.RegistryHosts, "registry.path": mgr.cfg.RegistryPath}
	logrus.WithFields(logfields).Debug("copying registry hosts to path")
	if err := containerd.CopyRegistryHosts(mgr.cfg.RegistryPath, mgr.cfg.RegistryHosts); err != nil {
		logrus.WithFields(logfields).WithError(err).Fatal("copying registry hosts to path")
	}
	logrus.WithFields(logfields).Info("registry hosts copied to path")
}

func (mgr *Manager) updateRegistryPath() (bool, error) {
	cfg, err := containerd.LoadConfig(mgr.cfg.ConfigFile)
	if err != nil {
		return false, fmt.Errorf("loading config: %s", err)
	}

	changed, err := containerd.SetRegistryPath(cfg, mgr.cfg.RegistryPath)
	if err != nil {
		return false, fmt.Errorf("setting registry path in config: %s", err)
	}

	if !changed {
		return false, nil
	}

	if err := containerd.WriteConfig(cfg, mgr.cfg.ConfigFile); err != nil {
		return true, fmt.Errorf("writting config to file: %s", err)
	}

	return true, nil
}

func (mgr *Manager) updateRegistryPathAndRestart(ctx context.Context) {
	logfields := logrus.Fields{"config.file": mgr.cfg.ConfigFile, "registry.path": mgr.cfg.RegistryPath}
	logrus.WithFields(logfields).Debug("updating registry path in config")
	changed, err := mgr.updateRegistryPath()
	if err != nil {
		logrus.WithFields(logfields).WithError(err).Fatal("updating registry path in config")
	}

	if !changed {
		return
	}
	logrus.WithFields(logfields).Info("registry path updated in config")

	logfields = logrus.Fields{"binary.name": mgr.cfg.BinaryName}
	logrus.WithFields(logfields).Info("restarting containerd process")
	if err := containerd.RestartProcess(ctx, mgr.cfg.BinaryName, mgr.cfg.RestartTimeout); err != nil {
		logrus.WithFields(logfields).WithError(err).Fatal("restarting containerd process")
	}
	logrus.WithFields(logfields).Info("containerd process restarted")
}

func (mgr *Manager) Run(ctx context.Context) error {
	mgr.copyRegistryHosts()
	mgr.updateRegistryPathAndRestart(ctx)

	return ctx.Err()
}
