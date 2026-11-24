package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestRunShutsDownAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})

	runtime := &testRuntime{
		serve: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return nil
		},
		shutdown: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("shutdown context has no deadline")
			}
			return nil
		},
	}

	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(ctx, runtime)
	}()

	<-started
	cancel()

	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !runtime.shutdownCalled {
		t.Fatal("Shutdown() was not called")
	}
}

func TestRunShutsDownAfterServeReturns(t *testing.T) {
	serveErr := errors.New("serve failed")
	runtime := &testRuntime{
		serve: func(context.Context) error {
			return serveErr
		},
		shutdown: func(context.Context) error {
			return nil
		},
	}

	err := Run(context.Background(), runtime)
	if !errors.Is(err, serveErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, serveErr)
	}
	if !runtime.shutdownCalled {
		t.Fatal("Shutdown() was not called after Serve() returned")
	}
}

func TestRunReturnsShutdownError(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")
	runtime := &testRuntime{
		serve: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		shutdown: func(context.Context) error {
			return shutdownErr
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, runtime)
	if !errors.Is(err, shutdownErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, shutdownErr)
	}
}

func TestRunRequiresContextAndRuntime(t *testing.T) {
	if err := Run(nil, &testRuntime{}); err == nil {
		t.Fatal("Run(nil, runtime) error = nil, want error")
	}
	if err := Run(context.Background(), nil); err == nil {
		t.Fatal("Run(ctx, nil) error = nil, want error")
	}
}

type testRuntime struct {
	serve          func(context.Context) error
	shutdown       func(context.Context) error
	shutdownCalled bool
}

func (r *testRuntime) Serve(ctx context.Context) error {
	if r.serve == nil {
		<-ctx.Done()
		return nil
	}
	return r.serve(ctx)
}

func (r *testRuntime) Shutdown(ctx context.Context) error {
	r.shutdownCalled = true
	if r.shutdown == nil {
		return nil
	}
	return r.shutdown(ctx)
}
