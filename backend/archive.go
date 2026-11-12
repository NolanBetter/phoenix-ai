package backend

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// extractArchive unpacks a .zip, .tar.gz or .tar.bz2 into dst. Every path is
// checked so a bad archive can't write outside dst.
func extractArchive(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	head := make([]byte, 4)
	_, _ = io.ReadFull(f, head)
	f.Close()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		return extractZip(src, dst)
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		return extractTarGz(src, dst)
	case bytes.HasPrefix(head, []byte("BZh")):
		return extractTarBz2(src, dst)
	default:
		return errors.New("unknown archive format")
	}
}

func extractZip(src, dst string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		target, ok := safeJoin(dst, zf.Name)
		if !ok {
			return fmt.Errorf("unsafe path in archive: %q", zf.Name)
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeFileFrom(target, rc, zf.Mode()|0o600)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractTar(tar.NewReader(gz), dst)
}

func extractTarBz2(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTar(tar.NewReader(bzip2.NewReader(f)), dst)
}

func extractTar(tr *tar.Reader, dst string) error {
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, ok := safeJoin(dst, h.Name)
		if !ok {
			if h.Name == "./" || h.Name == "." {
				continue
			}
			return fmt.Errorf("unsafe path in archive: %q", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFileFrom(target, tr, os.FileMode(h.Mode)|0o600); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Only allow links that stay inside the folder (used for .so version names).
			if filepath.IsAbs(h.Linkname) {
				continue
			}
			if _, ok := safeJoin(filepath.Dir(target), h.Linkname); !ok {
				continue
			}
			if rel, err := filepath.Rel(dst, filepath.Join(filepath.Dir(target), h.Linkname)); err != nil || rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
				continue
			}
			_ = os.MkdirAll(filepath.Dir(target), 0o755)
			_ = os.Symlink(h.Linkname, target)
		}
	}
}

func writeFileFrom(path string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
