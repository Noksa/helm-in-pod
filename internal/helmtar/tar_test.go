package helmtar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// extractTarGz reads a tar.gz buffer and returns a map of path -> content.
func extractTarGz(buf *bytes.Buffer) (map[string]string, error) {
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	files := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeDir {
			files[hdr.Name] = ""
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[hdr.Name] = string(data)
	}
	return files, nil
}

var _ = Describe("Compress", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "helmtar-test-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(tmpDir) })
	})

	Context("single file", func() {
		It("should compress a single file into tar.gz", func() {
			srcFile := filepath.Join(tmpDir, "hello.txt")
			Expect(os.WriteFile(srcFile, []byte("hello world"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(Compress(srcFile, "/dest/hello.txt", &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/hello.txt", "hello world"))
		})

		It("should handle an empty file", func() {
			srcFile := filepath.Join(tmpDir, "empty.txt")
			Expect(os.WriteFile(srcFile, []byte{}, 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(Compress(srcFile, "/dest/empty.txt", &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/empty.txt", ""))
		})
	})

	Context("directory", func() {
		It("should compress a directory with files", func() {
			srcDir := filepath.Join(tmpDir, "mydir")
			Expect(os.MkdirAll(srcDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("bbb"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(Compress(srcDir, "/dest", &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/a.txt", "aaa"))
			Expect(files).To(HaveKeyWithValue("/dest/b.txt", "bbb"))
		})

		It("should compress nested subdirectories", func() {
			srcDir := filepath.Join(tmpDir, "nested")
			subDir := filepath.Join(srcDir, "sub")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(subDir, "child.txt"), []byte("child"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(Compress(srcDir, "/dest", &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/root.txt", "root"))
			Expect(files).To(HaveKeyWithValue("/dest/sub/child.txt", "child"))
		})
	})

	Context("error cases", func() {
		It("should return error for non-existent source", func() {
			var buf bytes.Buffer
			err := Compress("/nonexistent/path", "/dest", &buf)
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("CompressMulti", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "helmtar-multi-test-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(tmpDir) })
	})

	Context("empty entries", func() {
		It("should produce a valid empty tar when given no entries", func() {
			var buf bytes.Buffer
			Expect(CompressMulti(nil, &buf)).To(Succeed())
			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
		})
	})

	Context("single file entry", func() {
		It("should pack one file to the specified destination", func() {
			src := filepath.Join(tmpDir, "values.yaml")
			Expect(os.WriteFile(src, []byte("replicaCount: 2"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{SrcPath: src, DestPath: "/tmp/values.yaml"}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/tmp/values.yaml", "replicaCount: 2"))
		})
	})

	Context("multiple entries", func() {
		It("should pack a directory and a separate file into one tar", func() {
			chartDir := filepath.Join(tmpDir, "chart")
			Expect(os.MkdirAll(filepath.Join(chartDir, "templates"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("name: myapp"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "templates", "deploy.yaml"), []byte("kind: Deployment"), 0644)).To(Succeed())

			valuesFile := filepath.Join(tmpDir, "values.yaml")
			Expect(os.WriteFile(valuesFile, []byte("env: prod"), 0644)).To(Succeed())

			entries := []BundleEntry{
				{SrcPath: chartDir, DestPath: "/tmp/chart"},
				{SrcPath: valuesFile, DestPath: "/tmp/values.yaml"},
			}
			var buf bytes.Buffer
			Expect(CompressMulti(entries, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/tmp/chart/Chart.yaml", "name: myapp"))
			Expect(files).To(HaveKeyWithValue("/tmp/chart/templates/deploy.yaml", "kind: Deployment"))
			Expect(files).To(HaveKeyWithValue("/tmp/values.yaml", "env: prod"))
		})

		It("should pack multiple independent files to different destinations", func() {
			f1 := filepath.Join(tmpDir, "a.txt")
			f2 := filepath.Join(tmpDir, "b.txt")
			Expect(os.WriteFile(f1, []byte("aaa"), 0644)).To(Succeed())
			Expect(os.WriteFile(f2, []byte("bbb"), 0644)).To(Succeed())

			entries := []BundleEntry{
				{SrcPath: f1, DestPath: "/dest/a.txt"},
				{SrcPath: f2, DestPath: "/dest/b.txt"},
			}
			var buf bytes.Buffer
			Expect(CompressMulti(entries, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/a.txt", "aaa"))
			Expect(files).To(HaveKeyWithValue("/dest/b.txt", "bbb"))
		})
	})

	Context("error cases", func() {
		It("should return error if a source path does not exist", func() {
			entries := []BundleEntry{
				{SrcPath: "/nonexistent/path", DestPath: "/tmp/x"},
			}
			var buf bytes.Buffer
			Expect(CompressMulti(entries, &buf)).To(HaveOccurred())
		})
	})
})
