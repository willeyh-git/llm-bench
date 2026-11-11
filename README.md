# llm-bencher

A terminal benchmark for OpenAI-compatible local LLM servers. Pick a model from the server's `/v1/models` list, and the tool measures prefill speed at several prompt sizes and greedy decode speed, then saves the run as JSON.

The benchmark reproduces the prefill and greedy-decode measurements from the `bench/harness/bench.py` harness in [nace-ai/kapitsa](https://github.com/nace-ai/kapitsa). It does not include that repo's inference engine logic, tool calling, agent replay or sampled decode.

## Features

- Model picker that lists every model id the server reports, with its `loaded` or `unloaded` status.
- Cold prefill at 4K, 16K, 32K, 64K, 96K and 128K prompt tokens, taken from the first request of each context decode run at that size.
- Greedy decode over 512 generated tokens, plus greedy decode after a corpus-sized prompt at 4K, 16K, 32K, 64K, 96K and 128K.
- Automatic skipping of any size that does not fit the model's `context_length`, with the reason recorded.
- A live run screen with one line per phase, and a summary card at the end.
- Each run saved as a JSON file.
- A single static binary. The prompt corpus is embedded with `go:embed`, so no data file is needed at runtime.

## Requirements

- Go 1.24.2 or newer.
- An OpenAI-compatible server that exposes `GET /v1/models` and `POST /v1/chat/completions` with streaming. The server must already be running with the models you want to test. The tool does not start, stop or load models.
- A terminal that supports full-screen TUI applications.

## Build

```sh
go build -o llm-bencher ./cmd/llm-bencher
```

Or run it directly:

```sh
go run ./cmd/llm-bencher
```

## Usage

```sh
llm-bencher [flags]
```

### Sidecar config

Settings live in `llm-bencher.json` in the working directory. If no `base_url` is saved, the app asks for one at startup and saves it there. The run counts default to the values below when a key is missing.

```json
{
  "base_url": "http://0.0.0.0:8080/v1",
  "runs": {
    "greedy": 5, "4k": 5, "16k": 5, "32k": 5, "64k": 5, "96k": 2, "128k": 2
  }
}
```

| Flag | Default | Description |
| --- | --- | --- |
| `--base-url` | (from `llm-bencher.json`) | OpenAI-compatible base URL for this session. Models are read from `{base-url}/models`. It is not saved to the sidecar. |
| `--model` | (empty) | Model id to benchmark. When set, the picker is skipped. |
| `--out` | (empty) | Path of the result JSON file. Default is `results/<UTC timestamp>-<model slug>.json`. |

Examples:

```sh
# Pick a model from the server's list
llm-bencher --base-url http://0.0.0.0:8080/v1

# Benchmark one model directly and write to a chosen file
llm-bencher --model "Qwen3.8-27B" --out results/qwen-run.json
```

Before the TUI starts, the binary checks the embedded corpus against its expected sha256 and exits with an error if it does not match.

### Picker keys

| Key | Action |
| --- | --- |
| `↑` / `↓` or `k` / `j` | Move the selection |
| `Enter` | Start a run with the selected model |
| `r` | Retry the model list after an error |
| `Esc` or `q` | Quit |

If the model list is empty or the request fails, the picker shows the error. Press `r` to retry or `q` to quit.

### Stopping a run

Press `Ctrl+C` during a run. The in-flight request is cancelled and the program exits. No result file is written for a cancelled run.

## What a run measures

A run executes these phases in order. Each phase is a line on the run screen with a check or warning mark and its measured value.

| Phase | What it measures |
| --- | --- |
| warm-up | A request of about 2,700 tokens taken from a corpus offset that no timed prompt uses. Its result is not recorded. It primes the server and loads an unloaded model. |
| decode greedy (5 runs) | Five greedy 512-token generations on a short fixed prompt. The median, min and max are reported. |
| prefill and decode 4K, 16K, 32K, 64K (5 runs each) | Five greedy 512-token generations after a cold corpus prompt of the given size. Each run has a random nonce, so the prefix cache cannot be reused. The TTFT and prompt tokens of each run give the prefill result for that size. |
| prefill and decode 96K, 128K (2 runs each) | Two runs of the same kind. These sizes use fewer runs because they are slow, so their medians rest on two samples. |

### Metric definitions

- **Prefill TTFT** is the time from sending the request to the first content-bearing stream event.
- **Prefill tok/s** is `usage.prompt_tokens` divided by TTFT.
- **Decode tok/s** (the headline figure) is measured on the middle of the stream. The first 16 and last 4 tokens are dropped, and the rate is computed over the rest.
- **End-to-end decode tok/s** is reported alongside, as `(tokens - 1) / total time`.

Requests use these settings:

- Thinking is off for every request, through `chat_template_kwargs.enable_thinking=false` and `reasoning_effort="none"`. Reasoning output therefore does not count as answer tokens.
- Greedy decoding: `temperature=0`, `top_p=1`, `top_k=1`, `min_p=0`, `presence_penalty=0`, `frequency_penalty=0`, `repeat_penalty=1`.
- Every request uses `max_tokens=512`. TTFT does not depend on `max_tokens`, so prefill numbers are taken from the context decode requests.
- Streaming is on, and `stream_options.include_usage` is set so the server reports prompt and completion token counts.

Prompts are cut from an embedded text corpus by character count, at 4.25 characters per token. Every run therefore gets identical text.

### Skipped sizes

A prefill or decode size is skipped when it does not fit the model's `context_length`. The check adds a 5% margin for prompt-token overshoot, because the corpus-to-token ratio varies between tokenizers.

- A size is skipped when the estimated prompt plus 512 output tokens is larger than `context_length`. Prefill and decode at that size are skipped together.
- If the server rejects a prompt for context overflow, that size is also recorded as skipped, with the server's message, and the run continues.

A skipped size appears on the run screen and in the summary with its reason. The other sizes still run and are saved.

If the model reports no `context_length`, every size is sent, and a server rejection is the only check.

### Validity and flags

A measurement is marked **invalid** when a check fails. Invalid numbers stay in the output, with a reason.

- `usage.prompt_tokens` is missing from the stream.
- No content-bearing event arrived.
- `cached_tokens` is above 16 on a cold prompt.
- The prompt-token count is outside ±1% of the reference count. This check runs only for Qwen model ids at 4K and 16K. Other models and sizes are recorded as unchecked.
- Reasoning output is present in a decode response, which means thinking was not disabled.

A decode run is **flagged** when `finish_reason` is not `length`, or when `completion_tokens` is not 512. The value is still kept.

## Results

Each completed run writes one JSON file, by default to `results/<UTC timestamp>-<model slug>.json`. Use `--out` to choose another path. The `results/` directory is created if it does not exist. A cancelled or failed run writes nothing.

The file has these top-level keys:

| Key | Contents |
| --- | --- |
| `schema` | Result schema version, currently `2`. |
| `model` | The model id. |
| `started_at` | UTC start time of the run. |
| `prefill` | Map from prompt size (`"4000"`, `"16000"`, and so on) to the prefill result: `target`, `prompt_tokens`, `ttft_seconds` (median), `ttft_samples` (one per run), `tps` (prompt tokens divided by the median TTFT), `cached_tokens`, `invalid` and `flags`. |
| `skipped` | Map from size to the skip reason, for prefill and context decode. Present only when something was skipped. |
| `decode` | The short-prompt greedy decode: `runs`, and `tps_median`, `tps_min`, `tps_max`, `invalid` and `flags`. |
| `decode_at_context` | Map from context size to a decode result of the same shape. |
| `decode_at_context_skipped` | Map from context size to the skip reason. Present only when something was skipped. |
| `corpus` | sha256 of the embedded corpus the run used. |

The summary card on screen shows the medians, the min and max, the invalid and flagged reasons, and the path of the saved file.

## Project layout

```
cmd/llm-bencher/       entry point and flags
internal/server/       GET /v1/models client
internal/bench/        measurement code
  corpus.go            embedded corpus, sha256 check, prompt cutting
  prompts.go           prefill and decode message builders, nonces
  stream.go            streaming chat client (TTFT, usage, timestamps)
  prefill.go           warm-up and prefill measurement and validation
  decode.go            greedy decode measurement and window rate
  runner.go            phase orchestration, skip rules, events, JSON output
  result.go            result types and statistics
  corpus/              corpus file and NOTICE.md
internal/tui/          Bubble Tea picker, run screen and summary card
results/               default location of result JSON files
LICENSE                Apache-2.0 license text
```

The `internal/bench` package has no terminal dependency, so the runner can be tested without a TUI. The TUI consumes the runner's events.

## Testing

```sh
gofmt -l .
go vet ./...
go test ./...
```

The tests use `httptest` fake servers that return canned `/v1/models` JSON and SSE streams. They do not need a GPU server. A real benchmark against a running server is a manual check.

## Corpus and license

The prompt corpus, `internal/bench/corpus/bench_corpus.txt`, is excerpts of Hugging Face Transformers. It is a frozen 650,000-character file, and its sha256 is `58a7756fd94fc48a7b7306d330b2c44663bdf22faa943f82430966c2878081b2`. The source and attribution are in `internal/bench/corpus/NOTICE.md`.

The corpus and the tool are licensed under Apache-2.0. The license text is in `LICENSE`. The corpus file is copied byte for byte from kapitsa, so its results stay comparable to that harness.

## Notes

- Results depend on the server, the hardware and the tokenizer. Compare runs only on the same machine and the same server settings.
- Prefill and decode at 64K and above are slow, and a full run can take a long time on a slow machine.
- Models with no reported `context_length` are sent every size, so a rejection by the server is the only protection there.
