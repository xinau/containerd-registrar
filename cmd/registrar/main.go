package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/xinau/containerd-registrar/internal/flags"
	"github.com/xinau/containerd-registrar/internal/version"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, version.Package, c.App.Version, version.Revision)
	}
}

func setLogLevel(ctx *cli.Context) error {
	lvl, err := logrus.ParseLevel(ctx.String("log.level"))
	if err != nil {
		return err
	}

	logrus.SetLevel(lvl)
	return nil
}

func App() *cli.App {
	app := cli.NewApp()
	app.Name = "containerd-registrar"
	app.Version = version.Version
	app.Flags = []cli.Flag{
		&cli.GenericFlag{
			Name:  "log.level",
			Usage: "set the logging level",
			Value: flags.NewLogLevel(logrus.InfoLevel),
		},
	}
	app.Commands = []*cli.Command{
		agentCommand,
		controllerCommand,
	}
	return app
}

func main() {
	if err := App().Run(os.Args); err != nil {
		logrus.Fatalf("running app: %s", err)
	}
}
