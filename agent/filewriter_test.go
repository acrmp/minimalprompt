package agent_test

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/acrmp/minimalprompt/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("FileWriter", func() {
	var (
		dir       string
		fw        agent.FileWriter
		logger    *slog.Logger
		logOutput *gbytes.Buffer
	)

	BeforeEach(func() {
		var err error

		logOutput = gbytes.NewBuffer()
		logger = slog.New(slog.NewTextHandler(logOutput, nil))

		dir, err = os.MkdirTemp("", "fw")
		Expect(err).ToNot(HaveOccurred())
		fw = agent.NewSimpleFileWriter(logger, dir)
	})

	AfterEach(func() {
		err := os.RemoveAll(dir)
		Expect(err).ToNot(HaveOccurred())
	})

	It("writes the provided content to the specified path", func() {
		err := fw.WriteFile("filename", "some content")
		Expect(err).ToNot(HaveOccurred())

		b, err := os.ReadFile(filepath.Join(dir, "filename"))
		Expect(err).ToNot(HaveOccurred())
		Expect(string(b)).To(Equal("some content"))
	})

	It("logs that it is performing the write", func() {
		err := fw.WriteFile("filename", "some content")
		Expect(err).ToNot(HaveOccurred())
		Expect(logOutput).To(gbytes.Say(`writing file.*filename`))
	})

	Context("when the provided path includes a directory", func() {
		It("makes any directories necessary", func() {
			err := fw.WriteFile("file/name/in/deeply/nested/path", "some content")
			Expect(err).ToNot(HaveOccurred())

			b, err := os.ReadFile(filepath.Join(dir, "file/name/in/deeply/nested/path"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(b)).To(Equal("some content"))
		})
		Context("when the parent directories cannot be created", func() {
			BeforeEach(func() {
				err := os.MkdirAll(filepath.Join(dir, "file/name/in/deeply"), 0700)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(filepath.Join(dir, "file/name/in/deeply/nested"), []byte{}, 0600)
				Expect(err).ToNot(HaveOccurred())
			})
			It("errors", func() {
				err := fw.WriteFile("file/name/in/deeply/nested/path", "some content")
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when the file cannot be created", func() {
			BeforeEach(func() {
				err := os.MkdirAll(filepath.Join(dir, "file/name/in/deeply/nested/path"), 0700)
				Expect(err).ToNot(HaveOccurred())
			})
			It("errors", func() {
				err := fw.WriteFile("file/name/in/deeply/nested/path", "some content")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the path is not a local path", func() {
			It("errors", func() {
				err := fw.WriteFile("../../traversal", "some content")
				Expect(err).To(MatchError(`path is not a local path: "../../traversal"`))
			})
		})
	})
})
