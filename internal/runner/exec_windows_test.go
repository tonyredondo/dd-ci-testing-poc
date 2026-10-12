package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// createBreakawayFromJob starts a process outside its parent's job.
const createBreakawayFromJob = 0x01000000

// TestExecHelperProcess is the wrapper, the -exec program and the process the
// program leaves running in TestExecWrapperEndsProgramWithItself.
func TestExecHelperProcess(t *testing.T) {
	self := []string{os.Args[0], "-test.run=^TestExecHelperProcess$"}
	switch os.Getenv("DDTO_EXEC_HELPER") {
	case "wrapper":
		os.Setenv("DDTO_EXEC_HELPER", os.Getenv("DDTO_EXEC_PROGRAM"))
		os.Exit(ExecWithCallerEnvironment(self))
	case "command":
		os.Setenv("DDTO_EXEC_HELPER", os.Getenv("DDTO_EXEC_PROGRAM"))
		os.Exit(ExecCommand(self))
	case "breakaway":
		if !breakAway(self) {
			os.Exit(4)
		}
		os.Exit(0)
	case "late-spawn":
		// A worker that the program leaves running breaks away once the
		// test lets it, after the wrapper has exited.
		cmd := exec.Command(self[0], self[1:]...)
		cmd.Env = append(os.Environ(), "DDTO_EXEC_HELPER=late-worker")
		if err := cmd.Start(); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	case "late-worker":
		result := "denied"
		for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			if _, err := os.Stat(os.Getenv("DDTO_EXEC_GO")); err == nil {
				if breakAway(self) {
					result = "ok"
				}
				break
			}
		}
		path := os.Getenv("DDTO_EXEC_RESULT")
		if err := os.WriteFile(path+".tmp", []byte(result), 0600); err != nil {
			os.Exit(3)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	case "sleep":
		path := os.Getenv("DDTO_EXEC_PID")
		if err := os.WriteFile(path+".tmp", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(3)
		}
		if err := os.Rename(path+".tmp", path); err != nil {
			os.Exit(3)
		}
		time.Sleep(time.Minute)
		os.Exit(0)
	case "spawn":
		cmd := exec.Command(self[0], self[1:]...)
		cmd.Env = append(os.Environ(), "DDTO_EXEC_HELPER=sleep")
		if err := cmd.Start(); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
}

// Native go test ends a timed-out -exec program by terminating it. Terminating
// ddto's wrapper must end the program as well, while a process that the
// program leaves running after it exits outlives the wrapper, as it outlives
// native go test.
func TestExecWrapperEndsProgramWithItself(t *testing.T) {
	start := func(t *testing.T, program string) (*exec.Cmd, string) {
		pid := filepath.Join(t.TempDir(), "pid")
		wrapper := exec.Command(os.Args[0], "-test.run=^TestExecHelperProcess$")
		wrapper.Env = append(os.Environ(), "DDTO_EXEC_HELPER=wrapper", "DDTO_EXEC_PROGRAM="+program, "DDTO_EXEC_PID="+pid)
		if err := wrapper.Start(); err != nil {
			t.Fatal(err)
		}
		return wrapper, pid
	}
	started := func(t *testing.T, path string) *os.Process {
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			process, err := os.FindProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = process.Kill() })
			return process
		}
		t.Fatal("exec program did not start")
		return nil
	}
	exits := func(process *os.Process, within time.Duration) bool {
		done := make(chan struct{})
		go func() { _, _ = process.Wait(); close(done) }()
		select {
		case <-done:
			return true
		case <-time.After(within):
			return false
		}
	}
	t.Run("terminated wrapper", func(t *testing.T) {
		wrapper, pid := start(t, "sleep")
		program := started(t, pid)
		if err := wrapper.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = wrapper.Wait()
		if !exits(program, 10*time.Second) {
			t.Fatal("exec program outlived its terminated wrapper")
		}
	})
	t.Run("finished program", func(t *testing.T) {
		wrapper, pid := start(t, "spawn")
		if err := wrapper.Wait(); err != nil {
			t.Fatal(err)
		}
		if exits(started(t, pid), time.Second) {
			t.Fatal("a process left by the exec program ended with the wrapper")
		}
	})
}

// breakAway starts self outside the job of this process, and reports whether
// it could.
func breakAway(self []string) bool {
	cmd := exec.Command(self[0], self[1:]...)
	cmd.Env = append(os.Environ(), "DDTO_EXEC_HELPER=")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createBreakawayFromJob}
	return cmd.Run() == nil
}

// runHelper runs TestExecHelperProcess as helper, with env, and returns its
// exit status.
func runHelper(t *testing.T, helper string, env ...string) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestExecHelperProcess$")
	cmd.Env = append(append(os.Environ(), "DDTO_EXEC_HELPER="+helper), env...)
	var exit *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exit) {
		return exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return 0
}

// A command that ddto passes through, and a process that it leaves running,
// may start processes outside its job, as they may without ddto.
func TestExecCommandAllowsBreakaway(t *testing.T) {
	if !breakAway([]string{os.Args[0], "-test.run=^TestExecHelperProcess$"}) {
		t.Skip("this process cannot start processes outside its job")
	}
	if status := runHelper(t, "command", "DDTO_EXEC_PROGRAM=breakaway"); status != 0 {
		t.Fatalf("a passed-through command could not start a process outside its job: status %d", status)
	}

	dir := t.TempDir()
	start, result := filepath.Join(dir, "go"), filepath.Join(dir, "result")
	if status := runHelper(t, "command", "DDTO_EXEC_PROGRAM=late-spawn", "DDTO_EXEC_GO="+start, "DDTO_EXEC_RESULT="+result); status != 0 {
		t.Fatalf("the program exited with %d", status)
	}
	// The wrapper has released the job, where the worker still runs.
	if err := os.WriteFile(start, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(10 * time.Millisecond) {
		data, err := os.ReadFile(result)
		if err == nil {
			if string(data) != "ok" {
				t.Fatalf("a process left in the released job could not break away: %s", data)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the worker did not report")
		}
	}
}
