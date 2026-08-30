package control

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const maxControlEnvelopeBytes = 16 << 20

func readEnvelopeLine(reader *bufio.Reader) ([]byte, error) {
	if reader == nil {
		return nil, ErrInvalidArgument
	}
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxControlEnvelopeBytes {
			return nil, fmt.Errorf("control envelope exceeds %d bytes: %w", maxControlEnvelopeBytes, ErrProtocol)
		}
		line = append(line, fragment...)
		if err == nil {
			return line, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
}
