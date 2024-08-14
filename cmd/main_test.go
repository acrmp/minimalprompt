package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("main", func() {

	var (
		dir, sysPath, initPath, outputPath string
	)

	BeforeEach(func() {
		var err error
		dir, err = os.MkdirTemp("", "cmd")
		Expect(err).ToNot(HaveOccurred())

		sysPath = filepath.Join(dir, "system-prompt.txt")
		err = os.WriteFile(sysPath, []byte("you are a green grocer"), 0600)
		Expect(err).ToNot(HaveOccurred())

		initPath = filepath.Join(dir, "initial-prompt.txt")
		err = os.WriteFile(initPath, []byte("you have 10 strawberries for sale"), 0600)
		Expect(err).ToNot(HaveOccurred())

		outputPath = filepath.Join(dir, "project")
		err = os.Mkdir(outputPath, 0700)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(dir)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when called with no arguments", func() {
		It("outputs command help to stderr", func() {
			command := exec.Command(promptCLI)
			command.Env = []string{}
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say(regexp.QuoteMeta("minimalprompt [SYSTEM PROMPT] [INITIAL PROMPT] [OUTPUT DIR]")))
		})
	})
	Context("when called with the system prompt path", func() {
		It("outputs command help to stderr", func() {
			command := exec.Command(promptCLI, sysPath)
			command.Env = []string{}
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say(regexp.QuoteMeta("minimalprompt [SYSTEM PROMPT] [INITIAL PROMPT] [OUTPUT DIR]")))
		})
	})
	Context("when called with the system and initial prompt paths", func() {
		It("outputs command help to stderr", func() {
			command := exec.Command(promptCLI, sysPath, initPath)
			command.Env = []string{}
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say(regexp.QuoteMeta("minimalprompt [SYSTEM PROMPT] [INITIAL PROMPT] [OUTPUT DIR]")))
		})
	})
	Context("when called with the required parameters", func() {
		It("outputs an error about the anthropic api key", func() {
			command := exec.Command(promptCLI, sysPath, initPath, outputPath)
			command.Env = []string{}
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say("ANTHROPIC_API_KEY"))
		})
	})
})
