// haco-review is the optional native Windows protocol handler. It is not a
// second user-facing haco CLI and never decides an approval.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/desktopreview"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type configuration struct {
	Distribution string `json:"distribution"`
}

func loadConfiguration(path string) (configuration, error) {
	var c configuration
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return c, errors.New("invalid local review configuration")
	}
	f, err := os.Open(path)
	if err != nil {
		return c, errors.New("cannot open local review configuration")
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 4097))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil {
		return c, errors.New("invalid local review configuration")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return c, errors.New("invalid local review configuration")
	}
	if _, err := desktopreview.Scheme(c.Distribution); err != nil {
		return c, err
	}
	return c, nil
}
func run(args []string) int {
	if runtime.GOOS != "windows" || len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Hacocoon review requires one Windows notification link.")
		return 2
	}
	own, err := os.Executable()
	if err != nil {
		return 1
	}
	c, err := loadConfiguration(filepath.Join(filepath.Dir(own), "review.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	p, err := desktopreview.Plan(c.Distribution, args[0], os.Getenv("SystemRoot"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid Hacocoon review link.")
		return 2
	}
	cmd := exec.Command(p.File, p.Args...)
	cmd.Env = p.Env
	cmd.Dir = filepath.Dir(own)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Review ended without confirmed success. Inspect Policy and audit before retrying.")
		return 1
	}
	return 0
}
func main() {
	code := run(os.Args[1:])
	// The protocol opens a console. Keep its exact receipt visible for the user.
	fmt.Fprintln(os.Stderr, "Press Enter to close.")
	var line string
	fmt.Fscanln(os.Stdin, &line)
	os.Exit(code)
}
