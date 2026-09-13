// haco-review presents exact approval choices inside Windows notifications.
// The controller's common review session owns Policy, audit and execution.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/desktopreview"
	"io"
	"os"
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
	id := ""
	if args[0] != "--toast-server" {
		id, err = desktopreview.RequestFromURI(c.Distribution, args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Invalid Hacocoon review link.")
		return 2
	}
	if err := nativeReview(c, own, id); err != nil {
		if errors.Is(err, desktopreview.ErrNoLongerPending) {
			fmt.Fprintln(os.Stderr, "Hacocoon request is no longer pending.")
		} else {
			reportReviewFailure(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "Windows notification review is unavailable. No answer was automatically retried.")
		}
		return 1
	}
	return 0
}
func main() {
	code := run(os.Args[1:])
	os.Exit(code)
}
