package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveScanTarget_DefaultLocal(t *testing.T) {
	target, err := resolveScanTarget(nil)
	if err != nil {
		t.Fatalf("resolveScanTarget returned error: %v", err)
	}
	if target.Remote {
		t.Fatal("expected local target")
	}
	if target.LocalPath != "." {
		t.Fatalf("unexpected local path: %q", target.LocalPath)
	}
}

func TestResolveScanTarget_ExistingLocalPathWins(t *testing.T) {
	root := t.TempDir()
	localPath := filepath.Join(root, "alice@server")
	if err := os.Mkdir(localPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	target, err := resolveScanTarget([]string{localPath})
	if err != nil {
		t.Fatalf("resolveScanTarget returned error: %v", err)
	}
	if target.Remote {
		t.Fatal("expected local target")
	}
	if target.LocalPath != localPath {
		t.Fatalf("unexpected local path: %q", target.LocalPath)
	}

	_, err = resolveScanTarget([]string{localPath, "/tmp"})
	if err == nil {
		t.Fatal("expected error for extra args in local mode")
	}
}

func TestResolveScanTarget_RemoteDefaultPath(t *testing.T) {
	target, err := resolveScanTarget([]string{"alice@10.0.0.5"})
	if err != nil {
		t.Fatalf("resolveScanTarget returned error: %v", err)
	}
	if !target.Remote {
		t.Fatal("expected remote target")
	}
	if target.SSHDestination != "alice@10.0.0.5" {
		t.Fatalf("unexpected ssh target: %q", target.SSHDestination)
	}
	if target.RemotePath != "." {
		t.Fatalf("unexpected remote path: %q", target.RemotePath)
	}
}

func TestResolveScanTarget_RemoteCustomPath(t *testing.T) {
	target, err := resolveScanTarget([]string{"alice@10.0.0.5", "/var/log"})
	if err != nil {
		t.Fatalf("resolveScanTarget returned error: %v", err)
	}
	if !target.Remote {
		t.Fatal("expected remote target")
	}
	if target.RemotePath != "/var/log" {
		t.Fatalf("unexpected remote path: %q", target.RemotePath)
	}
}

func TestResolveScanTarget_RejectsHostPortInTarget(t *testing.T) {
	_, err := resolveScanTarget([]string{"alice@example.com:2222"})
	if err == nil {
		t.Fatal("expected error for host:port target")
	}
	if !strings.Contains(err.Error(), "--ssh-port") {
		t.Fatalf("expected ssh-port hint, got: %v", err)
	}
}

func TestResolveScanTarget_BracketedIPv6Remote(t *testing.T) {
	target, err := resolveScanTarget([]string{"alice@[::1]"})
	if err != nil {
		t.Fatalf("resolveScanTarget returned error: %v", err)
	}
	if !target.Remote {
		t.Fatal("expected remote target")
	}
	if target.SSHDestination != "alice@[::1]" {
		t.Fatalf("unexpected ssh target: %q", target.SSHDestination)
	}
}

func TestResolveScanTarget_RejectsBracketedIPv6HostPortInTarget(t *testing.T) {
	_, err := resolveScanTarget([]string{"alice@[::1]:2222"})
	if err == nil {
		t.Fatal("expected error for bracketed host:port target")
	}
	if !strings.Contains(err.Error(), "--ssh-port") {
		t.Fatalf("expected ssh-port hint, got: %v", err)
	}
}
