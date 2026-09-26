package cli

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"reflect"
	"strings"
	"testing"
)

type execCLIStub struct {
	name    string
	request core.ProcessRequest
	input   string
}

func (f *execCLIStub) ExecStream(_ context.Context, name string, r core.ProcessRequest, in io.Reader, out, errout io.Writer) (core.ExecutionResult, error) {
	f.name = name
	f.request = r
	data, _ := io.ReadAll(in)
	f.input = string(data)
	io.WriteString(out, "stdout")
	io.WriteString(errout, "stderr")
	return core.ExecutionResult{ExitCode: 17}, nil
}
func TestExecCLIParsesOnlyItsOwnFlags(t *testing.T) {
	for _, input := range []bool{false, true} {
		f := &execCLIStub{}
		args := []string{"dev", "--", "sh", "-ec", "printf --help"}
		if input {
			args = append([]string{"-i"}, args...)
		}
		var out, errout bytes.Buffer
		code := execCommand(context.Background(), f, args, strings.NewReader("input"), &out, &errout)
		if code != 17 || f.name != "dev" || !reflect.DeepEqual(f.request.Argv, []string{"sh", "-ec", "printf --help"}) || f.request.TTY || out.String() != "stdout" || errout.String() != "stderr" {
			t.Fatal(code, f)
		}
		if (f.input == "input") != input {
			t.Fatal("-i ignored")
		}
	}
}
