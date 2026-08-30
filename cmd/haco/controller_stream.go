package main

import (
	"context"
	"errors"
	"io"
	"net"
)

// bridgeControllerStream copies terminal input/output over one bidirectional
// controller stream. Half-close input when supported so a remote shell can see
// EOF without dropping output that is still in flight.
func bridgeControllerStream(ctx context.Context, stream net.Conn, stdin io.Reader, stdout io.Writer) error {
	if stream == nil || stdin == nil || stdout == nil {
		return errors.New("invalid controller stream")
	}
	defer stream.Close()

	inputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stream, stdin)
		if closer, ok := stream.(interface{ CloseWrite() error }); ok {
			if closeErr := closer.CloseWrite(); copyErr == nil {
				copyErr = closeErr
			}
		}
		inputDone <- copyErr
	}()

	_, outputErr := io.Copy(stdout, stream)
	_ = stream.Close()
	if outputErr != nil && !errors.Is(outputErr, net.ErrClosed) && ctx.Err() == nil {
		return outputErr
	}
	select {
	case inputErr := <-inputDone:
		if inputErr != nil && !errors.Is(inputErr, net.ErrClosed) && ctx.Err() == nil {
			return inputErr
		}
	default:
	}
	return ctx.Err()
}
