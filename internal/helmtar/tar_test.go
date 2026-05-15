package helmtar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
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

var _ = Describe("CompressMulti", func() {
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
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcFile,
				DestPath: "/dest/hello.txt",
			}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/hello.txt", "hello world"))
		})

		It("should handle an empty file", func() {
			srcFile := filepath.Join(tmpDir, "empty.txt")
			Expect(os.WriteFile(srcFile, []byte{}, 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcFile,
				DestPath: "/dest/empty.txt",
			}}, &buf)).To(Succeed())

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
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcDir,
				DestPath: "/dest",
			}}, &buf)).To(Succeed())

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
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcDir,
				DestPath: "/dest",
			}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/dest/root.txt", "root"))
			Expect(files).To(HaveKeyWithValue("/dest/sub/child.txt", "child"))
		})

		// Regression test for the fd-leak bug: a defer inside filepath.Walk's callback
		// defers file.Close() until the entire Walk returns, not until the end of the
		// current iteration. For bundles with hundreds of files this exhausts the
		// per-process fd limit (RLIMIT_NOFILE, typically 256 on macOS / 1024 on Linux)
		// and causes EMFILE errors mid-walk. This test uses 512 files — above the macOS
		// default soft limit of 256 — to catch any regression.
		It("does not exhaust file descriptors when compressing large bundles", func() {
			const fileCount = 512
			srcDir := filepath.Join(tmpDir, "large-bundle")
			Expect(os.MkdirAll(srcDir, 0755)).To(Succeed())
			for i := range fileCount {
				name := filepath.Join(srcDir, fmt.Sprintf("file-%04d.yaml", i))
				Expect(os.WriteFile(name, []byte(fmt.Sprintf("index: %d\n", i)), 0644)).To(Succeed())
			}

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcDir,
				DestPath: "/dest",
			}}, &buf)).To(Succeed(), "CompressMulti must not return EMFILE even for large bundles")

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveLen(fileCount + 1)) // files + the root dir entry
		})
	})

	Context("error cases", func() {
		It("should return error for non-existent source", func() {
			var buf bytes.Buffer
			err := CompressMulti([]BundleEntry{{
				SrcPath:  "/nonexistent/path",
				DestPath: "/dest",
			}}, &buf)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("helm chart directory (simulates --copy chartDir:/tmp/chart)", func() {
		It("should place Chart.yaml at destPath/Chart.yaml", func() {
			// Simulate a helm chart directory structure
			chartDir := filepath.Join(tmpDir, "my-chart")
			templatesDir := filepath.Join(chartDir, "templates")
			Expect(os.MkdirAll(templatesDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: my-chart"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("key: value"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte("kind: Deployment"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  chartDir,
				DestPath: "/tmp/chart",
			}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())

			// Chart.yaml must be at /tmp/chart/Chart.yaml (not nested deeper)
			Expect(files).To(HaveKeyWithValue("/tmp/chart/Chart.yaml", "apiVersion: v2\nname: my-chart"))
			Expect(files).To(HaveKeyWithValue("/tmp/chart/values.yaml", "key: value"))
			Expect(files).To(HaveKeyWithValue("/tmp/chart/templates/deployment.yaml", "kind: Deployment"))
		})

		It("should handle chart directory with trailing slash in source", func() {
			chartDir := filepath.Join(tmpDir, "trailing-slash-chart")
			Expect(os.MkdirAll(chartDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2"), 0644)).To(Succeed())

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  chartDir + "/",
				DestPath: "/tmp/chart",
			}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/tmp/chart/Chart.yaml", "apiVersion: v2"))
		})

		It("should handle source path with .. components", func() {
			// Simulate: /path/to/oracle-ansible/../../deployment/_installable/my-chart
			baseDir := filepath.Join(tmpDir, "project", "examples", "oracle-ansible")
			chartDir := filepath.Join(tmpDir, "project", "deployment", "_installable", "my-chart")
			Expect(os.MkdirAll(baseDir, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(chartDir, "templates"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: my-chart"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("key: value"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(chartDir, "templates", "deploy.yaml"), []byte("kind: Deployment"), 0644)).To(Succeed())

			// Use a raw path with ../.. (like ansible passes via shell, NOT filepath.Join which resolves ..)
			srcWithDotDot := baseDir + "/../../deployment/_installable/my-chart"

			var buf bytes.Buffer
			Expect(CompressMulti([]BundleEntry{{
				SrcPath:  srcWithDotDot,
				DestPath: "/tmp/chart",
			}}, &buf)).To(Succeed())

			files, err := extractTarGz(&buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(HaveKeyWithValue("/tmp/chart/Chart.yaml", "apiVersion: v2\nname: my-chart"))
			Expect(files).To(HaveKeyWithValue("/tmp/chart/values.yaml", "key: value"))
			Expect(files).To(HaveKeyWithValue("/tmp/chart/templates/deploy.yaml", "kind: Deployment"))
		})
	})
})
