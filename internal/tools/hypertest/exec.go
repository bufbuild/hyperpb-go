// Copyright 2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	osuser "os/user"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"buf.build/go/hyperpb/internal/xerrors"
)

var errFailed = errors.New("tests failed")

// runner is all the information necessary to build and run a test.
type runner struct {
	tool     string   // Path to the go tool.
	pkgs     string   // Target to build.
	output   string   // Output directory.
	tags     string   // Build tags to use.
	profile  bool     // If set, -cpuprofile will be set.
	checkptr bool     // Whether to build with -c=checkptr.
	race     bool     // Whether to build with -race.
	args     []string // Args for the test binary(s).
}

type test string

func (t test) binary(r *runner, cwd string) string {
	if cwd == "" {
		cwd = r.output
	}

	return filepath.Join(cwd, string(t)+".test")
}

func (t test) profile(r *runner, cwd string) string {
	if cwd == "" {
		cwd = r.output
	}

	return filepath.Join(cwd, string(t)+".prof")
}

// build runs go test to build the requested tests.
func (r *runner) build() ([]test, error) {
	// Clean the output directory.
	if err := os.RemoveAll(r.output); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if err := os.MkdirAll(r.output, 0o777); err != nil {
		return nil, err
	}

	args := []string{
		"test",
		r.pkgs,
		"-c",
		"-o", r.output,
		"-tags", r.tags,
	}

	if r.tags != "" {
		args = append(args, "-tags", r.tags)
	}
	if r.checkptr {
		args = append(args, "-gcflags=-d=checkptr=1")
	}
	if r.race {
		args = append(args, "-race")
	}

	// Build the command we're going to run.
	cmd := exec.Command(r.tool, args...)
	cmd.Env = os.Environ()
	fmt.Printf("running: %s %s\n", cmd.Path, strings.Join(cmd.Args, " "))
	if out, err := cmd.CombinedOutput(); err != nil {
		if exit, ok := xerrors.As[*exec.ExitError](err); ok {
			exit.Stderr = out
		}

		return nil, err
	}

	// Enumerate the test binaries.
	dir, err := os.ReadDir(r.output)
	if err != nil {
		return nil, err
	}
	var tests []test
	for _, path := range dir {
		if name, ok := strings.CutSuffix(path.Name(), ".test"); ok {
			tests = append(tests, test(name))
		}
	}
	fmt.Printf("built tests: %q\n", tests)
	return tests, nil
}

func (r *runner) runLocally(tests []test) (string, error) {
	var stdout strings.Builder
	var failed bool
	for _, test := range tests {
		args := r.args
		if r.profile {
			args = append(args, "-test.cpuprofile", test.profile(r, ""))
		}

		// Run it locally.
		cmd := exec.Command(test.binary(r, ""), args...)
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = os.Stderr

		start := time.Now()
		err := cmd.Run()
		time := time.Since(start)

		what := "ok"
		if err != nil {
			if exit, ok := xerrors.As[*exec.ExitError](err); ok && exit.ExitCode() != 0 {
				what = "FAILED"
			} else {
				fmt.Printf("error: %v\n", err)
				what = "?"
			}
			failed = true
		}

		fmt.Printf("%s\t%s\t%.3vs\n", what, test.binary(r, ""), time.Seconds())
	}

	if failed {
		return "", errFailed
	}
	return stdout.String(), nil
}

