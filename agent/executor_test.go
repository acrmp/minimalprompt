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

var _ = Describe("Executor", func() {

	var (
		dir       string
		bash      *agent.BashExecutor
		logger    *slog.Logger
		logOutput *gbytes.Buffer
	)

	BeforeEach(func() {
		var err error
		dir, err = os.MkdirTemp("", "exec")
		Expect(err).ToNot(HaveOccurred())

		logOutput = gbytes.NewBuffer()
		logger = slog.New(slog.NewTextHandler(logOutput, nil))

		bash = agent.NewBashExecutor(logger, dir)
	})

	AfterEach(func() {
		err := os.RemoveAll(dir)
		Expect(err).ToNot(HaveOccurred())
	})

	It("executes the provided command", func() {
		output, err := bash.Execute("printf 'hello world'")
		Expect(err).ToNot(HaveOccurred())
		Expect(output).To(Equal("hello world"))
	})

	It("logs that it is executing the command", func() {
		_, err := bash.Execute("printf 'hello world'")
		Expect(err).ToNot(HaveOccurred())
		Expect(logOutput).To(gbytes.Say(`executing command.*printf 'hello world'`))
	})

	It("includes both stdout and stderr in the captured output", func() {
		output, err := bash.Execute("printf 'hello'\nprintf 'world' >&2\n")
		Expect(err).ToNot(HaveOccurred())
		Expect(output).To(ContainSubstring("hello"))
		Expect(output).To(ContainSubstring("world"))
	})

	It("uses the directory as the working directory", func() {
		_, err := bash.Execute("printf 'hello world' > some-file")
		Expect(err).ToNot(HaveOccurred())

		b, err := os.ReadFile(filepath.Join(dir, "some-file"))
		Expect(err).ToNot(HaveOccurred())

		Expect(string(b)).To(Equal("hello world"))
	})

	Context("when the command exits with a non-zero exit code", func() {
		It("errors", func() {
			o, err := bash.Execute("printf 'still captured'; false")
			Expect(err).To(HaveOccurred())
			Expect(o).To(Equal("still captured"))
		})
	})
})
