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
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

	t.Run("Remove Beer Flows", func(t *testing.T) {
		subCtx, subCancel := context.WithCancel(context.Background())
		defer subCancel()

		var checkinRequests atomic.Int32
		removeHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "checkin") || r.Method != http.MethodGet {
				checkinRequests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if strings.Contains(r.URL.Path, "4499") {
				_, _ = w.Write([]byte(`{
				  "meta": {"http_code": 200},
				  "response": {
				    "beer": {
				      "bid": 4499,
				      "beer_name": "Pliny the Elder",
				      "beer_abv": 8.0,
				      "brewery": {
				        "brewery_name": "Russian River Brewing Company"
				      }
				    }
				  }
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
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
		defer removeHTTP.Close()

		subDB := server.NewTestDatabase(subCtx)
		subUT := server.GetUntappd(removeHTTP.URL, removeHTTP.URL, removeHTTP.URL, "", "", "")
		subSrv := server.NewServer("", "", "", subDB, subUT)
		subSrv.StartBackgroundTasks(subCtx)

		subLis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to listen: %v", err)
		}
		removeAddr := subLis.Addr().String()

		subGS := grpc.NewServer()
		pb.RegisterBeerKellerAdminServer(subGS, subSrv)
		pb.RegisterBeerKellerServer(subGS, subSrv)
		pb.RegisterBeerKellerGoogleServer(subGS, subSrv)

		go func() {
			_ = subGS.Serve(subLis)
		}()
		defer subGS.Stop()

		subHome := t.TempDir()
		tokenFile := filepath.Join(subHome, ".beerkellar")
		if err := os.WriteFile(tokenFile, []byte("code: \"testuser\"\n"), 0600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		var env []string
		for _, e := range os.Environ() {
			if !strings.HasPrefix(e, "HOME=") {
				env = append(env, e)
			}
		}
		env = append(env,
			fmt.Sprintf("HOME=%s", subHome),
			fmt.Sprintf("BEERKELLAR_SERVER_ADDR=%s", removeAddr),
			"TUI_TEST_MODE=true",
		)

		cmd := exec.Command(binPath)
		cmd.Env = env

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

		// Seed 2 weekday beers (16630, 12 fl oz => 2.41 units each) and 1 weekend beer (4499, 16 fl oz => 3.79 units)
		seedCommands := []string{
			"add", "16630", "2", "12",
			"add", "4499", "1", "16",
		}
		for _, c := range seedCommands {
			if _, err := io.WriteString(stdin, c+"\n"); err != nil {
				t.Fatalf("Failed writing seed command %q: %v", c, err)
			}
		}

		// Wait deterministically for background CacheBeer queue to populate both beers
		deadline := time.Now().Add(5 * time.Second)
		for {
			b1, err1 := subDB.GetBeer(subCtx, 16630)
			b2, err2 := subDB.GetBeer(subCtx, 4499)
			if err1 == nil && b1.GetName() != "" && err2 == nil && b2.GetName() != "" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("Timed out waiting for CacheBeer queue to cache beers 16630 and 4499 (err1=%v, err2=%v)", err1, err2)
			}
			time.Sleep(50 * time.Millisecond)
		}

		flowCommands := []string{
			"remove", "1", "y",
			"remove 2", "y",
			"remove 1", "n",
			"cellar",
			"drunk",
			"exit",
		}
		for _, c := range flowCommands {
			if _, err := io.WriteString(stdin, c+"\n"); err != nil {
				t.Fatalf("Failed writing command %q: %v", c, err)
			}
		}
		_ = stdin.Close()

		outBytes, readErr := io.ReadAll(stdout)
		if readErr != nil {
			t.Fatalf("ReadAll stdout error: %v", readErr)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("beerkellar_cli exited with error: %v\nOutput:\n%s", err, string(outBytes))
		}

		output := string(outBytes)
		rawFrames := strings.Split(output, "██████╗ ███████╗")
		if len(rawFrames) < 2 {
			t.Fatalf("Expected rendered TUI frames in stdout, got:\n%s", output)
		}
		frames := rawFrames[1:]
		if len(frames) != 18 {
			t.Fatalf("Expected 18 TUI frames, got %d\nOutput:\n%s", len(frames), output)
		}

		// Frame 9: guided "remove" numbered list
		for _, want := range []string{
			"Select a beer to remove:",
			"1. Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)",
			"2. Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)",
			"3. Russian River Brewing Company - Pliny the Elder (3.79 units)",
		} {
			if !strings.Contains(frames[9], want) {
				t.Errorf("Frame 9 (remove) missing %q\nFrame:\n%s", want, frames[9])
			}
		}

		// Frame 10: guided item selection confirmation prompt
		if want := `Remove #1 "Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)"? (y/n)`; !strings.Contains(frames[10], want) {
			t.Errorf("Frame 10 (select 1) missing %q\nFrame:\n%s", want, frames[10])
		}

		// Frame 11: guided confirm "y" -> removal readout AND refreshed summary
		for _, want := range []string{
			`Removed "Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)" from cellar.`,
			"Cellar Size & Split: 2 Beers (1 Weekday, 1 Weekend)",
		} {
			if !strings.Contains(frames[11], want) {
				t.Errorf("Frame 11 (confirm y) missing %q\nFrame:\n%s", want, frames[11])
			}
		}

		// Frame 12: direct shortcut "remove 2" confirmation prompt
		if want := `Remove #2 "Russian River Brewing Company - Pliny the Elder (3.79 units)"? (y/n)`; !strings.Contains(frames[12], want) {
			t.Errorf("Frame 12 (remove 2) missing %q\nFrame:\n%s", want, frames[12])
		}

		// Frame 13: shortcut confirm "y" -> removal readout AND refreshed summary
		for _, want := range []string{
			`Removed "Russian River Brewing Company - Pliny the Elder (3.79 units)" from cellar.`,
			"Cellar Size & Split: 1 Beers (1 Weekday, 0 Weekend)",
		} {
			if !strings.Contains(frames[13], want) {
				t.Errorf("Frame 13 (shortcut confirm y) missing %q\nFrame:\n%s", want, frames[13])
			}
		}

		// Frame 14: "remove 1" confirmation prompt before cancellation
		if want := `Remove #1 "Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)"? (y/n)`; !strings.Contains(frames[14], want) {
			t.Errorf("Frame 14 (remove 1) missing %q\nFrame:\n%s", want, frames[14])
		}

		// Frame 15: cancelled removal "n" -> cancellation readout AND summary unchanged
		for _, want := range []string{
			"Removal cancelled. No beer was removed.",
			"Cellar Size & Split: 1 Beers (1 Weekday, 0 Weekend)",
		} {
			if !strings.Contains(frames[15], want) {
				t.Errorf("Frame 15 (cancel n) missing %q\nFrame:\n%s", want, frames[15])
			}
		}

		// Frame 16: "cellar" readout shows only remaining Celebration Ale
		cellarReadout := extractCommandReadout(frames[16])
		if !strings.Contains(cellarReadout, "1. Sierra Nevada Brewing Co. - Celebration Ale (2.41 units)") {
			t.Errorf("Frame 16 (cellar) missing remaining Celebration Ale:\n%s", cellarReadout)
		}
		for _, unwanted := range []string{"Pliny the Elder", "2. "} {
			if strings.Contains(cellarReadout, unwanted) {
				t.Errorf("Frame 16 (cellar) unexpectedly contains %q:\n%s", unwanted, cellarReadout)
			}
		}

		// Frame 17: "drunk" readout remains empty with no errors
		drunkReadout := extractCommandReadout(frames[17])
		if !strings.Contains(drunkReadout, "COMMAND READOUT") {
			t.Errorf("Frame 17 (drunk) missing COMMAND READOUT:\n%s", frames[17])
		}
		for _, unwanted := range []string{"Celebration Ale", "Pliny the Elder", "units", "Error"} {
			if strings.Contains(drunkReadout, unwanted) {
				t.Errorf("Frame 17 (drunk) unexpectedly contains %q:\n%s", unwanted, drunkReadout)
			}
		}

		// Backend state checks: no Untappd check-in or drink history side effects
		if got := checkinRequests.Load(); got != 0 {
			t.Errorf("Expected 0 Untappd checkin requests, got %d", got)
		}
		checkins, err := subDB.GetCheckins(subCtx, 100)
		if err != nil {
			t.Fatalf("GetCheckins error: %v", err)
		}
		if len(checkins) != 0 {
			t.Errorf("Expected 0 checkins in DB, got %d", len(checkins))
		}
		if _, err := subDB.GetDrunk(subCtx, 100); status.Code(err) != codes.NotFound {
			t.Errorf("Expected GetDrunk status NotFound, got err=%v", err)
		}
		cellar, err := subDB.GetCellar(subCtx, 100)
		if err != nil {
			t.Fatalf("GetCellar error: %v", err)
		}
		if len(cellar.GetEntries()) != 1 || cellar.GetEntries()[0].GetBeerId() != 16630 || cellar.GetEntries()[0].GetSizeFlOz() != 12 {
			t.Errorf("Unexpected remaining cellar entries: %+v", cellar.GetEntries())
		}
	})
}

func extractCommandReadout(frame string) string {
	idx := strings.Index(frame, "COMMAND READOUT")
	if idx == -1 {
		return ""
	}
	after := frame[idx:]
	if end := strings.Index(after, "└"); end != -1 {
		return after[:end]
	}
	return after
}
