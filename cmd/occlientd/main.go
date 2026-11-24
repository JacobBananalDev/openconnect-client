package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/JacobBananalDev/openconnect-client/internal/daemon"
	"github.com/JacobBananalDev/openconnect-client/internal/ipc"
	"github.com/JacobBananalDev/openconnect-client/internal/vpn"
)

type trustedRunner struct{ runner *vpn.RealProcessRunner }

func (r trustedRunner) StartCommand(ctx context.Context, cmd string, args []string) (vpn.Process, error) {
	if err := daemon.ValidateExecutableSearchPath(os.Getenv("PATH")); err != nil {
		return nil, err
	}
	if err := daemon.ValidateTrustedFile(r.runner.ExecutablePath, true); err != nil {
		return nil, fmt.Errorf("untrusted OpenConnect executable: %w", err)
	}
	return r.runner.StartCommand(ctx, cmd, args)
}

func authorizePeer(peer ipc.PeerCredentials, gid uint32) error {
	if peer.UID == 0 || peer.GID == gid {
		return nil
	}
	for _, group := range peer.Groups {
		if group == gid {
			return nil
		}
	}
	return fmt.Errorf("peer is not in the authorized group")
}

func run() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("occlientd requires root")
	}
	syscall.Umask(0o077)
	group, err := user.LookupGroup("openconnect-client")
	if err != nil {
		return fmt.Errorf("lookup authorized group: %w", err)
	}
	gid, err := strconv.ParseUint(group.Gid, 10, 32)
	if err != nil || gid == 0 || uint32(os.Getegid()) != uint32(gid) {
		return fmt.Errorf("daemon effective group must be the non-root openconnect-client group")
	}
	if err := daemon.ValidateRuntimeDirectory(filepath.Dir(ipc.DefaultSocketPath), uint32(gid)); err != nil {
		return fmt.Errorf("inspect runtime directory: %w", err)
	}
	if err := daemon.ValidateExecutableSearchPath(os.Getenv("PATH")); err != nil {
		return err
	}
	runner, err := vpn.NewRealProcessRunner()
	if err != nil {
		return err
	}
	if err := daemon.ValidateTrustedFile(runner.ExecutablePath, true); err != nil {
		return fmt.Errorf("untrusted OpenConnect executable: %w", err)
	}
	policy, err := daemon.LoadPolicy(daemon.PolicyPath)
	if err != nil {
		return fmt.Errorf("load connection policy: %w", err)
	}
	if policy == nil {
		log.Printf("occlientd: connection policy is absent; connect requests are denied")
	}
	authorize := ipc.AuthorizerFunc(func(_ context.Context, peer ipc.PeerCredentials) error {
		return authorizePeer(peer, uint32(gid))
	})
	service, err := daemon.NewService(ipc.DefaultSocketPath, trustedRunner{runner}, authorize, policy.Allows)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return daemon.Run(ctx, service)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "occlientd: %v\n", err)
		os.Exit(1)
	}
}
