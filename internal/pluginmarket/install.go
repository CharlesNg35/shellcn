package pluginmarket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const maxBinaryBytes = 512 << 20

// Install downloads the host-platform binary (mirror URL first), verifies the
// indexed sha256, and atomically places it in dir under the plugin's name.
func (s *Service) Install(ctx context.Context, e Entry, v Version, dir string) (string, error) {
	asset, ok := v.Assets[hostPlatform()]
	if !ok {
		return "", fmt.Errorf("%w: no %s build", plugin.ErrInvalidInput, hostPlatform())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, "."+e.Name+".download-*")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	var errs []error
	for _, url := range asset.URLs {
		if err := s.downloadVerified(ctx, url, asset.SHA256, tmp); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", url, err))
			continue
		}
		errs = nil
		break
	}
	if errs != nil {
		return "", fmt.Errorf("%w: download failed: %v", plugin.ErrUnavailable, errors.Join(errs...))
	}

	if err := tmp.Chmod(0o755); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, e.Name)
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *Service) downloadVerified(ctx context.Context, url, wantSHA string, f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxBinaryBytes)); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA {
		return fmt.Errorf("sha256 mismatch: index %s, downloaded %s", wantSHA, got)
	}
	return nil
}
