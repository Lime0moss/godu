package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestLockIsStale(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "known_hosts.godu.lock")
	if err := os.WriteFile(lockPath, []byte("123\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-knownHostsLockStaleAfter - time.Second)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	if !lockIsStale(lockPath, knownHostsLockStaleAfter) {
		t.Fatal("expected old lock file to be considered stale")
	}
}

func TestWithKnownHostsLock_RemovesStaleLock(t *testing.T) {
	tmp := t.TempDir()
	knownHosts := filepath.Join(tmp, "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	lockPath := knownHosts + ".godu.lock"
	if err := os.WriteFile(lockPath, []byte("9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-knownHostsLockStaleAfter - time.Second)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	called := false
	if err := withKnownHostsLock(knownHosts, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withKnownHostsLock failed: %v", err)
	}
	if !called {
		t.Fatal("expected callback to be executed")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be removed, stat err=%v", err)
	}
}

func TestWriteKnownHostsAtomic_ReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeKnownHostsAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("writeKnownHostsAtomic failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("expected replaced file content %q, got %q", "new", string(data))
	}
}
