package integration

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

type integrationTest struct {
	c testcontainers.Container
}

func runTestServer(ctx context.Context) (*integrationTest, error) {
	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.WithDockerfile(
			testcontainers.FromDockerfile{
				Context:    "..",
				Dockerfile: "Dockerfile",
			},
		),
	}
	tc, err := testcontainers.Run(ctx, "", opts...)

	return &integrationTest{
		c: tc,
	}, err
}

func (i *integrationTest) teardownContainer(t *testing.T) {
	testcontainers.CleanupContainer(t, i.c)
}
