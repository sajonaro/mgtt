package providersupport

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerCmd is the small surface mgtt needs to install providers from
// Docker images. Tests inject Run; production uses the real `docker` CLI.
type DockerCmd struct {
	Run     func(ctx context.Context, args ...string) ([]byte, error)
	Timeout time.Duration
}

// NewDockerCmd returns a DockerCmd that shells out to the host `docker`.
func NewDockerCmd() *DockerCmd {
	return &DockerCmd{
		Timeout: 60 * time.Second,
		Run: func(ctx context.Context, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		},
	}
}

// ValidateImageRef rejects refs that aren't pinned by digest. Mgtt's whole
// point of supporting image install is reproducibility — bare tags can be
// re-rolled (the lesson from grafana/tempo:2.6.0). The check is structural,
// not semantic: the ref must contain "@sha256:".
func ValidateImageRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("image ref is required")
	}
	if !strings.Contains(ref, "@sha256:") {
		return fmt.Errorf("image ref must include @sha256: digest for reproducibility (got %q)", ref)
	}
	return nil
}

// PullImage runs `docker pull <ref>`. Returns the docker output on error so
// callers can show the user what went wrong.
func (d *DockerCmd) PullImage(ctx context.Context, ref string) error {
	cctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	out, err := d.Run(cctx, "pull", ref)
	if err != nil {
		return fmt.Errorf("docker pull %s: %w\n%s", ref, err, out)
	}
	return nil
}

// ExtractManifest runs `docker run --rm <ref> cat /provider.yaml` and returns
// the file contents. The provider image MUST embed its provider.yaml at
// /provider.yaml — no other location is supported.
func (d *DockerCmd) ExtractManifest(ctx context.Context, ref string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	out, err := d.Run(cctx, "run", "--rm", ref, "cat", "/provider.yaml")
	if err != nil {
		return nil, fmt.Errorf("extract /provider.yaml from %s: %w", ref, err)
	}
	return out, nil
}
