package agent_test

import (
	"errors"
	"io"
	"strings"

	"github.com/acrmp/minimalprompt/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Prompter", func() {
	var (
		r  io.Reader
		w  *gbytes.Buffer
		tp *agent.TerminalPrompter
	)

	BeforeEach(func() {
		r = strings.NewReader("A response from the user")
		w = gbytes.NewBuffer()
		tp = agent.NewTerminalPrompter(r, w)
	})

	It("prompts the user", func() {
		p, err := tp.Prompt("some prompt")
		Expect(err).ToNot(HaveOccurred())
		Eventually(w).Should(gbytes.Say("some prompt\n\nreply>"))
		Expect(p).To(Equal("A response from the user"))
	})

	Context("when there is an error reading the prompt", func() {
		BeforeEach(func() {
			r = &erroringReader{}
			tp = agent.NewTerminalPrompter(r, w)
		})
		It("errors", func() {
			_, err := tp.Prompt("some prompt")
			Expect(err).To(MatchError("an error"))
		})
	})
})

type erroringReader struct{}

func (er *erroringReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("an error")
}
