package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	ComposeFile string
	ProjectDir  string
	Timeout     time.Duration
}

type Container struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

func (c Client) Run(ctx context.Context, args ...string) (string, error) {
	if c.Timeout == 0 {
		c.Timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	base := []string{"compose", "--project-directory", c.ProjectDir, "--file", c.ComposeFile}
	cmd := exec.CommandContext(ctx, "docker", append(base, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (c Client) Validate(ctx context.Context) error {
	_, err := c.Run(ctx, "config", "--quiet")
	return err
}
func (c Client) Up(ctx context.Context) error {
	_, err := c.Run(ctx, "up", "--detach", "--remove-orphans")
	return err
}
func (c Client) Down(ctx context.Context) error { _, err := c.Run(ctx, "down"); return err }

func (c Client) Logs(ctx context.Context, service string) (string, error) {
	return c.Run(ctx, "logs", "--no-color", "--tail", "200", service)
}

func (c Client) PS(ctx context.Context) ([]Container, error) {
	out, err := c.Run(ctx, "ps", "--format", "json")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var list []Container
	if strings.HasPrefix(out, "[") {
		if err := json.Unmarshal([]byte(out), &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	for _, line := range strings.Split(out, "\n") {
		var item Container
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}
