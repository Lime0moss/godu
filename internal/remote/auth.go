package remote

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

var defaultPrivateKeyFiles = []string{
	"id_ed25519",
	"id_ecdsa",
	"id_rsa",
	"id_dsa",
}

func parseSSHTarget(target string) (string, string, error) {
	if strings.TrimSpace(target) == "" {
		return "", "", fmt.Errorf("remote target is required")
	}

	user, host, ok := strings.Cut(target, "@")
	if !ok || user == "" || host == "" {
		return "", "", fmt.Errorf("invalid remote target %q: expected user@host", target)
	}

	return user, host, nil
}

func hostKeyCallback(host string, port int, batchMode bool) (ssh.HostKeyCallback, error) {
	knownHostsPath, err := ensureKnownHostsFile()
	if err != nil {
		return nil, err
	}

	verify, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot load known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := verify(hostname, remote, key); err == nil {
			return nil
		} else {
			var keyErr *knownhosts.KeyError
			if !errors.As(err, &keyErr) {
				return fmt.Errorf("host key verification failed: %w", err)
			}

			address := knownHostAddress(host, port)
			presented := ssh.FingerprintSHA256(key)

			// No keys matched host -> Trust On First Use flow.
			if len(keyErr.Want) == 0 {
				if batchMode {
					return fmt.Errorf("unknown host key for %s (%s); run ssh once to trust it or disable --ssh-batch", address, presented)
				}
				ok, promptErr := promptYesNo(
					fmt.Sprintf(
						"The authenticity of host '%s' can't be established.\n%s key fingerprint is %s.\nTrust this host and continue connecting (yes/no)? ",
						address, key.Type(), presented,
					),
				)
				if promptErr != nil {
					return promptErr
				}
				if !ok {
					return fmt.Errorf("host key for %s was not trusted", address)
				}
				if err := addKnownHost(knownHostsPath, host, port, key); err != nil {
					return err
				}
				return nil
			}

			expected := make([]string, 0, len(keyErr.Want))
			for _, want := range keyErr.Want {
				expected = append(expected, ssh.FingerprintSHA256(want.Key))
			}

			if batchMode {
				return fmt.Errorf(
					"host key mismatch for %s: expected %s, presented %s",
					address,
					strings.Join(expected, ", "),
					presented,
				)
			}

			ok, promptErr := promptYesNo(
				fmt.Sprintf(
					"WARNING: HOST KEY CHANGED for '%s'.\nExpected: %s\nPresented: %s\nReplace stored key and continue (yes/no)? ",
					address,
					strings.Join(expected, ", "),
					presented,
				),
			)
			if promptErr != nil {
				return promptErr
			}
			if !ok {
				return fmt.Errorf("host key mismatch for %s", address)
			}
			if err := replaceKnownHost(knownHostsPath, host, port, key); err != nil {
				return err
			}
			return nil
		}
	}, nil
}

func ensureKnownHostsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory for known_hosts: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create ~/.ssh directory: %w", err)
	}

	path := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return "", fmt.Errorf("cannot create known_hosts: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("cannot access known_hosts: %w", err)
	}

	return path, nil
}

func knownHostAddress(host string, port int) string {
	if port == 22 {
		return host
	}
	return fmt.Sprintf("[%s]:%d", host, port)
}

func knownHostCandidates(host string, port int) map[string]bool {
	candidates := map[string]bool{
		host:                               true,
		fmt.Sprintf("[%s]:%d", host, port): true,
	}
	if port == 22 {
		candidates[fmt.Sprintf("[%s]:22", host)] = true
	}
	return candidates
}

func addKnownHost(path, host string, port int, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cannot update known_hosts: %w", err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{knownHostAddress(host, port)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("cannot write known_hosts entry: %w", err)
	}
	return nil
}

func replaceKnownHost(path, host string, port int, key ssh.PublicKey) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read known_hosts: %w", err)
	}

	updated := removeKnownHostEntries(data, host, port)
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	updated = append(updated, knownhosts.Line([]string{knownHostAddress(host, port)}, key)...)
	updated = append(updated, '\n')

	if err := os.WriteFile(path, updated, 0o600); err != nil {
		return fmt.Errorf("cannot write known_hosts: %w", err)
	}
	return nil
}

