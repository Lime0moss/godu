package remote

import (
	"strings"
	"testing"
)

func TestParseSSHTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		user    string
		host    string
		wantErr bool
	}{
		{name: "valid", target: "alice@example.com", user: "alice", host: "example.com"},
		{name: "empty", target: "", wantErr: true},
		{name: "no at", target: "example.com", wantErr: true},
		{name: "missing user", target: "@example.com", wantErr: true},
		{name: "missing host", target: "alice@", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user, host, err := parseSSHTarget(tc.target)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user != tc.user || host != tc.host {
				t.Fatalf("unexpected result: got %q@%q want %q@%q", user, host, tc.user, tc.host)
			}
		})
	}
}

func TestCleanRemotePath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: "."},
		{in: ".", want: "."},
		{in: "/tmp/../var", want: "/var"},
		{in: `C:\temp\x`, want: "C:/temp/x"},
	}

	for _, tc := range tests {
		if got := cleanRemotePath(tc.in); got != tc.want {
			t.Fatalf("cleanRemotePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestKnownHostAddress(t *testing.T) {
	if got := knownHostAddress("example.com", 22); got != "example.com" {
		t.Fatalf("unexpected address for port 22: %q", got)
	}
	if got := knownHostAddress("example.com", 2222); got != "[example.com]:2222" {
		t.Fatalf("unexpected address for custom port: %q", got)
	}
}

func TestRemoveKnownHostEntries(t *testing.T) {
	input := strings.Join([]string{
		"example.com ssh-ed25519 AAAA",
		"[example.com]:22 ssh-ed25519 BBBB",
		"[example.com]:2222 ssh-ed25519 CCCC",
		"other.com ssh-ed25519 DDDD",
		"",
	}, "\n")

	out22 := string(removeKnownHostEntries([]byte(input), "example.com", 22))
	if strings.Contains(out22, "example.com ssh-ed25519 AAAA") {
		t.Fatal("expected plain host entry removed for port 22")
	}
	if strings.Contains(out22, "[example.com]:22 ssh-ed25519 BBBB") {
		t.Fatal("expected bracketed :22 entry removed")
	}
	if !strings.Contains(out22, "[example.com]:2222 ssh-ed25519 CCCC") {
		t.Fatal("expected non-target port entry to remain")
	}

	out2222 := string(removeKnownHostEntries([]byte(input), "example.com", 2222))
	if strings.Contains(out2222, "[example.com]:2222 ssh-ed25519 CCCC") {
		t.Fatal("expected custom port entry removed")
	}
	if !strings.Contains(out2222, "example.com ssh-ed25519 AAAA") {
		t.Fatal("expected default host entry to remain when replacing custom port")
	}
	if !strings.Contains(out2222, "[example.com]:22 ssh-ed25519 BBBB") {
		t.Fatal("expected :22 entry to remain when replacing custom port")
	}
	if !strings.Contains(out2222, "other.com ssh-ed25519 DDDD") {
		t.Fatal("expected unrelated host entry to remain")
	}
}
