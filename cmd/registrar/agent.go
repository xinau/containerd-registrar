package main

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/xinau/containerd-registrar/internal/agent"
	"github.com/xinau/containerd-registrar/internal/flags"
	"github.com/xinau/containerd-registrar/internal/version"
)

var agentCommand = &cli.Command{
	Name:  "agent",
	Usage: "run agent configuring containerd's registries",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "binary.name",
			Usage: "containerd binary name",
			Value: "/usr/bin/containerd",
		},
		&cli.GenericFlag{
			Name:  "config.file",
			Usage: "containerd config filepath",
			Value: flags.NewFile("/etc/containerd/config.toml"),
		},
		&cli.StringFlag{
			Name:  "registry.path",
			Usage: "containerd cri registry config directory",
			Value: "/etc/containerd/certs.d",
		},
		&cli.GenericFlag{
			Name:     "registry.hosts",
			Usage:    "containerd cri registry hosts files",
			Value:    flags.NewFileSlice(),
			Required: true,
		},
		&cli.DurationFlag{
			Name:  "restart.timeout",
			Usage: "containerd restart timeout",
			Value: 30 * time.Second,
		},
	},
	Action: func(ctx *cli.Context) error {
		logrus.SetLevel(ctx.Value("log.level").(logrus.Level))

		mgr := agent.NewManager(agent.Config{
			BinaryName:     ctx.String("binary.name"),
			ConfigFile:     ctx.String("config.file"),
			RegistryPath:   ctx.Value("registry.path").(string),
			RegistryHosts:  ctx.Value("registry.hosts").([]string),
			RestartTimeout: ctx.Duration("restart.timout"),
		})

		logrus.WithFields(logrus.Fields{"version": version.Version, "revision": version.Revision}).Info("running containerd-registrar agent")
		return mgr.Run(ctx.Context)
	},
}
