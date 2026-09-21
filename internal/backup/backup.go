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
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		resolved, err := safeTarget(root, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(resolved, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(resolved), 0o750); err != nil {
				return err
			}
			out, err := os.OpenFile(resolved, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
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

// safeTarget resolves an archive entry below the install root and rejects
// traversal. The production root is "/", where the separator must not be
// appended twice or every legitimate entry is refused.
func safeTarget(root, name string) (string, error) {
	resolved, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return "", fmt.Errorf("unsafe backup path %q", name)
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if resolved == root || !strings.HasPrefix(resolved, prefix) {
		return "", fmt.Errorf("unsafe backup path %q", name)
	}
	return resolved, nil
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
