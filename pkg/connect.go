package pkg

import (
	"errors"
	"fmt"
	"github.com/mattn/go-shellwords"
	"github.com/peterh/liner"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"io"
	"os"
	"time"
)

func Connect(cctx *cli.Context, cfg Config) error {
	id, err := getHostID(cfg.HostID)
	if err != nil {
		return fmt.Errorf("get host id: %w", err)
	}

	rp, err := getPortFromStr(id)
	if err != nil {
		return fmt.Errorf("get port from host id: %w", err)
	}

	lp, err := GetFreePort()
	if err != nil {
		return fmt.Errorf("get local port: %w", err)
	}

	ctx := cctx.Context

	startChiselClient(ctx, lp, rp, false, cfg)

	// give some time for chisel to connect to server before attempting to create ssh connection
	time.Sleep(1 * time.Second)

	ccfg := ssh.ClientConfig{
		Timeout:         cfg.RetryPeriod,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
	}

	c, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", lp), &ccfg)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}

	defer func() { _ = c.Close() }()

	if cfg.EnableSFTP {
		err = startSftpSession(c)
	} else {
		err = startSSHSession(c)
	}

	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("start session: %w", err)
	}

	return nil
}

func startSSHSession(c *ssh.Client) error {
	sess, err := c.NewSession()
	if err != nil {
		return fmt.Errorf("new ssh session: %w", err)
	}

	defer func() { _ = sess.Close() }()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		logrus.WithError(err).Warning("get terminal size")

		w, h = 80, 80
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("put terminal in raw mode: %w", err)
	}

	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	if err := sess.RequestPty("xterm-256color", h, w, modes); err != nil {
		return fmt.Errorf("request session pty: %w", err)
	}

	sshIn, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("ssh session stdin pipe: %w", err)
	}

	sshOut, err := sess.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ssh session stdout pipe: %w", err)
	}

	sshErr, err := sess.StderrPipe()
	if err != nil {
		return fmt.Errorf("ssh session stderr pine: %w", err)
	}

	go func() { _, _ = io.Copy(os.Stdout, sshOut) }()

	go func() { _, _ = io.Copy(os.Stderr, sshErr) }()

	go func() { _, _ = io.Copy(sshIn, os.Stdin) }()

	if err := sess.Shell(); err != nil {
		return fmt.Errorf("start session shell: %w", err)
	}

	if err := sess.Wait(); err != nil {
		return fmt.Errorf("session shell wait: %w", err)
	}

	return nil
}

func startSftpSession(c *ssh.Client) error {
	cc, err := newSftpClient(c)
	if err != nil {
		return fmt.Errorf("new sftp client: %w", err)
	}

	defer func() { _ = cc.Close() }()

	line := liner.NewLiner()
	defer func() { _ = line.Close() }()

	for {
		l, err := line.Prompt("sftp> ")
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}

		parts, err := shellwords.Parse(l)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		if len(parts) == 0 {
			continue
		}

		command, args := parts[0], parts[1:]

		switch command {
		case `ls`:
			cc.ls(args...)
		case `lls`:
			cc.lls(args...)
		case `lcd`:
			cc.lcd(args...)
		case `cd`:
			cc.cd(args...)
		case `pwd`:
			cc.pwd()
		case `lpwd`:
			cc.lpwd()
		case `put`:
			cc.put(args...)
		case `get`:
			cc.get(args...)
		case `rm`:
			cc.rm(args...)
		case `exit`:
			return nil
		default:
			fmt.Println("unknown command")
		}
	}
}
