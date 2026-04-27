package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	runDockerSmokeEnv = "YJSBRIDGE_RUN_DOCKER_SMOKE"
	postgresImage     = "postgres:16-alpine"
)

var smokeSchemaSeq uint64

type dockerPostgres struct {
	containerID string
	dsn         string
}

func requireDockerSmoke(t *testing.T) {
	t.Helper()

	if os.Getenv(runDockerSmokeEnv) == "" {
		t.Skipf("smoke docker ignorado: defina %s=1", runDockerSmokeEnv)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker indisponivel: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		t.Skipf("docker nao esta funcional: %v", err)
	}
}

func startDockerPostgres(t *testing.T) *dockerPostgres {
	t.Helper()
	requireDockerSmoke(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"docker", "run", "--rm", "-d",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_DB=yjsbridge",
		"-p", "127.0.0.1::5432",
		postgresImage,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker run postgres failed: %v\n%s", err, string(out))
	}

	containerID := strings.TrimSpace(string(out))
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		_ = exec.CommandContext(stopCtx, "docker", "rm", "-f", containerID).Run()
	})

	port := waitDockerMappedPort(t, containerID)
	dsn := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/yjsbridge?sslmode=disable", port)
	waitPostgresReady(t, dsn)

	return &dockerPostgres{
		containerID: containerID,
		dsn:         dsn,
	}
}

func waitDockerMappedPort(t *testing.T, containerID string) string {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := exec.CommandContext(
			ctx,
			"docker", "inspect", "-f",
			`{{(index (index .NetworkSettings.Ports "5432/tcp") 0).HostPort}}`,
			containerID,
		).CombinedOutput()
		cancel()
		if err == nil {
			port := strings.TrimSpace(string(out))
			if port != "" && port != "<no value>" {
				return port
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("docker nao publicou porta postgres para container %s", containerID)
	return ""
}

func waitPostgresReady(t *testing.T, dsn string) {
	t.Helper()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			_ = conn.Close(ctx)
			cancel()
			return
		}
		cancel()
		time.Sleep(300 * time.Millisecond)
	}

	t.Fatalf("postgres nao ficou pronto a tempo: %s", dsn)
}

func newSmokeSchema(prefix string) string {
	seq := atomic.AddUint64(&smokeSchemaSeq, 1)
	sanitized := strings.ToLower(prefix)
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	return fmt.Sprintf("yjs_bridge_smoke_%s_%d", sanitized, seq)
}
