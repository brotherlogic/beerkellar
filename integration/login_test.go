package integration

import (
	"context"
	"log"
	"testing"
)

func TestLogin(t *testing.T) {
	ctx := context.Background()
	i, err := runTestServer(ctx)
	if err != nil {
		t.Fatalf("Unable to bring up server: %v", err)
	}

	log.Printf("Running: %v", i)
	defer i.teardownContainer(t)
}
