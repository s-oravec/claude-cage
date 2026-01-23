package network

import (
	"testing"
)

func TestForwarder(t *testing.T) {
	t.Run("StartForwarding with empty forwards", func(t *testing.T) {
		_, err := StartForwarding("test", "192.168.100.2", nil)
		if err != ErrNoPortsToForward {
			t.Errorf("expected ErrNoPortsToForward, got %v", err)
		}
	})

	t.Run("StartForwarding with empty IP", func(t *testing.T) {
		forwards := []PortForward{{HostPort: 8080, GuestPort: 80, Protocol: "tcp", Bind: "127.0.0.1"}}
		_, err := StartForwarding("test", "", forwards)
		if err == nil {
			t.Error("expected error for empty IP")
		}
	})

	t.Run("StopForwarderByPID with invalid PID", func(t *testing.T) {
		// Should not error for invalid PID
		err := StopForwarderByPID(0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		err = StopForwarderByPID(-1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("IsForwarderRunning with invalid PID", func(t *testing.T) {
		if IsForwarderRunning(0) {
			t.Error("expected false for PID 0")
		}
		if IsForwarderRunning(-1) {
			t.Error("expected false for PID -1")
		}
	})
}

func TestForwarderStop(t *testing.T) {
	t.Run("Stop with nil process", func(t *testing.T) {
		f := &Forwarder{
			CageName: "test",
			Process:  nil,
		}
		err := f.Stop()
		if err != ErrForwarderNotRunning {
			t.Errorf("expected ErrForwarderNotRunning, got %v", err)
		}
	})
}
