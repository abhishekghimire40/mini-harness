# mini-harness

A tiny agent harness in Go, built to understand what tools like Claude Code do under the hood.

An LLM can only output text. A **harness** is the code around it that gives it tools, runs them, and feeds the results back. This project is that loop in one file, using the [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go).

## What it does

- **Chat loop**: type questions in the terminal; the full conversation is re-sent on every call, so the model "remembers".
- **Agent loop**: call the model → run any tools it asks for → send the results back → repeat until `stop_reason` is not `tool_use`.
- **Two tools**: `read_file` and `list_files`.
- **Errors go back to the model**: bad paths, unknown tools and malformed input return an `isError` tool result instead of crashing.
- **Basic file blocking**: `.env` (and a few other types) can't be read.

## Run it

```sh
echo 'API_KEY=your-anthropic-key' > .env
go run .
```

```
prompt -> which go files are in this folder?
[harness] API call 1
[harness] list_files(.)
[harness] API call 2
There's one Go source file in this folder: main.go ...
prompt -> exit
```

## Known gaps

This is a learning project, not a safe sandbox:

- Paths aren't confined to the project folder (`../`, absolute paths work).
- `.env` variants like `.env.local` aren't blocked; `list_files` has no checks.
- One API error ends the session; history is never trimmed.

## Credit

Inspired by Theo's article "What is a 'harness'?".
