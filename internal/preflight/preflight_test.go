package preflight

import (
	"net"
	"testing"
)

func TestPortChecksReportOccupiedTCPPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	checks := portChecks([]string{"127.0.0.1"}, []PortBinding{{Name: "test", Port: port}})
	if len(checks) != 1 || checks[0].Status != "failed" {
		t.Fatalf("expected occupied TCP port to fail, got %#v", checks)
	}
}

func TestPortChecksReportOccupiedUDPPort(t *testing.T) {
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.LocalAddr().(*net.UDPAddr).Port

	checks := portChecks([]string{"127.0.0.1"}, []PortBinding{{Name: "discovery", Port: port, Protocol: "udp"}})
	if len(checks) != 1 || checks[0].Status != "failed" {
		t.Fatalf("expected occupied UDP port to fail, got %#v", checks)
	}
}

func TestPortChecksReportAvailablePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	checks := portChecks([]string{"127.0.0.1"}, []PortBinding{{Name: "test", Port: port}})
	if len(checks) != 1 || checks[0].Status != "healthy" {
		t.Fatalf("expected available TCP port to pass, got %#v", checks)
	}
}
