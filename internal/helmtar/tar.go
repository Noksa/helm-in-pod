package helmtar

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/noksa/helm-in-pod/internal/logz"
)

// BundleEntry represents a single (src → dest) mapping to include in a multi-entry tar bundle.
type BundleEntry struct {
	SrcPath  string // local path (file or directory)
	DestPath string // absolute path inside the pod
}

// CompressMulti packs multiple (src, dest) pairs into a single gzip-compressed tar stream.
// The stream can be piped to "tar zxf - -C /" inside the pod to extract all files at once.
func CompressMulti(entries []BundleEntry, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	for _, e := range entries {
		if err := addToTar(tw, e.SrcPath, e.DestPath); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return zr.Close()
}

// addToTar walks the source path and adds all files/directories to the tar writer
// with the correct destination path inside the pod.
func addToTar(tw *tar.Writer, src string, destPath string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	isDir := stat.IsDir()

	return filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Compute destination path inside the tar
		dir := file
		trim := strings.TrimSuffix(src, "/")
		filename := ""
		if !fi.IsDir() {
			filename = fi.Name()
		}

		dir = strings.ReplaceAll(dir, trim, destPath)
		if !fi.IsDir() && !strings.Contains(dir, filename) && isDir {
			dir = fmt.Sprintf("%v/%v", dir, fi.Name())
		}

		header.Name = filepath.ToSlash(dir)

		logz.HostPod().Debug().Msgf("%v will be copied to %v",
			color.CyanString(file), color.MagentaString(dir))

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		// Open, copy, and close the file immediately — do NOT use defer here.
		// defer inside a filepath.Walk callback defers until the entire Walk
		// returns, leaving every opened file descriptor alive for the whole
		// traversal. For bundles with hundreds of files this exhausts the
		// per-process fd limit (typically 1024) and causes EMFILE errors.
		data, err := os.Open(file)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, data)
		_ = data.Close()
		return copyErr
	})
}
