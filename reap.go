package reap

// ErrorCh is an error channel that lets you know when an error was
// encountered while reaping child processes.
type ErrorCh chan error

// PidCh returns the process IDs of reaped child processes.
type PidCh chan int

// ChildStatus reports the PID and raw wait status of a reaped
// child process. On Unix, Status can be converted to
// syscall.WaitStatus for inspection.
type ChildStatus struct {
	Pid    int
	Status uint32
}

// StatusCh receives the PID and wait status of reaped child
// processes.
type StatusCh chan ChildStatus
