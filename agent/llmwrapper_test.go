package agent_test

import (
	"context"
	"errors"
	"log/slog"

	"github.com/acrmp/minimalprompt/agent"
	"github.com/acrmp/minimalprompt/agent/agentfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tmc/langchaingo/llms"
)

var _ = Describe("LLMWrapper", func() {

	var (
		logger    *slog.Logger
		logOutput *gbytes.Buffer
		m         *agentfakes.FakeModel
		e         *agentfakes.FakeCommandExecutor
		w         *agentfakes.FakeFileWriter
		p         *agentfakes.FakePrompter
		a         *agent.LLMWrapper
		ctx       context.Context
		cancel    context.CancelFunc
		errCh     chan error = make(chan error)
	)

	BeforeEach(func() {
		logOutput = gbytes.NewBuffer()
		logger = slog.New(slog.NewTextHandler(logOutput, nil))

		m = &agentfakes.FakeModel{}
		m.GenerateContentReturns(&llms.ContentResponse{}, nil)

		w = &agentfakes.FakeFileWriter{}
		e = &agentfakes.FakeCommandExecutor{}
		p = &agentfakes.FakePrompter{}
		ctx, cancel = context.WithCancel(context.Background())
	})

	JustBeforeEach(func() {
		a = agent.NewLLMWrapper(
			logger,
			"You are a Software Engineer",
			"Please develop a simple calculator",
			m, e, w, p)
		go func() {
			defer GinkgoRecover()
			errCh <- a.Run(ctx)
		}()
	})

	AfterEach(func() {
		cancel()
	})

	It("sets the system prompt and initial prompt for the specified persona", func() {
		Eventually(m.GenerateContentCallCount).Should(Equal(1))

		ctx, msgs, _ := m.GenerateContentArgsForCall(0)
		Expect(ctx).To(Equal(ctx))

		Expect(msgs).To(HaveLen(2))
		Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
		Expect(msgs[0].Parts).To(Equal([]llms.ContentPart{llms.TextPart("You are a Software Engineer")}))
		Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))
		Expect(msgs[1].Parts).To(Equal([]llms.ContentPart{llms.TextPart("Please develop a simple calculator")}))
	})

	Context("when the model describes what it is doing", func() {
		BeforeEach(func() {
			m.GenerateContentReturnsOnCall(0,
				&llms.ContentResponse{
					Choices: []*llms.ContentChoice{
						{
							Content: "Checking wheel alignment",
						},
					},
				},
				nil,
			)
		})
		It("logs the description", func() {
			Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
			ctx, msgs, _ := m.GenerateContentArgsForCall(0)
			Expect(ctx).To(Equal(ctx))
			Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
			Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

			Eventually(logOutput).Should(gbytes.Say(`AI says.*Checking wheel alignment`))
		})
	})

	Context("when the model asks for help", func() {
		BeforeEach(func() {
			m.GenerateContentReturnsOnCall(0, &llms.ContentResponse{
				Choices: []*llms.ContentChoice{{Content: "What do you think?", StopReason: "end_turn"}},
			}, nil)
			p.PromptReturns("Please make it yellow.", nil)
			m.GenerateContentReturnsOnCall(1, &llms.ContentResponse{
				Choices: []*llms.ContentChoice{{Content: "Sure"}},
			}, nil)
		})

		It("prompts the user", func() {
			Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
			Eventually(logOutput).Should(gbytes.Say(`AI says.*What do you think?`))
			Eventually(p.PromptCallCount).Should(Equal(1))

			prompt := p.PromptArgsForCall(0)
			Expect(prompt).To(Equal("What do you think?"))
		})

		It("shares the users answer with the model", func() {
			Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))

			ctx, msgs, _ := m.GenerateContentArgsForCall(0)
			Expect(ctx).To(Equal(ctx))
			Expect(msgs).To(HaveLen(2))
			Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
			Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

			Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 2))
			Eventually(p.PromptCallCount).Should(Equal(1))

			ctx, msgs, _ = m.GenerateContentArgsForCall(1)
			Expect(ctx).To(Equal(ctx))
			Expect(msgs).To(HaveLen(4))
			Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
			Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

			Expect(msgs[2].Role).To(Equal(llms.ChatMessageTypeAI))
			Expect(msgs[2].Parts).To(Equal([]llms.ContentPart{llms.TextPart("What do you think?")}))

			Expect(msgs[3].Role).To(Equal(llms.ChatMessageTypeHuman))
			Expect(msgs[3].Parts).To(Equal([]llms.ContentPart{llms.TextPart("Please make it yellow.")}))
		})
		Context("when prompting errors", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0, &llms.ContentResponse{
					Choices: []*llms.ContentChoice{{Content: "What do you think?", StopReason: "end_turn"}},
				}, nil)
				p.PromptReturns("", errors.New("prompt error"))
			})

			It("errors", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
				Eventually(p.PromptCallCount).Should(Equal(1))
				Eventually(errCh).Should(Receive(MatchError("prompt error")))
			})
		})
	})
	Describe("writing files", func() {
		It("advertises a tool to write to the filesystem", func() {
			Eventually(m.GenerateContentCallCount).Should(Equal(1))
			_, _, opts := m.GenerateContentArgsForCall(0)
			co := &llms.CallOptions{}
			for _, o := range opts {
				o(co)
			}

			var tool llms.Tool
			for _, t := range co.Tools {
				if t.Type == "function" && t.Function.Name == "writeFile" {
					tool = t
					break
				}
			}
			Expect(tool.Type).To(Equal("function"))
			Expect(tool.Function.Name).To(Equal("writeFile"))
			Expect(tool.Function.Description).To(Equal("Write a file to the filesystem"))

			params := tool.Function.Parameters.(map[string]any)
			Expect(params["type"]).To(Equal("object"))
			props := params["properties"].(map[string]any)

			Expect(props).To(HaveKey("content"))
			Expect(props["content"]).To(HaveKeyWithValue("type", "string"))
			Expect(props["content"]).To(HaveKeyWithValue("description", "The content of the file as a string"))

			Expect(props).To(HaveKey("path"))
			Expect(props["path"]).To(HaveKeyWithValue("type", "string"))
			Expect(props["path"]).To(HaveKeyWithValue("description", "The relative path of the file within the project"))

			Expect(params["required"]).To(ConsistOf([]string{"content", "path"}))
		})

		Context("when the model invokes the tool", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "abc123",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"path":"/path/to/some/file","content":"content for the file"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})
			It("writes to the filesystem", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
				ctx, msgs, _ := m.GenerateContentArgsForCall(0)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				Eventually(w.WriteFileCallCount).Should(Equal(1))
				path, content := w.WriteFileArgsForCall(0)
				Expect(path).To(Equal("/path/to/some/file"))
				Expect(content).To(Equal("content for the file"))
			})

			It("does not log empty text output", func() {
				Consistently(logOutput).ShouldNot(gbytes.Say("AI says:"))
			})

			It("shares the command output with the model", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 2))

				ctx, msgs, _ := m.GenerateContentArgsForCall(0)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(2))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				ctx, msgs, _ = m.GenerateContentArgsForCall(1)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(4))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				Expect(msgs[2].Role).To(Equal(llms.ChatMessageTypeAI))
				Expect(msgs[2].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCall{
							ID:   "abc123",
							Type: "function",
							FunctionCall: &llms.FunctionCall{
								Name:      "writeFile",
								Arguments: `{"path":"/path/to/some/file","content":"content for the file"}`,
							},
						},
					},
				))

				Expect(msgs[3].Role).To(Equal(llms.ChatMessageTypeTool))
				Expect(msgs[3].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: "abc123",
							Name:       "writeFile",
							Content:    "ok",
						},
					},
				))
			})
		})

		Context("when the model returns multiple choices and not all invoke the tool", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								Content: "Sure. Let me consider that for a moment.",
							},
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "abc123",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"path":"/path/to/some/file","content":"content for the file"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})
			It("chooses the choice that invokes the tool and writes to the filesystem", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
				Eventually(w.WriteFileCallCount).Should(Equal(1))
				path, content := w.WriteFileArgsForCall(0)
				Expect(path).To(Equal("/path/to/some/file"))
				Expect(content).To(Equal("content for the file"))
			})
		})
		Context("when the model returns multiple choices that invoke the tool", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "abc123",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"path":"/path/to/some/file","content":"content for the file"}`,
										},
									},
								},
							},
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "def234",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"path":"/path/to/other/file","content":"content for the other file"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})
			It("chooses the first choice that invokes the tool and writes to the filesystem", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
				Eventually(w.WriteFileCallCount).Should(Equal(1))
				path, content := w.WriteFileArgsForCall(0)
				Expect(path).To(Equal("/path/to/some/file"))
				Expect(content).To(Equal("content for the file"))
			})
		})
		Context("when the model invokes a tool that doesn't exist", func() {
			BeforeEach(func() {
				m.GenerateContentReturns(
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "accelerate",
											Arguments: `{"warp_factor":5}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})

			It("errors", func() {
				Eventually(errCh).Should(Receive(MatchError(`unrecognised tool call from model: "accelerate"`)))
			})
		})

		Context("when the model tool arguments cannot be parsed", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"not json"`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})
			It("errors", func() {
				Eventually(errCh).Should(Receive(MatchError(MatchRegexp(`could not parse tool call arguments: "writeFile":.*JSON`))))
			})
		})

		Context("when the file cannot be written to", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "writeFile",
											Arguments: `{"path":"/path/to/some/file","content":"content for the file"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
				w.WriteFileReturns(errors.New("some-error"))
			})
			It("errors", func() {
				Eventually(errCh).Should(Receive(MatchError(MatchRegexp(`tool call failed: "writeFile": some-error`))))
			})
		})
	})

	Describe("executing commands", func() {
		It("advertises a tool to execute commands", func() {
			Eventually(m.GenerateContentCallCount).Should(Equal(1))
			_, _, opts := m.GenerateContentArgsForCall(0)
			co := &llms.CallOptions{}
			for _, o := range opts {
				o(co)
			}

			var tool llms.Tool
			for _, t := range co.Tools {
				if t.Type == "function" && t.Function.Name == "executeCommand" {
					tool = t
					break
				}
			}
			Expect(tool.Type).To(Equal("function"))
			Expect(tool.Function.Name).To(Equal("executeCommand"))
			Expect(tool.Function.Description).To(Equal("Execute an operating system bash command"))

			params := tool.Function.Parameters.(map[string]any)
			Expect(params["type"]).To(Equal("object"))
			props := params["properties"].(map[string]any)

			Expect(props).To(HaveKey("command"))
			Expect(props["command"]).To(HaveKeyWithValue("type", "string"))
			Expect(props["command"]).To(HaveKeyWithValue("description", "The command to execute"))

			Expect(params["required"]).To(ConsistOf([]string{"command"}))
		})

		Context("when the model invokes the tool", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "abc123",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "executeCommand",
											Arguments: `{"command":"whoami"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
				e.ExecuteReturns("engineer", nil)
			})

			It("executes the command", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 1))
				ctx, msgs, _ := m.GenerateContentArgsForCall(0)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))

				Eventually(e.ExecuteCallCount).Should(Equal(1))
				command := e.ExecuteArgsForCall(0)
				Expect(command).To(Equal("whoami"))
			})

			It("shares the command output with the model", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 2))

				ctx, msgs, _ := m.GenerateContentArgsForCall(0)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(2))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				ctx, msgs, _ = m.GenerateContentArgsForCall(1)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(4))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				Expect(msgs[2].Role).To(Equal(llms.ChatMessageTypeAI))
				Expect(msgs[2].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCall{
							ID:   "abc123",
							Type: "function",
							FunctionCall: &llms.FunctionCall{
								Name:      "executeCommand",
								Arguments: `{"command":"whoami"}`,
							},
						},
					},
				))

				Expect(msgs[3].Role).To(Equal(llms.ChatMessageTypeTool))
				Expect(msgs[3].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: "abc123",
							Name:       "executeCommand",
							Content:    "The command ran successfully with the output:\nengineer",
						},
					},
				))
			})

		})

		Context("when the model tool arguments cannot be parsed", func() {
			BeforeEach(func() {
				m.GenerateContentReturnsOnCall(0,
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "executeCommand",
											Arguments: `{"not json"`,
										},
									},
								},
							},
						},
					},
					nil,
				)
			})
			It("errors", func() {
				Eventually(errCh).Should(Receive(MatchError(MatchRegexp(`could not parse tool call arguments: "executeCommand":.*JSON`))))
			})
		})

		Context("when there is an error executing the command", func() {
			BeforeEach(func() {
				m.GenerateContentReturns(
					&llms.ContentResponse{
						Choices: []*llms.ContentChoice{
							{
								ToolCalls: []llms.ToolCall{
									{
										ID:   "abc123",
										Type: "function",
										FunctionCall: &llms.FunctionCall{
											Name:      "executeCommand",
											Arguments: `{"command":"whoami"}`,
										},
									},
								},
							},
						},
					},
					nil,
				)
				e.ExecuteReturns("user unknown", errors.New("some-error"))
			})

			It("shares the command output with the model", func() {
				Eventually(m.GenerateContentCallCount).Should(BeNumerically(">=", 2))

				ctx, msgs, _ := m.GenerateContentArgsForCall(0)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(2))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				ctx, msgs, _ = m.GenerateContentArgsForCall(1)
				Expect(ctx).To(Equal(ctx))
				Expect(msgs).To(HaveLen(4))
				Expect(msgs[0].Role).To(Equal(llms.ChatMessageTypeSystem))
				Expect(msgs[1].Role).To(Equal(llms.ChatMessageTypeHuman))

				Expect(msgs[2].Role).To(Equal(llms.ChatMessageTypeAI))
				Expect(msgs[2].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCall{
							ID:   "abc123",
							Type: "function",
							FunctionCall: &llms.FunctionCall{
								Name:      "executeCommand",
								Arguments: `{"command":"whoami"}`,
							},
						},
					},
				))

				Expect(msgs[3].Role).To(Equal(llms.ChatMessageTypeTool))
				Expect(msgs[3].Parts).To(Equal(
					[]llms.ContentPart{
						llms.ToolCallResponse{
							ToolCallID: "abc123",
							Name:       "executeCommand",
							Content:    "The command failed with the output:\nuser unknown",
						},
					},
				))
			})
		})
	})
	Context("when there is an error talking to the model", func() {
		BeforeEach(func() {
			m.GenerateContentReturns(nil, errors.New("some error"))
		})
		It("errors", func() {
			Eventually(errCh).Should(Receive(MatchError("some error")))
		})
	})

})
