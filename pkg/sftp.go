package pkg

import (
	"fmt"
	"github.com/bmatcuk/doublestar"
	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh"
	"io"
	"os"
	"path/filepath"
)

type sftpClient struct {
	*sftp.Client

	currDir string
}

func newSftpClient(c *ssh.Client) (*sftpClient, error) {
	cc, err := sftp.NewClient(c)
	if err != nil {
		return nil, fmt.Errorf("new sftp client: %w", err)
	}

	wd, err := cc.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	return &sftpClient{
		Client:  cc,
		currDir: wd,
	}, nil
}

func (c *sftpClient) ls(args ...string) {
	dir := c.currDir
	if len(args) > 0 {
		dir = c.remotePath(args[0])
	}

	files, err := c.ReadDir(dir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, f := range files {
		p := f.Name()
		if f.IsDir() {
			p += "/"
		}

		fmt.Println(p)
	}
}

func (c *sftpClient) lls(args ...string) {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, f := range files {
		p := filepath.Join(dir, f.Name())
		if f.IsDir() {
			p += "/"
		}

		fmt.Println(p)
	}
}

func (c *sftpClient) remotePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	p, err := c.RealPath(c.Join(c.currDir, path))
	if err != nil {
		fmt.Println(err.Error())
	}

	return p
}

func (c *sftpClient) lcd(args ...string) {
	if len(args) != 1 {
		fmt.Println("only one argument is accepted")
		return
	}

	if err := os.Chdir(args[0]); err != nil {
		fmt.Println(err.Error())
	}

	c.lpwd()
}

func (c *sftpClient) cd(args ...string) {
	if len(args) != 1 {
		fmt.Println("only one argument is accepted")
		return
	}

	targetDir := c.remotePath(args[0])

	if s, err := c.Stat(targetDir); err != nil {
		fmt.Println(err.Error())
	} else if !s.IsDir() {
		fmt.Println("cannot change directory")
	} else {
		c.currDir = targetDir
	}
}

func (c *sftpClient) pwd() {
	fmt.Println(c.currDir)
}

func (c *sftpClient) lpwd() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(wd)
}

func (c *sftpClient) put(args ...string) {
	if len(args) != 2 {
		fmt.Println("usage: put path-to-local-source path-to-remote-destination")
		return
	}

	srcPath, err := filepath.Abs(args[0])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	src, err := os.Open(srcPath)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() { _ = src.Close() }()

	dstPath, err := c.RealPath(c.remotePath(args[1]))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	dst, err := c.OpenFile(dstPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() { _ = dst.Close() }()

	stat, err := src.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	bar := progressbar.DefaultBytes(stat.Size(), "uploading")

	if _, err := io.Copy(io.MultiWriter(dst, bar), src); err != nil {
		fmt.Println(err.Error())
	}
}

func (c *sftpClient) get(args ...string) {
	if len(args) != 2 {
		fmt.Println("usage: get path-to-remote-source path-to-local-destination")
		return
	}

	srcPath, err := c.RealPath(c.remotePath(args[0]))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	src, err := c.Open(srcPath)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() { _ = src.Close() }()

	dstPath, err := filepath.Abs(args[1])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	dst, err := os.OpenFile(dstPath, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer func() { _ = dst.Close() }()

	stat, err := src.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	bar := progressbar.DefaultBytes(stat.Size(), "uploading")

	if _, err := io.Copy(io.MultiWriter(dst, bar), src); err != nil {
		fmt.Println(err.Error())
	}
}

func (c *sftpClient) rm(args ...string) {
	if len(args) != 1 {
		fmt.Println("usage: rm pattern")
		return
	}

	path := c.remotePath(args[0])

	dir, base := filepath.Dir(path), filepath.Base(path)

	entries, err := c.ReadDir(dir)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, entry := range entries {
		if m, err := doublestar.PathMatch(base, entry.Name()); err != nil || !m {
			continue
		}

		p := filepath.Join(dir, entry.Name())

		s, err := c.Stat(p)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		if s.IsDir() {
			if err := c.RemoveDirectory(p); err != nil {
				fmt.Println("error:", err.Error())
			}
		} else {
			if err := c.Remove(p); err != nil {
				fmt.Println("error:", err.Error())
			}
		}
	}
}
