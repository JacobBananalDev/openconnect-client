package vpn

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"
)

// FakeRunner implements ProcessRunner for testing.
// It allows injection of success/failure, delays, and other test scenarios.
type FakeRunner struct {
	mu              sync.Mutex
	ShouldFail      bool
	FailureError    error
	StartDelay      time.Duration
	ExitDelay       time.Duration
	StartedCommands []string
	Started         bool
}

// StartCommand starts a fake process.
func (f *FakeRunner) StartCommand(ctx context.Context, cmd string, args []string) (Process, error) {
	f.mu.Lock()
	f.Started = true
	f.StartedCommands = append(f.StartedCommands, cmd)
	shouldFail := f.ShouldFail
	failErr := f.FailureError
	startDelay := f.StartDelay
	exitDelay := f.ExitDelay
	f.mu.Unlock()

	if shouldFail {
		if failErr == nil {
			failErr = errors.New("injected test failure")
		}
		return nil, failErr
	}

	return &FakeProcess{
		startDelay: startDelay,
		exitDelay:  exitDelay,
		exitErr:    nil,
	}, nil
}

// FakeProcess implements Process for testing.
type FakeProcess struct {
	mu         sync.Mutex
	startDelay time.Duration
	exitDelay  time.Duration
	exitErr    error
	started    bool
	killed     bool
	signaled   bool
}

// Wait blocks until the process "exits" after exitDelay.
func (fp *FakeProcess) Wait() error {
	fp.mu.Lock()
	started := fp.started
	fp.mu.Unlock()

	if !started {
		fp.mu.Lock()
		fp.started = true
		fp.mu.Unlock()
		time.Sleep(fp.startDelay)
	}

	time.Sleep(fp.exitDelay)

	fp.mu.Lock()
	defer fp.mu.Unlock()
	return fp.exitErr
}

// Signal records that a signal was sent (for test verification).
func (fp *FakeProcess) Signal(sig os.Signal) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if sig == os.Kill {
		fp.killed = true
	}
	fp.signaled = true
	return nil
}

// PID returns a fake PID for testing.
func (fp *FakeProcess) PID() int {
	return 9999
}

// WasKilled returns whether SIGKILL was sent.
func (fp *FakeProcess) WasKilled() bool {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return fp.killed
}

// WasSignaled returns whether any signal was sent.
func (fp *FakeProcess) WasSignaled() bool {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return fp.signaled
}

// SetExitError sets the error returned by Wait().
func (fp *FakeProcess) SetExitError(err error) {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.exitErr = err
}