func removeKnownHostEntries(data []byte, host string, port int) []byte {
	lines := strings.Split(string(data), "\n")
	keep := make([]string, 0, len(lines))
	candidates := knownHostCandidates(host, port)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			keep = append(keep, line)
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			keep = append(keep, line)
			continue
		}

		hostFieldIdx := 0
		if strings.HasPrefix(fields[0], "@") {
			if len(fields) < 2 {
				keep = append(keep, line)
				continue
			}
			hostFieldIdx = 1
		}

		hostField := fields[hostFieldIdx]
		drop := false
		for _, h := range strings.Split(hostField, ",") {
			if candidates[h] {
				drop = true
				break
			}
		}
		if drop {
			continue
		}
		keep = append(keep, line)
	}

	return []byte(strings.Join(keep, "\n"))
}

func promptYesNo(prompt string) (bool, error) {
	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) {
		return false, fmt.Errorf("cannot prompt for host key trust: stdin is not a terminal")
	}

	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("host key prompt failed: %w", err)
	}

	a := strings.ToLower(strings.TrimSpace(answer))
	return a == "y" || a == "yes", nil
}

func buildAuthMethods(user, host string, batchMode bool) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 4)

	if m := agentAuthMethod(); m != nil {
		methods = append(methods, m)
	}

	signers := loadDefaultKeySigners()
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}

	if !batchMode {
		prompter := &passwordPrompter{user: user, host: host}
		methods = append(methods, ssh.PasswordCallback(prompter.password))
		methods = append(methods, ssh.KeyboardInteractive(prompter.keyboardInteractive))
	}

	if len(methods) == 0 {
		if batchMode {
			return nil, fmt.Errorf("no SSH auth methods available (configure ssh-agent or private keys, or disable --ssh-batch)")
		}
		return nil, fmt.Errorf("no SSH auth methods available")
	}

	return methods, nil
}

func agentAuthMethod() ssh.AuthMethod {
	sock := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if sock == "" {
		return nil
	}

	return ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		conn, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		return agent.NewClient(conn).Signers()
	})
}

func loadDefaultKeySigners() []ssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	signers := make([]ssh.Signer, 0, len(defaultPrivateKeyFiles))
	for _, name := range defaultPrivateKeyFiles {
		path := filepath.Join(home, ".ssh", name)
		pem, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		signer, err := ssh.ParsePrivateKey(pem)
		if err != nil {
			var passphraseErr *ssh.PassphraseMissingError
			if errors.As(err, &passphraseErr) {
				continue
			}
			continue
		}
		signers = append(signers, signer)
	}

	return signers
}

type passwordPrompter struct {
	user string
	host string

	mu      sync.Mutex
	cached  string
	hasPass bool
}

func (p *passwordPrompter) password() (string, error) {
	p.mu.Lock()
	if p.hasPass {
		pass := p.cached
		p.mu.Unlock()
		return pass, nil
	}
	p.mu.Unlock()

	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) {
		return "", fmt.Errorf("cannot prompt for SSH password: stdin is not a terminal")
	}

	fmt.Fprintf(os.Stderr, "%s@%s's password: ", p.user, p.host)
	bytes, err := term.ReadPassword(stdinFD)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("password prompt failed: %w", err)
	}

	pass := string(bytes)
	p.mu.Lock()
	p.cached = pass
	p.hasPass = true
	p.mu.Unlock()
	return pass, nil
}

func (p *passwordPrompter) keyboardInteractive(_ string, _ string, questions []string, echos []bool) ([]string, error) {
	pass, err := p.password()
	if err != nil {
		return nil, err
	}

	answers := make([]string, len(questions))
	for i := range questions {
		if i < len(echos) && echos[i] {
			answers[i] = ""
			continue
		}
		answers[i] = pass
	}
	return answers, nil
}
