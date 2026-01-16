package helmtar

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
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
	isDir := stat.IsDir()
	// walk through every file in the path
	err = filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
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
			filename = fi.Name()
		}
		dir = strings.ReplaceAll(dir, trim, destPath)
		if !fi.IsDir() && !strings.Contains(dir, filename) && isDir {
			dir = fmt.Sprintf("%v/%v", dir, fi.Name())
		}
		header.Name = filepath.ToSlash(dir)
		log.Debugf("%v %v %v will be copied to %v", logz.LogHost(), logz.LogPod(), color.CyanString(file), color.MagentaString(dir))
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
	if err != nil {
		return err
	}

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
