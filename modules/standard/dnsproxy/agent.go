package dnsproxy

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// RunAgent is the untrusted guest-side adapter. It owns no credentials, provider
// handles or control socket. Its only upstream is the existing fixed relay.
func RunAgent(ctx context.Context) error {
	udp, err := net.ListenPacket("udp4", "127.0.0.1:53")
	if err != nil {
		return fmt.Errorf("bind guest DNS UDP listener: %w", err)
	}
	tcp, err := net.Listen("tcp4", "127.0.0.1:53")
	if err != nil {
		udp.Close()
		return fmt.Errorf("bind guest DNS TCP listener: %w", err)
	}
	defer udp.Close()
	defer tcp.Close()
	if endpoint := os.Getenv("NOTIFY_SOCKET"); endpoint != "" {
		conn, err := net.DialTimeout("unixgram", endpoint, time.Second)
		if err != nil {
			return fmt.Errorf("notify guest DNS readiness: %w", err)
		}
		err = conn.SetWriteDeadline(time.Now().Add(time.Second))
		if err == nil {
			_, err = conn.Write([]byte("READY=1"))
		}
		conn.Close()
		if err != nil {
			return fmt.Errorf("notify guest DNS readiness: %w", err)
		}
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, MaxResponseHeaderBytes: 4096}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return serveAgent(ctx, udp, tcp, func(ctx context.Context, input []byte) []byte { return forwardHTTP(ctx, client, Endpoint, input) })
}
func forwardHTTP(ctx context.Context, client *http.Client, endpoint string, input []byte) []byte {
	query, err := question(input)
	if err != nil {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(input))
	if err != nil {
		return failure(input)
	}
	request.Header.Set("Content-Type", ContentType)
	response, err := client.Do(request)
	if err != nil {
		return failure(input)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != ContentType {
		return failure(input)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxMessageBytes+1))
	if err != nil || len(data) > MaxMessageBytes {
		return failure(input)
	}
	var answer dnsmessage.Message
	if answer.Unpack(data) != nil || !answer.Response || answer.ID != query.ID || len(answer.Questions) != 1 || answer.Questions[0] != query.Questions[0] {
		return failure(input)
	}
	return data
}

type forwardFunc func(context.Context, []byte) []byte

func serveAgent(parent context.Context, udp net.PacketConn, tcp net.Listener, forward forwardFunc) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer udp.Close()
	defer tcp.Close()
	stop := context.AfterFunc(ctx, func() { udp.Close(); tcp.Close() })
	defer stop()
	var workers sync.WaitGroup
	slots := make(chan struct{}, 32)
	results := make(chan error, 2)
	go func() {
		input := make([]byte, MaxMessageBytes+1)
		for {
			n, peer, err := udp.ReadFrom(input)
			if err != nil {
				results <- err
				return
			}
			if n > MaxMessageBytes {
				continue
			}
			select {
			case slots <- struct{}{}:
				data := append([]byte(nil), input[:n]...)
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-slots }()
					queryCtx, done := context.WithTimeout(ctx, 6*time.Second)
					defer done()
					answer := forward(queryCtx, data)
					if len(answer) > 512 {
						query, err := question(data)
						if err != nil {
							return
						}
						answer = reply(query, dnsmessage.RCodeSuccess, nil)
						if len(answer) < 12 {
							return
						}
						binary.BigEndian.PutUint16(answer[2:4], binary.BigEndian.Uint16(answer[2:4])|0x0200)
					}
					if len(answer) > 0 {
						_, _ = udp.WriteTo(answer, peer)
					}
				}()
			default:
				// Drop overload; never queue an unbounded amount of untrusted work.
			}
		}
	}()
	go func() {
		for {
			conn, err := tcp.Accept()
			if err != nil {
				results <- err
				return
			}
			select {
			case slots <- struct{}{}:
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-slots }()
					defer conn.Close()
					stop := context.AfterFunc(ctx, func() { conn.Close() })
					defer stop()
					if conn.SetDeadline(time.Now().Add(6*time.Second)) != nil {
						return
					}
					var prefix [2]byte
					if _, err := io.ReadFull(conn, prefix[:]); err != nil {
						return
					}
					size := int(binary.BigEndian.Uint16(prefix[:]))
					if size < 12 || size > MaxMessageBytes {
						return
					}
					data := make([]byte, size)
					if _, err := io.ReadFull(conn, data); err != nil {
						return
					}
					queryCtx, done := context.WithTimeout(ctx, 6*time.Second)
					defer done()
					answer := forward(queryCtx, data)
					if len(answer) == 0 || len(answer) > MaxMessageBytes {
						return
					}
					binary.BigEndian.PutUint16(prefix[:], uint16(len(answer)))
					_, _ = io.Copy(conn, bytes.NewReader(append(prefix[:], answer...)))
				}()
			default:
				conn.Close()
			}
		}
	}()
	err := <-results
	cancel()
	<-results
	workers.Wait()
	if parent.Err() != nil {
		return parent.Err()
	}
	return fmt.Errorf("guest DNS listener stopped: %w", err)
}
