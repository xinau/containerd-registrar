package main

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/xinau/containerd-registrar/internal/controller"
	"github.com/xinau/containerd-registrar/internal/flags"
	"github.com/xinau/containerd-registrar/internal/version"
)

var controllerCommand = &cli.Command{
	Name:  "controller",
	Usage: "control containerd registrar pods",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "agent.namespace",
			Usage: "containerd registrar agent namespace",
			Value: "kube-system",
		},
		&cli.GenericFlag{
			Name:  "agent.labels",
			Usage: "containerd registrar agent label selector",
			Value: flags.NewLabelSelector("app.kubernetes.io/name=containerd-registrar-agent"),
		},
		&cli.StringFlag{
			Name:  "agent.nodetaint",
			Usage: "containerd registrar agent node taint",
			Value: "node.containerd-registrar.io/agent-not-ready",
		},
		&cli.DurationFlag{
			Name:  "resync.interval",
			Usage: "kubernetes informer resync interval duration",
			Value: time.Minute,
		},
		&cli.GenericFlag{
			Name:  "kubeconfig",
			Usage: "kubernetes config filepath",
			Value: flags.NewFile(""),
		},
	},
	Action: func(ctx *cli.Context) error {
		logrus.SetLevel(ctx.Value("log.level").(logrus.Level))

		// if kubeconfig is empty, in-cluster config will be used
		file := ctx.Value("kubeconfig").(string)
		config, err := clientcmd.BuildConfigFromFlags("", file)
		if err != nil {
			logrus.WithField("kubeconfig", file).WithError(err).Fatal("building kubernets config")
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			logrus.WithError(err).Fatal("getting kubernets config")
		}

		mgr := controller.NewManager(clientset, controller.Config{
			AgentNamespace: ctx.String("agent.namespace"),
			AgentLabels:    ctx.Value("agent.labels").(string),
			AgentNodeTaint: ctx.String("agent.nodetaint"),
			ResyncInterval: ctx.Duration("resync.interval"),
		})

		logrus.WithFields(logrus.Fields{"version": version.Version, "revision": version.Revision}).Info("running containerd-registrar controller")
		return mgr.Run(ctx.Context)
	},
}
