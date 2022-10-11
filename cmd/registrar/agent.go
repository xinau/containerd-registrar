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
			Name:  "containerd-binary",
			Usage: "name of the containerd binary to be restarted",
			Value: "/usr/bin/containerd",
		},
		&cli.GenericFlag{
			Name:  "containerd-config-file",
			Usage: "path to containerd's configuration file",
			Value: flags.NewFile("/etc/containerd/config.toml"),
		},
		&cli.StringFlag{
			Name:  "containerd-cri-registry-path",
			Usage: "value being set as containerd cri registry path",
			Value: "/etc/containerd/certs.d",
		},
		&cli.GenericFlag{
			Name:     "containerd-cri-registry-files",
			Usage:    "files to copy to containerd cri registry path",
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
			BinaryName:     ctx.String("containerd-binary"),
			ConfigFile:     ctx.String("containerd-config-file"),
			RegistryPath:   ctx.String("containerd-cri-registry-path"),
			RegistryHosts:  ctx.StringSlice("containerd-cri-registry-files"),
			RestartTimeout: ctx.Duration("restart.timout"),
		})

		logrus.WithFields(logrus.Fields{"version": version.Version, "revision": version.Revision}).Info("running containerd-registrar agent")
		return mgr.Run(ctx.Context)
	},
}
