package daemon

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"

	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

const (
	// PolicyPath is the administrator-managed connection allowlist.
	PolicyPath     = "/etc/openconnect-client/policy.json"
	maxPolicyBytes = 64 * 1024
)

// Policy contains administrator-approved connection configurations.
type Policy struct {
	connections []vpn.ConnectionConfig
	root        string
	owner       uint32
}

// LoadPolicy loads a root-owned policy. A missing file leaves connections denied.
func LoadPolicy(path string) (*Policy, error) {
	return loadPolicy(path, "/", 0)
}

func loadPolicy(path, root string, owner uint32) (*Policy, error) {
	info, err := validatePath(root, path, owner)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("validate policy file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("policy must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open policy: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPolicyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	if len(data) > maxPolicyBytes {
		return nil, errors.New("policy exceeds size limit")
	}
	var document struct {
		Connections []ipc.ConnectParams `json:"connections"`
	}
	if err := ipc.DecodeStrictJSON(data, &document); err != nil {
		return nil, fmt.Errorf("decode policy: %w", err)
	}
	if document.Connections == nil {
		return nil, errors.New("policy requires a connections array")
	}
	policy := &Policy{root: root, owner: owner}
	for _, entry := range document.Connections {
		config := vpn.ConnectionConfig{ServerURL: entry.Server, Protocol: entry.Protocol, CACert: entry.CACert, Script: entry.Script}
		if err := validateConnection(config); err != nil {
			return nil, fmt.Errorf("invalid policy connection: %w", err)
		}
		for _, path := range []string{config.CACert, config.Script} {
			if path != "" {
				if err := validateFile(root, path, owner, path == config.Script); err != nil {
					return nil, fmt.Errorf("invalid configured file: %w", err)
				}
			}
		}
		policy.connections = append(policy.connections, config)
	}
	return policy, nil
}

// Allows checks for an exact match and revalidates privileged files before use.
func (p *Policy) Allows(config vpn.ConnectionConfig) bool {
	if p == nil {
		return false
	}
	for _, allowed := range p.connections {
		if allowed != config {
			continue
		}
		for _, path := range []string{config.CACert, config.Script} {
			if path != "" {
				if err := validateFile(p.root, path, p.owner, path == config.Script); err != nil {
					log.Printf("occlientd: configured file is no longer trusted: %v", err)
					return false
				}
			}
		}
		return true
	}
	return false
}

// Resolve selects a policy-approved script when the request omits one.
func (p *Policy) Resolve(config vpn.ConnectionConfig) (vpn.ConnectionConfig, bool) {
	if p == nil {
		return vpn.ConnectionConfig{}, false
	}
	if config.Script != "" {
		return config, p.Allows(config)
	}
	var selected vpn.ConnectionConfig
	found := false
	for _, allowed := range p.connections {
		if allowed.ServerURL != config.ServerURL || allowed.Protocol != config.Protocol || allowed.CACert != config.CACert {
			continue
		}
		if found && selected != allowed {
			return vpn.ConnectionConfig{}, false
		}
		selected = allowed
		found = true
	}
	if !found || !p.Allows(selected) {
		return vpn.ConnectionConfig{}, false
	}
	return selected, true
}

func validateConnection(config vpn.ConnectionConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if strings.HasPrefix(config.ServerURL, "-") || strings.ContainsAny(config.ServerURL, "@?#") ||
		strings.IndexFunc(config.ServerURL, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return errors.New("server contains an option, credentials, query, fragment, or whitespace")
	}
	switch config.Protocol {
	case "anyconnect", "nc", "gp", "pulse", "f5", "fortinet", "array":
	default:
		return errors.New("unsupported protocol")
	}
	if config.Script == "" {
		return errors.New("an explicit trusted script is required")
	}
	for _, r := range config.Script {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-", r)) {
			return errors.New("script path contains unsupported characters")
		}
	}
	return nil
}

// ValidateTrustedFile checks the root-owned, non-writable path and file.
func ValidateTrustedFile(path string, executable bool) error {
	return validateFile("/", path, 0, executable)
}

// ValidateExecutableSearchPath requires trusted directories for subprocess helpers.
func ValidateExecutableSearchPath(path string) error {
	return validateSearchPath("/", path, 0)
}

func validateSearchPath(root, path string, owner uint32) error {
	for _, directory := range filepath.SplitList(path) {
		info, err := validatePath(root, directory, owner)
		if err != nil {
			return fmt.Errorf("untrusted executable search path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("executable search path is not a directory: %s", directory)
		}
	}
	if path == "" {
		return errors.New("executable search path is empty")
	}
	return nil
}

func validateFile(root, path string, owner uint32, executable bool) error {
	info, err := validatePath(root, path, owner)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if executable && info.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("%s is not owner-executable", path)
	}
	return nil
}

// ValidateRuntimeDirectory verifies the trusted socket parent and group.
func ValidateRuntimeDirectory(path string, gid uint32) error {
	return validateRuntimeDirectory("/", path, 0, gid)
}

func validateRuntimeDirectory(root, path string, owner, gid uint32) error {
	info, err := validatePath(root, path, owner)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Gid != gid || info.Mode().Perm() != 0o750 {
		return errors.New("runtime directory must be owned by root and the authorized group with mode 0750")
	}
	return nil
}

func validatePath(root, path string, owner uint32) (os.FileInfo, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("trusted path must be absolute and clean")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, "../") {
		return nil, errors.New("path is outside trusted root")
	}
	current := root
	parts := []string{"."}
	if relative != "." {
		parts = append(parts, strings.Split(relative, string(os.PathSeparator))...)
	}
	var info os.FileInfo
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil {
			return nil, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("cannot inspect ownership: %s", current)
		}
		if stat.Uid != owner {
			return nil, fmt.Errorf("unexpected owner on %s", current)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink is not permitted: %s", current)
		}
		if info.Mode().Perm()&0o022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			return nil, fmt.Errorf("unsafe mode %v on %s", info.Mode(), current)
		}
		for _, name := range []string{"system.posix_acl_access", "system.posix_acl_default"} {
			if _, err := syscall.Getxattr(current, name, nil); err == nil {
				return nil, fmt.Errorf("ACL is not permitted: %s", current)
			} else if !errors.Is(err, syscall.ENODATA) && !errors.Is(err, syscall.ENOTSUP) {
				return nil, fmt.Errorf("inspect ACL on %s: %w", current, err)
			}
		}
	}
	return info, nil
}
