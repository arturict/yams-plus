package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/arturict/yams-plus/internal/layout"
)

func Create(paths layout.Layout, destination, passphrase string, includeSecrets bool) error {
	if strings.TrimSpace(passphrase) == "" {
		return fmt.Errorf("backup passphrase is required")
	}
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cleanup := func() { _ = file.Close(); _ = os.Remove(destination) }
	writer, err := age.Encrypt(file, recipient)
	if err != nil {
		cleanup()
		return err
	}
	gz := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gz)
	roots := []string{paths.ConfigDir(), paths.StateDir(), paths.InstallDir()}
	for _, root := range roots {
		if err := addTree(tarWriter, paths.Root, root, includeSecrets, paths.SecretsDir()); err != nil {
			_ = tarWriter.Close()
			_ = gz.Close()
			_ = writer.Close()
			cleanup()
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		cleanup()
		return err
	}
	if err := gz.Close(); err != nil {
		cleanup()
		return err
	}
	if err := writer.Close(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nil
}

func Restore(paths layout.Layout, source, passphrase string) error {
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return err
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := age.Decrypt(file, identity)
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gz.Close()
	tarReader := tar.NewReader(gz)
	root, err := filepath.Abs(paths.Root)
	if err != nil {
		return err
	}
	// Each managed tree is opened as an os.Root. Every write below resolves its
	// path components with openat and is refused if any of them leaves the
	// tree, so a symlink a container planted under the state directory cannot
	// redirect a root-owned write. O_NOFOLLOW alone guarded only the last
	// component, and os.MkdirAll followed the rest.
	roots := map[string]*os.Root{}
	defer func() {
		for _, opened := range roots {
			_ = opened.Close()
		}
	}()
	openManaged := func(dir string) (*os.Root, error) {
		if opened, ok := roots[dir]; ok {
			return opened, nil
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
		opened, err := os.OpenRoot(dir)
		if err != nil {
			return nil, err
		}
		roots[dir] = opened
		return opened, nil
	}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		managed, rel, err := safeTarget(paths, root, header.Name)
		if err != nil {
			return err
		}
		tree, err := openManaged(managed)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := tree.MkdirAll(rel, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("restore %s: %w", header.Name, err)
			}
		case tar.TypeReg:
			if err := tree.MkdirAll(filepath.Dir(rel), 0o750); err != nil {
				return fmt.Errorf("restore %s: %w", header.Name, err)
			}
			// os.Root would still follow a final-component symlink that stays
			// inside the tree; O_NOFOLLOW refuses that too, so an entry is
			// always written to the path the archive names.
			out, err := tree.OpenFile(rel, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|openNoFollow, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("restore %s: %w", header.Name, err)
			}
			if _, err := io.Copy(out, tarReader); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
	return nil
}

// safeTarget confines an archive entry to one of the three directories Create
// archives and returns that directory with the entry's path relative to it.
// Confining to the install root alone is not enough: the production root is
// "/", under which every absolute path qualifies, so a tampered archive could
// write /root/.ssh/authorized_keys or /etc/cron.d as root. This check is
// lexical; symlinks on disk are handled by opening the returned directory as
// an os.Root.
func safeTarget(paths layout.Layout, root, name string) (string, string, error) {
	resolved, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", "", fmt.Errorf("unsafe backup path %q", name)
	}
	for _, managed := range []string{paths.ConfigDir(), paths.StateDir(), paths.InstallDir()} {
		abs, err := filepath.Abs(managed)
		if err != nil {
			continue
		}
		if resolved == abs || strings.HasPrefix(resolved, abs+string(filepath.Separator)) {
			rel, err := filepath.Rel(abs, resolved)
			if err != nil {
				return "", "", fmt.Errorf("unsafe backup path %q", name)
			}
			return abs, rel, nil
		}
	}
	return "", "", fmt.Errorf("unsafe backup path %q", name)
}

func addTree(writer *tar.Writer, archiveRoot, source string, includeSecrets bool, secretsDir string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !includeSecrets && (path == secretsDir || strings.HasPrefix(path, secretsDir+string(filepath.Separator))) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			return nil
		}
		name, err := filepath.Rel(archiveRoot, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(name)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(writer, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		}
		return nil
	})
}
