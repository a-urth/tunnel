package pkg

import (
	"context"
	"errors"
	"fmt"
	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	chc "github.com/jpillora/chisel/client"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	HostID string
	Proxy  string
	Auth   string

	RetryPeriod time.Duration
	RelayServer string
	EnableSFTP  bool
}

func Host(cctx *cli.Context, cfg Config) error {
	ctx := cctx.Context

	id, err := getHostID(cfg.HostID)
	if err != nil {
		return fmt.Errorf("get host id: %w", err)
	}

	port, err := getPortFromStr(id)
	if err != nil {
		return fmt.Errorf("get port from host id: %w", err)
	}

	logrus.WithField("port", port).Debug("starting on")

	startChiselClient(ctx, 0, port, true, cfg)

	if err := startSSHServer(ctx, port, cfg.EnableSFTP); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return fmt.Errorf("start ssh server: %w", err)
	}

	return nil
}

func startChiselClient(ctx context.Context, lp, rp int, reversed bool, cfg Config) {
	var parts []string

	if reversed {
		parts = append(parts, "R")
	}

	if lp > 0 {
		parts = append(parts, fmt.Sprintf("%d", lp))
	}

	parts = append(parts, fmt.Sprintf("%d", rp))

	ccfg := chc.Config{
		Server:        cfg.RelayServer,
		Proxy:         cfg.Proxy,
		Auth:          cfg.Auth,
		Headers:       http.Header{},
		Remotes:       []string{strings.Join(parts, ":")},
		Verbose:       true,
		MaxRetryCount: 1,
	}

	go func() {
		defer func() {
			logrus.Debug("chisel worker is stopping")
		}()

		t := time.NewTicker(cfg.RetryPeriod)
		defer t.Stop()

		for {
			logrus.WithFields(
				logrus.Fields{
					"server":  ccfg.Server,
					"auth":    ccfg.Auth,
					"proxy":   ccfg.Proxy,
					"remotes": ccfg.Remotes,
				},
			).Debug("starting chisel worker")

			c, err := chc.NewClient(&ccfg)
			if err != nil {
				logrus.WithError(err).Error("new chisel client")
				return
			}

			if err := c.Start(ctx); err != nil {
				logrus.WithError(err).Error("chisel client run")
			}

			if err := c.Wait(); err != nil {
				logrus.WithError(err).Error("chisel client run")
			}

			select {
			case <-ctx.Done():
				_ = c.Close()
				return
			case <-t.C:
				break
			}
		}
	}()
}

func startSSHServer(ctx context.Context, port int, enableSFTP bool) error {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Println("error:", err.Error())

		w, h = 80, 30
	}

	server := ssh.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", port),
		Handler: ssh.Handler(func(s ssh.Session) {
			ptyReq, winCh, isPty := s.Pty()
			if !isPty {
				if _, err := io.WriteString(s, "No PTY requested.\n"); err != nil {
					logrus.WithError(err).Warn("write to session")
					return
				}

				if err := s.Exit(1); err != nil {
					logrus.WithError(err).Warn("exit session")
					return
				}

				return
			}

			logger := logrus.WithField("addr", s.RemoteAddr().String())

			logger.Debug("session started")

			exe, err := os.Executable()
			if err != nil {
				exe = os.Args[0]
			}

			cmd := exec.Command(exe, "sh")

			// pass local env to shell and add folder with current binary to PATH, so it wll be always accessible
			cmd.Env = append(
				os.Environ(),
				fmt.Sprintf("PATH=%s%c%s", filepath.Dir(exe), os.PathListSeparator, os.Getenv("PATH")),
			)

			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))

			f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)})
			if err != nil {
				logger.WithError(err).Warn("pty start")
				return
			}

			defer func() { _ = f.Close() }()

			// handle window resize
			go func() {
				for win := range winCh {
					if err := pty.Setsize(f,
						&pty.Winsize{
							Cols: uint16(win.Width),
							Rows: uint16(win.Height),
						}); err != nil {
						logger.WithError(err).Warn("resize pty")
					}
				}
			}()

			// input
			go func() { _, _ = io.Copy(f, s) }()

			// output
			go func() { _, _ = io.Copy(s, f) }()

			if err := cmd.Wait(); err != nil {
				logger.WithError(err).Warn("wait for command")
			}

			logger.Debug("session ended")
		}),
	}

	if enableSFTP {
		server.SubsystemHandlers = map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		}
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve ssh: %w", err)
	}

	return nil
}

func sftpHandler(sess ssh.Session) {
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(io.Discard),
	}

	server, err := sftp.NewServer(sess, serverOptions...)
	if err != nil {
		logrus.WithError(err).Debug("sftp server init error")
		return
	}

	if err := server.Serve(); err != nil {
		if errors.Is(err, io.EOF) {
			_ = server.Close()

			logrus.Debug("sftp client exited session.")

			return
		}

		logrus.WithError(err).Debug("sftp server completed with error")
	}
}
