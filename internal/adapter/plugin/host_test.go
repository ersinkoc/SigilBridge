package plugin

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestHostReportsCrash(t *testing.T) {
	command := "sh"
	args := []string{"-c", "exit 1"}
	if runtime.GOOS == "windows" {
		command = "powershell"
		args = []string{"-NoProfile", "-Command", "exit 1"}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	crashed := make(chan Manifest, 1)
	host := NewHost(Manifest{ID: "crashy", Version: "1", Protocol: "grpc", Command: command, Args: args, ProviderIDs: []string{"crashy"}}, func(m Manifest) { crashed <- m })
	if err := host.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case got := <-crashed:
		if got.ID != "crashy" {
			t.Fatalf("manifest = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for crash")
	}
}
