package nlp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Sidecar struct {
	cmd *exec.Cmd
}

func StartSidecar(command string, script string, grpcPort string) (*Sidecar, error) {
	cmd := exec.Command(command, script)
	cmd.Env = append(cmd.Environ(), "NLP_GRPC_PORT="+grpcPort)

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Sidecar{cmd: cmd}, nil
}

func (s *Sidecar) Stop(ctx context.Context) error {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = s.cmd.Process.Kill()
		<-done
		return fmt.Errorf("stop nlp sidecar: %w", ctx.Err())
	case err := <-done:
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
	}

	return nil
}

func WaitForReady(ctx context.Context, client Client) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		_, err := client.Health(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
