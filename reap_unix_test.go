//go:build !windows && !solaris

package reap

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReap_IsSupported(t *testing.T) {
	if !IsSupported() {
		t.Fatalf("reap should be supported on %s", runtime.GOOS)
	}
}

func TestReap_ReapChildren(t *testing.T) {
	pids := make(PidCh, 1)
	errors := make(ErrorCh, 1)
	done := make(chan struct{}, 1)
	var reapLock sync.RWMutex

	didExit := make(chan struct{}, 1)
	go func() {
		ReapChildren(pids, errors, done, &reapLock)
		didExit <- struct{}{}
	}()
	time.Sleep(1 * time.Second)

	killAndCheck := func() {
		cmd := exec.Command("sleep", "5")
		if err := cmd.Start(); err != nil {
			t.Fatalf("err: %v", err)
		}

		childPid := cmd.Process.Pid
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("err: %v", err)
		}

		select {
		case pid := <-pids:
			if pid != childPid {
				t.Fatalf("unexpected pid: %d != %d", pid, childPid)
			}
		case err := <-errors:
			t.Fatalf("err: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatalf("should have reaped %d", childPid)
		}
	}

	// Kill a child process and make sure it gets detected.
	killAndCheck()

	// Fire off a subprocess.
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Send a spurious SIGCHLD.
	if err := unix.Kill(os.Getpid(), unix.SIGCHLD); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the reaper didn't report anything.
	select {
	case pid := <-pids:
		t.Fatalf("unexpected pid: %d", pid)
	case err := <-errors:
		t.Fatalf("err: %v", err)
	case <-time.After(1 * time.Second):
		// Good - nothing was sent to the channels.
	}

	// Take the reap lock.
	reapLock.RLock()

	// Now kill the child subprocess.
	childPid := cmd.Process.Pid
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure the reaper didn't report anything.
	select {
	case pid := <-pids:
		t.Fatalf("unexpected pid: %d", pid)
	case err := <-errors:
		t.Fatalf("err: %v", err)
	case <-time.After(1 * time.Second):
		// Good - nothing was sent to the channels.
	}

	// Give up the reap lock.
	reapLock.RUnlock()

	// Make sure the reaper sees it.
	select {
	case pid := <-pids:
		if pid != childPid {
			t.Fatalf("unexpected pid: %d != %d", pid, childPid)
		}
	case err := <-errors:
		t.Fatalf("err: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatalf("should have reaped %d", childPid)
	}

	// Run a few more cycles to make sure things work.
	killAndCheck()
	killAndCheck()
	killAndCheck()

	// Shut it down.
	close(done)
	select {
	case <-didExit:
		// Good - the goroutine shut down.
	case <-time.After(1 * time.Second):
		t.Fatalf("should have shut down")
	}
}

func TestReap_ReapChildrenWithStatus(t *testing.T) {
	statuses := make(StatusCh, 1)
	errors := make(ErrorCh, 1)
	done := make(chan struct{}, 1)

	didExit := make(chan struct{}, 1)
	go func() {
		ReapChildrenWithStatus(statuses, errors, done, nil)
		didExit <- struct{}{}
	}()
	time.Sleep(1 * time.Second)

	// Spawn a child that exits with a known code.
	cmd := exec.Command("/bin/sh", "-c", "exit 42")
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}
	childPid := cmd.Process.Pid

	select {
	case cs := <-statuses:
		if cs.Pid != childPid {
			t.Fatalf("unexpected pid: %d != %d", cs.Pid, childPid)
		}
		ws := syscall.WaitStatus(cs.Status)
		if !ws.Exited() {
			t.Fatalf("expected normal exit, got status %v", ws)
		}
		if ws.ExitStatus() != 42 {
			t.Fatalf("unexpected exit status: %d != 42", ws.ExitStatus())
		}
	case err := <-errors:
		t.Fatalf("err: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("should have reaped %d", childPid)
	}

	// Spawn a child and kill it to verify signal status.
	cmd = exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %v", err)
	}
	childPid = cmd.Process.Pid
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case cs := <-statuses:
		if cs.Pid != childPid {
			t.Fatalf("unexpected pid: %d != %d", cs.Pid, childPid)
		}
		ws := syscall.WaitStatus(cs.Status)
		if !ws.Signaled() {
			t.Fatalf("expected signal termination, got status %v", ws)
		}
		if ws.Signal() != syscall.SIGKILL {
			t.Fatalf("unexpected signal: %v != SIGKILL", ws.Signal())
		}
	case err := <-errors:
		t.Fatalf("err: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("should have reaped %d", childPid)
	}

	// Shut it down.
	close(done)
	select {
	case <-didExit:
		// Good - the goroutine shut down.
	case <-time.After(1 * time.Second):
		t.Fatalf("should have shut down")
	}
}
