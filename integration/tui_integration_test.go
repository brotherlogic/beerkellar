//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beerkellar/proto"
	"github.com/brotherlogic/beerkellar/server"
)

func TestTUIIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spin up mock Untappd HTTP server
	mockHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
		  "meta": {"http_code": 200},
		  "response": {
		    "beer": {
		      "bid": 16630,
		      "beer_name": "Celebration Ale",
		      "beer_abv": 6.8,
		      "brewery": {
		        "brewery_name": "Sierra Nevada Brewing Co."
		      }
		    }
		  }
		}`))
	}))
	defer mockHTTP.Close()

	db := server.NewTestDatabase(ctx)
	ut := server.GetUntappd(mockHTTP.URL, mockHTTP.URL, mockHTTP.URL, "", "", "")
	s := server.NewServer("", "", "", db, ut)
	s.StartBackgroundTasks(ctx)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	serverAddr := lis.Addr().String()

	gs := grpc.NewServer()
	pb.RegisterBeerKellerAdminServer(gs, s)
	pb.RegisterBeerKellerServer(gs, s)
	pb.RegisterBeerKellerGoogleServer(gs, s)

	go func() {
		_ = gs.Serve(lis)
	}()
	defer gs.Stop()

	// Build the beerkellar_cli binary
	tmpDir, err := os.MkdirTemp("", "tui-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "beerkellar_cli")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../beerkellar_cli")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build beerkellar_cli: %v, output: %s", err, string(out))
	}

	t.Run("Connection Error Handling", func(t *testing.T) {
		// Run TUI pointing to an invalid address
		cmd := exec.Command(binPath)
		cmd.Env = append(os.Environ(), "BEERKELLAR_SERVER_ADDR=127.0.0.1:9999", "TUI_TEST_MODE=true")

		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("StdinPipe error: %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("StdoutPipe error: %v", err)
		}

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}

		// Wait for the UI to attempt a render with connection error
		time.Sleep(1 * time.Second)

		// Send exit to gracefully terminate the Bubble Tea app
		_, _ = io.WriteString(stdin, "exit\n")

		outBytes, _ := io.ReadAll(stdout)
		output := string(outBytes)

		// We expect the TUI cellar summary pane to indicate a loading/connection error
		if !strings.Contains(output, "Error loading summary") && !strings.Contains(output, "connection error") {
			t.Errorf("Expected cellar summary to contain connection error message, got: %q", output)
		}

		cmd.Wait()
	})

	t.Run("Successful Command Routing and Input Parsing", func(t *testing.T) {
		// Point the TUI to the in-process local gRPC server
		cmd := exec.Command(binPath)
		cmd.Env = append(os.Environ(), fmt.Sprintf("BEERKELLAR_SERVER_ADDR=%s", serverAddr), "TUI_TEST_MODE=true")

		// Configure token file
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir error: %v", err)
		}
		tokenFile := filepath.Join(homeDir, ".beerkellar")
		tokenData := []byte("code: \"testuser\"\n")
		if err := os.WriteFile(tokenFile, tokenData, 0600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
		defer os.Remove(tokenFile)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("StdinPipe error: %v", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("StdoutPipe error: %v", err)
		}

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start command: %v", err)
		}

		// Let the UI fetch initial cellar summary
		time.Sleep(1 * time.Second)

		// Send commands to add a beer, inspect cellar, and then exit
		commands := []string{"add", "16630", "6", "12", "cellar", "exit"}
		for _, c := range commands {
			_, _ = io.WriteString(stdin, c+"\n")
			// Give a little time for processing and queue execution
			time.Sleep(500 * time.Millisecond)
		}

		outBytes, _ := io.ReadAll(stdout)
		output := string(outBytes)

		// Verify command routing for "cellar" and the wizard input parsing
		if !strings.Contains(output, "Celebration Ale") {
			t.Errorf("Expected cellar output in command readout to list beer Celebration Ale, got: %q", output)
		}

		cmd.Wait()
	})
}
