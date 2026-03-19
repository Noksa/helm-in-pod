package hippod

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// createTarGz builds an in-memory tar.gz archive from a list of entries.
type tarEntry struct {
	Name    string
	Content string
	IsDir   bool
	Mode    int64
}

func createTarGz(entries []tarEntry) *bytes.Buffer {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		if e.Mode == 0 {
			if e.IsDir {
				e.Mode = 0o755
			} else {
				e.Mode = 0o644
			}
		}
		hdr := &tar.Header{
			Name: e.Name,
			Mode: e.Mode,
			Size: int64(len(e.Content)),
		}
		if e.IsDir {
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
		} else {
			hdr.Typeflag = tar.TypeReg
		}
		Expect(tw.WriteHeader(hdr)).To(Succeed())
		if !e.IsDir {
			_, err := tw.Write([]byte(e.Content))
			Expect(err).NotTo(HaveOccurred())
		}
	}
	Expect(tw.Close()).To(Succeed())
	Expect(gz.Close()).To(Succeed())
	return buf
}

var _ = Describe("extractTarGz", func() {
	var destDir string

	BeforeEach(func() {
		var err error
		destDir, err = os.MkdirTemp("", "extract-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(destDir)
	})

	It("should extract a single file", func() {
		archive := createTarGz([]tarEntry{
			{Name: "hello.txt", Content: "hello world"},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		data, err := os.ReadFile(filepath.Join(destDir, "hello.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("hello world"))
	})

	It("should extract multiple files", func() {
		archive := createTarGz([]tarEntry{
			{Name: "a.txt", Content: "aaa"},
			{Name: "b.txt", Content: "bbb"},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		dataA, err := os.ReadFile(filepath.Join(destDir, "a.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(dataA)).To(Equal("aaa"))

		dataB, err := os.ReadFile(filepath.Join(destDir, "b.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(dataB)).To(Equal("bbb"))
	})

	It("should create directories from archive", func() {
		archive := createTarGz([]tarEntry{
			{Name: "subdir/", IsDir: true},
			{Name: "subdir/file.txt", Content: "nested"},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		info, err := os.Stat(filepath.Join(destDir, "subdir"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		data, err := os.ReadFile(filepath.Join(destDir, "subdir/file.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("nested"))
	})

	It("should create parent directories for files automatically", func() {
		archive := createTarGz([]tarEntry{
			{Name: "deep/nested/file.txt", Content: "deep content"},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		data, err := os.ReadFile(filepath.Join(destDir, "deep/nested/file.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("deep content"))
	})

	It("should reject path traversal attempts", func() {
		archive := createTarGz([]tarEntry{
			{Name: "../../../etc/passwd", Content: "malicious"},
		})
		err := extractTarGz(archive, destDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid tar entry path"))
	})

	It("should preserve file permissions", func() {
		archive := createTarGz([]tarEntry{
			{Name: "script.sh", Content: "#!/bin/sh", Mode: 0o755},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		info, err := os.Stat(filepath.Join(destDir, "script.sh"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))
	})

	It("should return error for invalid gzip data", func() {
		buf := bytes.NewBufferString("not gzip data")
		err := extractTarGz(buf, destDir)
		Expect(err).To(HaveOccurred())
	})

	It("should handle empty archive", func() {
		buf := &bytes.Buffer{}
		gz := gzip.NewWriter(buf)
		tw := tar.NewWriter(gz)
		Expect(tw.Close()).To(Succeed())
		Expect(gz.Close()).To(Succeed())

		Expect(extractTarGz(buf, destDir)).To(Succeed())
	})

	It("should overwrite existing files", func() {
		existingFile := filepath.Join(destDir, "existing.txt")
		Expect(os.WriteFile(existingFile, []byte("old"), 0o644)).To(Succeed())

		archive := createTarGz([]tarEntry{
			{Name: "existing.txt", Content: "new"},
		})
		Expect(extractTarGz(archive, destDir)).To(Succeed())

		data, err := os.ReadFile(existingFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("new"))
	})
})
