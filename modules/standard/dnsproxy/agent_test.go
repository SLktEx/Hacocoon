package dnsproxy

import (
	"context"
	"encoding/binary"
	"errors"
	"golang.org/x/net/dns/dnsmessage"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGuestDNSAgentServesUDPAndTCPAndClosesIdleConnections(t *testing.T) {
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		udp.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serveAgent(ctx, udp, tcp, func(_ context.Context, input []byte) []byte {
			query, err := question(input)
			if err != nil {
				return nil
			}
			return reply(query, dnsmessage.RCodeRefused, nil)
		})
	}()
	query := packet(t, dnsmessage.TypeA)
	client, err := net.Dial("udp4", udp.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(query); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4096)
	n, err := client.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	var response dnsmessage.Message
	if response.Unpack(data[:n]) != nil || response.RCode != dnsmessage.RCodeRefused {
		t.Fatal("UDP answer lost")
	}
	stream, err := net.Dial("tcp4", tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	stream.SetDeadline(time.Now().Add(time.Second))
	prefix := []byte{byte(len(query) >> 8), byte(len(query))}
	if _, err := stream.Write(append(prefix, query...)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(stream, prefix); err != nil {
		t.Fatal(err)
	}
	n = int(binary.BigEndian.Uint16(prefix))
	if _, err := io.ReadFull(stream, data[:n]); err != nil {
		t.Fatal(err)
	}
	if response.Unpack(data[:n]) != nil || response.RCode != dnsmessage.RCodeRefused {
		t.Fatal("TCP answer lost")
	}
	idle, err := net.Dial("tcp4", tcp.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("idle connection blocked DNS shutdown")
	}
}
func TestGuestDNSRejectsUnrelatedRelayReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		query, err := question(data)
		if err != nil {
			t.Error(err)
			return
		}
		query.ID++
		w.Header().Set("Content-Type", ContentType)
		w.Write(reply(query, dnsmessage.RCodeSuccess, nil))
	}))
	defer server.Close()
	data := forwardHTTP(context.Background(), server.Client(), server.URL, packet(t, dnsmessage.TypeA))
	var answer dnsmessage.Message
	if answer.Unpack(data) != nil || answer.ID != 42 || answer.RCode != dnsmessage.RCodeServerFailure {
		t.Fatal("unrelated response accepted")
	}
}
