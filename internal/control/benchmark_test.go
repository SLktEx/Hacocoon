package control

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const hundredGiB int64 = 100 << 30

func BenchmarkUnixStream100GiB(b *testing.B) {
	if os.Getenv("HACO_BENCH_100GB") != "1" {
		b.Skip("set HACO_BENCH_100GB=1 and use -benchtime=1x to run the 100 GiB baseline")
	}
	benchmarkUnixSink(b, hundredGiB)
}

func benchmarkUnixSink(b *testing.B, bytes int64) {
	path := filepath.Join(b.TempDir(), "control.sock")
	listener, err := ListenUnix(path, 0o600)
	if err != nil {
		b.Fatal(err)
	}
	server := NewServer()
	if err := server.RegisterStream("benchmark.sink", func(_ context.Context, payload json.RawMessage) (Stream, error) {
		var request struct {
			Bytes int64 `json:"bytes"`
		}
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, err
		}
		return func(_ context.Context, conn net.Conn) error {
			if _, err := io.CopyN(io.Discard, conn, request.Bytes); err != nil {
				return err
			}
			_, err := conn.Write([]byte{1})
			return err
		}, nil
	}); err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			b.Error("benchmark server did not stop")
		}
	}()
	client, err := NewClient(UnixDialer(path))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(bytes)
	b.ResetTimer()
	for range b.N {
		stream, err := client.OpenStream(context.Background(), "benchmark.sink", struct {
			Bytes int64 `json:"bytes"`
		}{Bytes: bytes})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.CopyN(stream, zeroReader{}, bytes); err != nil {
			stream.Close()
			b.Fatal(err)
		}
		var ack [1]byte
		if _, err := io.ReadFull(stream, ack[:]); err != nil {
			stream.Close()
			b.Fatal(err)
		}
		if err := stream.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
