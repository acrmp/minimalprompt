# Minimal prompting

Playing with Claude.

**WARNING: It dangerously provides the LLM access to write files and run
commands. Any usage is at your own risk.**

## Usage

1. Export your Anthropic API key:

```
$ export ANTHROPIC_API_KEY=YOUR_API_KEY
```

2.  Generate stories for the epic:

```
$ go run cmd/main.go prompts/product-manager.txt prompts/epic.txt output-dir
```

3. Review the stories that were generated:

```
$ less output-dir/stories.txt
```

Optionally provide the model feedback on the stories generated:

```
reply>Can you add some more detail to the user profile story?
<CTRL-D>
```

4. Generate code for the stories:

```
$ go run cmd/main.go prompts/engineer.txt output-dir/stories.txt output-dir
```
