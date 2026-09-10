// Package archive extracts tar.gz and zip archives into a directory,
// optionally restricted to one subpath, with path traversal protection.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrSubpathMissing is returned when the requested subpath is not in the archive.
var ErrSubpathMissing = errors.New("subpath not found in archive")

// Result describes an extraction.
type Result struct {
	// TopDir is the single top-level directory of the archive ("" when the
	// archive has none or several).
	TopDir string
	Files  int
}

// Options control extraction.
type Options struct {
	// StripTop drops the single top-level directory (GitHub tarballs have one).
	StripTop bool
	// Subpath, when set, extracts only entries under it (after StripTop) and
	// writes them relative to dst.
	Subpath string
	// MaxBytes caps the total extracted size (0 = 512 MiB).
	MaxBytes int64
}

// ExtractTarGz extracts a gzip tarball from r into dst.
func ExtractTarGz(r io.Reader, dst string, opts Options) (Result, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return Result{}, fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	w := newWriter(dst, opts)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return w.result(), fmt.Errorf("read tar: %w", err)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := w.dir(hdr.Name); err != nil {
				return w.result(), err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := w.file(hdr.Name, tr, hdr.FileInfo().Mode().Perm()); err != nil {
				return w.result(), err
			}
		case tar.TypeSymlink:
			if err := w.symlink(hdr.Name, hdr.Linkname); err != nil {
				return w.result(), err
			}
		}
	}
	return w.finish()
}

// ExtractZip extracts a zip archive (read fully into memory) into dst.
func ExtractZip(r io.Reader, dst string, opts Options) (Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Result{}, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, fmt.Errorf("read zip: %w", err)
	}
	w := newWriter(dst, opts)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			if err := w.dir(f.Name); err != nil {
				return w.result(), err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return w.result(), err
		}
		err = w.file(f.Name, rc, f.Mode().Perm())
		rc.Close()
		if err != nil {
			return w.result(), err
		}
	}
	return w.finish()
}

type writer struct {
	dst     string
	opts    Options
	tops    map[string]bool
	files   int
	written int64
	matched bool
}

func newWriter(dst string, opts Options) *writer {
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 512 << 20
	}
	return &writer{dst: dst, opts: opts, tops: map[string]bool{}}
}

// target maps an archive entry name to a destination path, or "" to skip.
func (w *writer) target(name string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(filepath.ToSlash(name), "./"))
	if clean == "." || clean == "" || path.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", nil
	}
	parts := strings.Split(clean, "/")
	w.tops[parts[0]] = true
	if w.opts.StripTop {
		parts = parts[1:]
	}
	rel := strings.Join(parts, "/")
	if sub := strings.Trim(path.Clean(w.opts.Subpath), "/"); sub != "" && sub != "." {
		if rel == sub {
			w.matched = true
			return w.dst, nil
		}
		if !strings.HasPrefix(rel, sub+"/") {
			return "", nil
		}
		w.matched = true
		rel = strings.TrimPrefix(rel, sub+"/")
	}
	if rel == "" {
		return w.dst, nil
	}
	out := filepath.Join(w.dst, filepath.FromSlash(rel))
	if !strings.HasPrefix(out, filepath.Clean(w.dst)+string(filepath.Separator)) && out != filepath.Clean(w.dst) {
		return "", fmt.Errorf("archive entry %q escapes destination", name)
	}
	return out, nil
}

func (w *writer) dir(name string) error {
	out, err := w.target(name)
	if err != nil || out == "" {
		return err
	}
	return os.MkdirAll(out, 0o755)
}

func (w *writer) file(name string, r io.Reader, perm os.FileMode) error {
	out, err := w.target(name)
	if err != nil || out == "" {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if perm&0o400 == 0 {
		perm = 0o644
	}
	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm|0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, w.opts.MaxBytes-w.written+1))
	f.Close()
	if err != nil {
		return err
	}
	w.written += n
	if w.written > w.opts.MaxBytes {
		return fmt.Errorf("archive exceeds %d bytes", w.opts.MaxBytes)
	}
	w.files++
	return nil
}

func (w *writer) symlink(name, link string) error {
	out, err := w.target(name)
	if err != nil || out == "" {
		return err
	}
	if path.IsAbs(link) || strings.HasPrefix(path.Clean(link), "../") {
		return nil // never write links that leave the tree
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.Symlink(link, out)
}

func (w *writer) result() Result {
	res := Result{Files: w.files}
	if len(w.tops) == 1 {
		for k := range w.tops {
			res.TopDir = k
		}
	}
	return res
}

func (w *writer) finish() (Result, error) {
	res := w.result()
	if sub := strings.Trim(path.Clean(w.opts.Subpath), "/"); sub != "" && sub != "." && !w.matched {
		return res, fmt.Errorf("%w: %s", ErrSubpathMissing, sub)
	}
	return res, nil
}
