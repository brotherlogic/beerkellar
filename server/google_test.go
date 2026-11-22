package server

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/beerkellar/proto"
)

type trackingContext struct {
	context.Context
	valueCalled bool
}

func (c *trackingContext) Value(key any) any {
	c.valueCalled = true
	return c.Context.Value(key)
}

func TestAddGoogleTask_DisabledShortCircuits(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_REDIRECT_URL", "")

	tests := []struct {
		name string
		user *pb.User
	}{
		{
			name: "disabled user",
			user: &pb.User{
				UserId:             100,
				GoogleTasksEnabled: false,
			},
		},
		{
			name: "nil user",
			user: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &trackingContext{Context: context.Background()}
			err := (AddGoogleTask{user: tc.user}).run(ctx)
			if err != nil {
				t.Fatalf("run() returned unexpected error: %v", err)
			}
			if ctx.valueCalled {
				t.Fatalf("expected AddGoogleTask.run to short-circuit before OAuth/Tasks client setup")
			}
		})
	}
}
