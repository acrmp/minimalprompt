/*
Minimalprompt invokes a LLM to manipulate an output directory.

It requires the system prompt, an initial prompt and the output directory.

WARNING: It dangerously provides the LLM access to write files and run
commands. Any usage is at your own risk.

It will prompt the user if the LLM will not proceed without a prompt. Send
an EOF (CTRL-D) to end the prompt message.
*/
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"

	"github.com/acrmp/minimalprompt/agent"
	"github.com/tmc/langchaingo/llms/anthropic"
)

const anthropicVersion = "claude-3-5-sonnet-20240620"

func printUsageAndExit() {
	fmt.Fprintf(os.Stderr, "minimalprompt [SYSTEM PROMPT] [INITIAL PROMPT] [OUTPUT DIR]\n")
	os.Exit(1)
}

func main() {
	logger := slog.New(tint.NewHandler(os.Stderr, nil))

	if len(os.Args) != 4 {
		printUsageAndExit()
	}

	sp, err := os.ReadFile(os.Args[1])
	if err != nil {
		printUsageAndExit()
	}
	p, err := os.ReadFile(os.Args[2])
	if err != nil {
		printUsageAndExit()
	}
	d := os.Args[3]

	m, err := anthropic.New(anthropic.WithModel(anthropicVersion))
	if err != nil {
		logger.Error("initializing model", "err", err)
		os.Exit(1)
	}

	a := agent.NewLLMWrapper(
		logger,
		string(sp),
		string(p),
		m,
		agent.NewBashExecutor(logger, d),
		agent.NewSimpleFileWriter(logger, d),
		agent.NewTerminalPrompter(os.Stdin, os.Stdout),
	)

	if err = a.Run(context.Background()); err != nil {
		logger.Error("running agent", "err", err)
		os.Exit(1)
	}
}
