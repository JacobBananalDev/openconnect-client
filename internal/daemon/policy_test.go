package daemon

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

func writePolicy(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func trustedTestRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPolicyExactAllowlist(t *testing.T) {
	dir := trustedTestRoot(t)
	script := filepath.Join(dir, "script")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := writePolicy(t, dir, `{"connections":[{"server":"vpn.example.com","protocol":"anyconnect","script":"`+script+`"}]}`)
	policy, err := loadPolicy(path, dir, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	allowed := vpn.ConnectionConfig{ServerURL: "vpn.example.com", Protocol: "anyconnect", Script: script}
	if !policy.Allows(allowed) {
		t.Fatal("configured connection was denied")
	}
	for _, changed := range []vpn.ConnectionConfig{
		{ServerURL: "other.example.com", Protocol: "anyconnect", Script: script},
		{ServerURL: allowed.ServerURL, Protocol: "gp", Script: script},
		{ServerURL: allowed.ServerURL, Protocol: allowed.Protocol, Script: "/tmp/script"},
		{ServerURL: allowed.ServerURL, Protocol: allowed.Protocol, Script: script, CACert: "/tmp/ca"},
		{ServerURL: allowed.ServerURL, Protocol: allowed.Protocol},
	} {
		if policy.Allows(changed) {
			t.Errorf("unapproved configuration allowed: %+v", changed)
		}
	}
	var missing *Policy
	if missing.Allows(allowed) {
		t.Fatal("missing policy allowed a connection")
	}
}

func TestPolicyRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"missing array", `{}`},
		{"null array", `{"connections":null}`},
		{"unknown field", `{"connections":[],"unknown":true}`},
		{"duplicate field", `{"connections":[],"CONNECTIONS":[]}`},
		{"duplicate entry field", `{"connections":[{"server":"a","SERVER":"b","protocol":"gp"}]}`},
		{"option server", `{"connections":[{"server":"--script=x","protocol":"gp"}]}`},
		{"credentials", `{"connections":[{"server":"https://user:pass@vpn","protocol":"gp"}]}`},
		{"query", `{"connections":[{"server":"vpn.example.com?token=x","protocol":"gp"}]}`},
		{"whitespace", `{"connections":[{"server":"vpn example","protocol":"gp"}]}`},
		{"option protocol", `{"connections":[{"server":"vpn","protocol":"--script=x"}]}`},
		{"shell script path", `{"connections":[{"server":"vpn","protocol":"gp","script":"/tmp/x;echo"}]}`},
		{"relative certificate", `{"connections":[{"server":"vpn","protocol":"gp","ca_cert":"ca.pem"}]}`},
		{"oversized", `{"connections":[]}` + strings.Repeat(" ", maxPolicyBytes)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := trustedTestRoot(t)
			path := writePolicy(t, dir, tt.json)
			if _, err := loadPolicy(path, dir, uint32(os.Getuid())); err == nil {
				t.Fatal("invalid policy accepted")
			}
		})
	}
}

func TestPolicyMissingAndUnsafeFiles(t *testing.T) {
	dir := trustedTestRoot(t)
	owner := uint32(os.Getuid())
	missing := filepath.Join(dir, "missing.json")
	if policy, err := loadPolicy(missing, dir, owner); err != nil || policy != nil {
		t.Fatalf("missing policy = %v, %v", policy, err)
	}
	path := writePolicy(t, dir, `{"connections":[]}`)
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(path, dir, owner); err == nil {
		t.Fatal("writable policy accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(path, dir, owner+1); err == nil {
		t.Fatal("wrong owner accepted")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(link, dir, owner); err == nil {
		t.Fatal("symlinked policy accepted")
	}
	if err := os.Chmod(dir, 0o770); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPolicy(path, dir, owner); err == nil {
		t.Fatal("writable policy directory accepted")
	}
}

func TestPolicyRechecksConfiguredFiles(t *testing.T) {
	dir := trustedTestRoot(t)
	script := filepath.Join(dir, "script")
	cert := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cert, []byte("certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writePolicy(t, dir, `{"connections":[{"server":"vpn","protocol":"gp","script":"`+script+`","ca_cert":"`+cert+`"}]}`)
	policy, err := loadPolicy(path, dir, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	config := vpn.ConnectionConfig{ServerURL: "vpn", Protocol: "gp", Script: script, CACert: cert}
	if !policy.Allows(config) {
		t.Fatal("trusted configured script was denied")
	}
	if err := os.Chmod(cert, 0o622); err != nil {
		t.Fatal(err)
	}
	if policy.Allows(config) {
		t.Fatal("certificate made group/world writable was allowed")
	}
	if err := os.Chmod(cert, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(script, 0o722); err != nil {
		t.Fatal(err)
	}
	if policy.Allows(config) {
		t.Fatal("script made group/world writable was allowed")
	}
}

func TestRuntimeDirectoryChecks(t *testing.T) {
	root := trustedTestRoot(t)
	dir := filepath.Join(root, "runtime")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	owner, gid := uint32(os.Getuid()), uint32(os.Getgid())
	if err := validateRuntimeDirectory(root, dir, owner, gid); err != nil {
		t.Fatalf("trusted runtime directory: %v", err)
	}
	if err := validateRuntimeDirectory(root, dir, owner, gid+1); err == nil {
		t.Fatal("wrong runtime group accepted")
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeDirectory(root, dir, owner, gid); err == nil {
		t.Fatal("world-accessible runtime directory accepted")
	}
}

func TestExecutableSearchPathChecks(t *testing.T) {
	root := trustedTestRoot(t)
	dir := filepath.Join(root, "bin")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	owner := uint32(os.Getuid())
	if err := validateSearchPath(root, dir, owner); err != nil {
		t.Fatalf("trusted search path: %v", err)
	}
	for _, path := range []string{"", "relative", dir + ":"} {
		if err := validateSearchPath(root, path, owner); err == nil {
			t.Errorf("unsafe search path %q accepted", path)
		}
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := validateSearchPath(root, dir, owner); err == nil {
		t.Fatal("writable search directory accepted")
	}
}

func TestTrustedPathRejectsExtendedACL(t *testing.T) {
	root := trustedTestRoot(t)
	path := writePolicy(t, root, `{"connections":[]}`)
	acl := make([]byte, 4+5*8)
	binary.LittleEndian.PutUint32(acl, 2)
	entries := []struct {
		tag, perm uint16
		id        uint32
	}{
		{1, 6, ^uint32(0)}, {2, 4, uint32(os.Getuid()) + 1},
		{4, 0, ^uint32(0)}, {16, 4, ^uint32(0)}, {32, 0, ^uint32(0)},
	}
	for i, entry := range entries {
		offset := 4 + i*8
		binary.LittleEndian.PutUint16(acl[offset:], entry.tag)
		binary.LittleEndian.PutUint16(acl[offset+2:], entry.perm)
		binary.LittleEndian.PutUint32(acl[offset+4:], entry.id)
	}
	if err := syscall.Setxattr(path, "system.posix_acl_access", acl, 0); err != nil {
		if errors.Is(err, syscall.ENOTSUP) {
			t.Skipf("filesystem does not support POSIX ACLs: %v", err)
		}
		t.Fatal(err)
	}
	if _, err := validatePath(root, path, uint32(os.Getuid())); err == nil {
		t.Fatal("extended ACL accepted")
	}
	if err := syscall.Removexattr(path, "system.posix_acl_access"); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setxattr(root, "system.posix_acl_default", acl, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := validatePath(root, path, uint32(os.Getuid())); err == nil {
		t.Fatal("default ACL accepted")
	}
}