func (r *runner) runOverSSH(remote string, tests []test) (string, error) {
	// Dial an SSH connection, if requested.
	user, addr, hasUser := strings.Cut(remote, "@")
	if !hasUser {
		addr = user
		u, err := osuser.Current()
		if err != nil {
			return "", err
		}
		user = u.Username
	}
	auth, _ := goph.UseAgent()
	auth = append(auth, ssh.KeyboardInteractive(askStdin))

	// We're cool with not checking known_hosts; remote execution is only used
	// for development.
	ssh, err := goph.NewUnknown(user, addr, auth)
	if err != nil {
		return "", fmt.Errorf("could not dial remote host: %w", err)
	}
	defer ssh.Close()
	fmt.Printf("dialed ssh://%s@%s\n", user, addr)

	// Create a temporary directory to stick test binaries in.
	mktemp, err := ssh.Command("mktemp", "-d", "/tmp/hyperpb-benchmark.XXXXXXXXX")
	if err != nil {
		return "", fmt.Errorf("could not create tempdir: %w", err)
	}
	path, err := mktemp.Output()
	if err != nil {
		return "", fmt.Errorf("could not create tempdir: %w", err)
	}
	tmpdir := strings.TrimSpace(string(path))
	defer func() {
		rmdir, err := ssh.Command("rm", "-r", tmpdir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not clean tempdir: %v", err)
		}
		if err := rmdir.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "could not clean tempdir: %v", err)
		}
	}()
	fmt.Printf("created remote tempdir: %s\n", tmpdir)

	// Upload all of the tests in parallel.
	sftp, err := ssh.NewSftp()
	if err != nil {
		return "", err
	}
	wg := new(sync.WaitGroup)
	syncErr := new(atomic.Pointer[error])
	for _, test := range tests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			{
				start := time.Now()
				src, err := os.Open(test.binary(r, ""))
				if err != nil {
					goto error
				}
				defer src.Close()

				dst, err := sftp.Create(test.binary(r, tmpdir))
				if err != nil {
					goto error
				}
				defer dst.Close()

				if err := dst.Chmod(0o777); err != nil {
					goto error
				}

				if _, err := io.Copy(dst, src); err != nil {
					goto error
				}

				fmt.Printf("uploaded %s in %.3vs\n", test.binary(r, ""), time.Since(start).Seconds())
				return
			}
		error:
			syncErr.CompareAndSwap(nil, &err)
		}()
	}
	wg.Wait()
	if err := syncErr.Load(); err != nil {
		return "", *err
	}

	// Run the tests in series.
	var stdout strings.Builder
	var failed bool
	for _, test := range tests {
		args := r.args
		if r.profile {
			args = append(args, "-test.cpuprofile", test.profile(r, tmpdir))
		}

		for i, arg := range args {
			// goph doesn't know that it has to escape shell arguments >_>
			args[i] = shellescape.Quote(arg)
		}

		cmd, err := ssh.Command(test.binary(r, tmpdir), args...)
		if err != nil {
			return "", err
		}
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = os.Stderr

		start := time.Now()
		err = cmd.Run()
		time := time.Since(start)

		what := "ok"
		if err != nil {
			if exit, ok := xerrors.As[*exec.ExitError](err); ok && exit.ExitCode() != 0 {
				what = "FAILED"
			} else {
				fmt.Printf("error: %v\n", err)
				what = "?"
			}
			failed = true
		}

		fmt.Printf("%s\t%s\t%.3vs\n", what, test.binary(r, tmpdir), time.Seconds())

		if r.profile && what == "ok" {
			// Download the profile.
			err := ssh.Download(
				test.profile(r, tmpdir),
				test.profile(r, ""),
			)
			if err != nil {
				return "", err
			}
			fmt.Printf("downloaded %s\n", test.profile(r, ""))
		}
	}

	if failed {
		return "", errFailed
	}
	return stdout.String(), nil
}

func askStdin(name, instruction string, questions []string, echos []bool) (answers []string, err error) {
	if len(questions) == 0 && name != "" {
		fmt.Printf("%s: %s\n", name, instruction)
	}

	answers = make([]string, len(questions))
	for i, q := range questions {
		fmt.Printf("%s ", q)
		if echos[i] {
			_, err := fmt.Scan("%s", &answers[i])
			if err != nil {
				return nil, err
			}
			continue
		}

		answer, err := term.ReadPassword(syscall.Stdin)
		fmt.Println()
		if err != nil {
			return nil, err
		}
		answers[i] = string(answer)
	}

	return answers, nil
}
