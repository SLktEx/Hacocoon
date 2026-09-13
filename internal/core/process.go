package core

import "strings"

// ProcessRequest describes streamed execution without transporting Host state,
// credentials, arbitrary environment variables or provider command options.
type ProcessRequest struct {
	WorkingDirectory string
	Argv             []string
	TTY              bool
}

func ValidateProcessRequest(request ProcessRequest) error {
	if len(request.Argv) == 0 || len(request.Argv) > 256 || request.Argv[0] == "" {
		return ErrInvalidArgument
	}
	size := 0
	for _, argument := range request.Argv {
		size += len(argument)
		if size > 32<<10 || strings.ContainsRune(argument, 0) {
			return ErrInvalidArgument
		}
	}
	if request.WorkingDirectory != "" && (!strings.HasPrefix(request.WorkingDirectory, "/") || strings.ContainsAny(request.WorkingDirectory, "\x00\r\n")) {
		return ErrInvalidArgument
	}
	return nil
}
