package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const shutdownTimeout = 5 * time.Second

// Runtime serves daemon requests and releases its resources during shutdown.
type Runtime interface {
	Serve(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// Run serves until the runtime exits or ctx is canceled, then shuts it down.
func Run(ctx context.Context, runtime Runtime) error {
	if ctx == nil {
		return errors.New("daemon context is required")
	}
	if runtime == nil {
		return errors.New("daemon runtime is required")
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- runtime.Serve(ctx)
	}()

	select {
	case serveErr := <-serveDone:
		return errors.Join(serveErr, shutdown(runtime, nil))
	case <-ctx.Done():
		return shutdown(runtime, serveDone)
	}
}

func shutdown(runtime Runtime, serveDone <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- runtime.Shutdown(ctx)
	}()

	var shutdownErr error
	select {
	case shutdownErr = <-shutdownDone:
	case <-ctx.Done():
		shutdownErr = fmt.Errorf("daemon shutdown exceeded %s: %w", shutdownTimeout, ctx.Err())
	}

	if serveDone == nil {
		return shutdownErr
	}

	select {
	case serveErr := <-serveDone:
		return errors.Join(serveErr, shutdownErr)
	case <-ctx.Done():
		return errors.Join(
			shutdownErr,
			fmt.Errorf("daemon runtime did not stop before the shutdown deadline: %w", ctx.Err()),
		)
	}
}
