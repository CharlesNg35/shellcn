package transport

import (
	"bytes"
	"io"
	"net"
	"testing"
)

func TestDatagramFramingRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	msgs := [][]byte{[]byte("snmp-get"), {}, bytes.Repeat([]byte("x"), 1500)}
	go func() {
		for _, m := range msgs {
			_ = WriteDatagram(client, m)
		}
	}()
	for i, want := range msgs {
		got, err := ReadDatagram(server)
		if err != nil {
			t.Fatalf("msg %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("msg %d = %q; want %q", i, got, want)
		}
	}
}

func TestWriteDatagramRejectsOversize(t *testing.T) {
	if err := WriteDatagram(io.Discard, make([]byte, maxDatagram+1)); err == nil {
		t.Fatal("expected error for oversize datagram")
	}
}

func TestDatagramConnPreservesBoundaries(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	conn := NewDatagramConn(client)
	go func() {
		_, _ = conn.Write([]byte("aaa"))
		_, _ = conn.Write([]byte("bb"))
	}()

	first, err := ReadDatagram(server)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReadDatagram(server)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "aaa" || string(second) != "bb" {
		t.Fatalf("boundaries not preserved: %q %q", first, second)
	}
}
