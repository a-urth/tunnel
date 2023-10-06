package main

import (
	"context"
	"fmt"
	"github.com/a-urth/tunnel/pkg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"time"
)

var App = &cli.App{
	Name:  "tunnel",
	Usage: "start tunnel ssh server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "srv",
			Usage:    "server address",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "auth",
			Usage: "server auth",
		},
		&cli.StringFlag{
			Name:  "proxy",
			Usage: "proxy",
		},
		&cli.StringFlag{
			Name:    "host-id",
			Aliases: []string{"id"},
			Usage:   "id of the host to connect to or file with it",
		},
		&cli.DurationFlag{
			Name:        "retry",
			Usage:       "time to wait for retry",
			Value:       10 * time.Second,
			DefaultText: "10s",
		},
		&cli.BoolFlag{
			Name:  "sftp",
			Usage: "if set then sftp will be started",
		},
	},
	Commands: []*cli.Command{
		{
			Name:  "host",
			Usage: "open tunnel and host ssh server",
			Action: func(cctx *cli.Context) error {
				cfg := pkg.Config{
					HostID:      cctx.String("host-id"),
					Proxy:       cctx.String("proxy"),
					Auth:        cctx.String("auth"),
					RetryPeriod: cctx.Duration("retry"),
					RelayServer: cctx.String("srv"),
					EnableSFTP:  cctx.Bool("sftp"),
				}

				if err := pkg.Host(cctx, cfg); err != nil {
					return fmt.Errorf("host: %w", err)
				}

				return nil
			},
		},
		{
			Name:  "connect",
			Usage: "open tunnel and connect to ssh host",
			Action: func(cctx *cli.Context) error {
				cfg := pkg.Config{
					HostID:      cctx.String("host-id"),
					Proxy:       cctx.String("proxy"),
					Auth:        cctx.String("auth"),
					RetryPeriod: cctx.Duration("retry"),
					RelayServer: cctx.String("srv"),
					EnableSFTP:  cctx.Bool("sftp"),
				}

				if err := pkg.Connect(cctx, cfg); err != nil {
					return fmt.Errorf("connect: %w", err)
				}

				return nil
			},
		},
	},
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background())
	defer cancel()

	if err := App.RunContext(ctx, os.Args); err != nil {
		logrus.Fatalln(err)
	}
}
