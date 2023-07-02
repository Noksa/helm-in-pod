package helmtar

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Compress(src string, destPath string, buf io.Writer) error {
	// tar > gzip > buf
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	isDir := false
	if stat.IsDir() {
		isDir = true
	}
	// walk through every file in the path
	_ = filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		// generate tar header
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}
		dir := file
		trim := strings.TrimSuffix(src, "/")
		filename := ""
		if !fi.IsDir() {
			dir = filepath.Dir(file)
			trim = filepath.Dir(trim)
			filename = fi.Name()
		}
		dir = strings.ReplaceAll(dir, trim, destPath)
		if !fi.IsDir() && !strings.Contains(dir, filename) && isDir {
			dir = fmt.Sprintf("%v/%v", dir, fi.Name())
		}
		header.Name = filepath.ToSlash(dir)

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})

	// produce tar
	if err := tw.Close(); err != nil {
		return err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return err
	}
	//
	return nil
}
