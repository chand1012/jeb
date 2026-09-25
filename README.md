# Jeb

**Jev-compatible decisions from an OpenAI-compatible LLM with logprobs.**

[![Crossbuild](https://github.com/chand1012/jeb/actions/workflows/crossbuild.yml/badge.svg)](https://github.com/chand1012/jeb/actions/workflows/crossbuild.yml)
[![Docker](https://github.com/chand1012/jeb/actions/workflows/docker.yml/badge.svg)](https://github.com/chand1012/jeb/actions/workflows/docker.yml)
[![Go 1.27.1](https://img.shields.io/badge/Go-1.27.1-00ADD8?logo=go&logoColor=white)](go.mod)

**[Quick start](#quick-start) · [Question types](#request-format) · [HTTP API](#http-server) · [Examples](examples/README.md)**

Send Jeb a `state` and one or more Choice, Score, or Noul questions. It prompts
the configured model, reads its token log probabilities, and returns structured
JSON at `POST /v1/systemone`. The same request format works through the CLI.

Inspired by this [NobodyWho article](https://www.nobodywho.ai/posts/jev-in-25-lines/). See [TypeSafe's introduction to Jev](https://docs.typesafe.ai/introduction)
for the original decision model and its three primitives.

## Requirements

- An OpenAI-compatible `POST /v1/chat/completions` endpoint and a model that returns
  `choices[0].logprobs.content[0].top_logprobs` when asked for `logprobs`.
- A provider that accepts the request fields Jeb sends: `model`, `messages`,
  `logprobs`, `top_logprobs`, and `max_tokens`. Jeb also sends `reasoning_effort`
  unless it is configured as an empty string, and sends `temperature` when set.

An endpoint can be OpenAI-compatible for ordinary chat while lacking the token
log probabilities Jeb needs. Check that capability before using a provider.

### Known Working Endpoints

- [Ollama](https://ollama.com)
- [OpenAI](https://openai.com)
- [Cerebras](https://cerebras.ai) Qwen 3.8 only

## Install the Binary

### Linux and macOS
```sh
curl -fsSL https://raw.githubusercontent.com/chand1012/jeb/main/install.sh -o install.sh
# Optional. Set if you need an alternative installation directory
# export JEB_INSTALL_DIR=/path/to/install
sh install.sh # installs to ~/.local/bin by default
```

### Windows
Download the binary from [releases](https://github.com/chand1012/jeb/releases)


### Install with Go

Install the latest version from source with Go:

```sh
go install github.com/chand1012/jeb@latest
```

To install the code in your current checkout instead, run this from the
repository root:

```sh
go install .
```

## Quick start

1. [Install Jeb](#installation).

2. If you use another OpenAI-compatible provider, set its base URL
   and model name as described in [Configuration](#configuration).

   ```sh
   export OPENAI_MODEL=qwen3.5:9b # or another model you have available
   ```

3. Evaluate an example request:

   ```sh
   jeb < examples/choice.json
   ```

The CLI reads **one** JSON request from stdin and writes **one** JSON response to
stdout. Model calls may vary between runs. Start with
[the mixed example](examples/mixed.json) to exercise all three question types:

```sh
jeb < examples/mixed.json
```

## Request format

The request contains a shared `state` string and a `questions` object. Each key
in `questions` becomes a key in the response's `answers` object. Jeb sends each
question as a separate chat completion request against the same state.

```json
{
  "state": "The battery is at 3%. A firmware update requires at least 30%.",
  "questions": {
    "start_update": {
      "type": "noul",
      "instructions": "Should the update start now?"
    }
  }
}
```

Jeb supports these question types:

| Type | `criteria` | Returned fields |
| --- | --- | --- |
| `choice` | Object mapping option names to descriptions | `type`, `choice`, `probabilities`, `confidence` |
| `score` | Ordered array of level descriptions | `type`, `score`, `legend`, `probabilities`, `confidence` |
| `noul` | Optional object with `true` and `false` descriptions | `type`, `noul` |

### Choice: select one option

The object keys name the options. Write each description to explain when that
option fits; Jeb builds the model prompt and handles option numbering for you.

```json
{
  "type": "choice",
  "instructions": "Choose the safer venue under the forecast.",
  "criteria": {
    "Outdoor": "Fits everyone but provides no shelter from heavy rain.",
    "Indoor": "Provides shelter but requires limiting attendance."
  }
}
```

The answer contains the selected option, probabilities, and a `confidence`
value. Probability keys are zero-based numeric option labels assigned in
alphabetical order of option name.

### Score: rate against an ordered rubric

Put rubric levels in order from low to high. Jeb returns a `legend` that maps
level numbers back to their descriptions. Levels start at 0, so a three-level
rubric has labels `0`, `1`, and `2`. The numeric `score` is a weighted average
of those indices and can fall between levels.

```json
{
  "type": "score",
  "instructions": "Rate the weather risk for an outdoor event.",
  "criteria": ["Low risk", "Moderate risk", "High risk"]
}
```

The answer also includes a probability for each zero-based level and a
`confidence` value.

### Noul: estimate the chance of Yes

A Noul answer is a number from 0 to 1 representing the model's reported
probability for `Yes`. You may omit `criteria`, or supply descriptions for
`true` and `false`:

```json
{
  "type": "noul",
  "instructions": "Should the refund be approved under the policy?",
  "criteria": {
    "true": "The request is within 30 days and the item is unused.",
    "false": "The request is late or the item has been used."
  }
}
```

Jeb uses the first token's `Yes` log probability when present. If only `No`
is present and the model answered `No`, it returns `1 - P(No)`. It returns an
error when it cannot derive either value. Noul does not return a separate
`confidence` field.

## Response format

The response contains the configured model name, one answer per named question,
and summed upstream token usage:

```json
{
  "model": "qwen3.5:9b",
  "answers": {
    "start_update": {
      "type": "noul",
      "noul": 0.02
    }
  },
  "usage": {
    "input_tokens": 87,
    "output_tokens": 1
  }
}
```

The numbers above illustrate the shape; they are not the result of a measured
model run. See [all example requests](examples/README.md).
## HTTP server

Start the server with the same provider settings used by the CLI:

```sh
jeb serve --host 127.0.0.1 --port 6102
```

Then send a JSON request:

```sh
curl --fail-with-body \
  -H 'Content-Type: application/json' \
  --data-binary @examples/mixed.json \
  http://127.0.0.1:6102/v1/systemone
```

The endpoint accepts `POST` only. Invalid JSON receives HTTP 400; method
mismatches receive 405; processing and upstream errors currently receive 502.
The server logs method, path, status, duration, and remote address. It does not
provide authentication or TLS, and its default host is `0.0.0.0`; bind it to
`127.0.0.1` for local use or place access controls in front of it.

## Configuration

Jeb reads optional `config.yaml` from the working directory, `./config/`, or
`~/.config/jeb/`. Set `JEB_CONFIG_FILE` to use a specific path. Copy
[config.example.yaml](config.example.yaml) as a starting point. The precedence
is **explicit CLI flags > environment variables > config file > built-in
defaults**.

| Setting | Environment variable | Default | Purpose |
| --- | --- | --- | --- |
| `server.host` | `JEB_HOST` | `0.0.0.0` | HTTP listen address |
| `server.port` | `JEB_PORT` | `6102` | HTTP listen port |
| `openai.base_url` | `OPENAI_BASE_URL` | `http://localhost:11434/v1` | Provider API base URL |
| `openai.api_key` | `OPENAI_API_KEY` | Empty | Bearer token, if needed |
| `openai.model` | `OPENAI_MODEL` | `qwen3.5:9b` | Upstream model ID |
| `openai.max_tokens` | `OPENAI_MAX_TOKENS` | `10` | Maximum generated tokens per question |
| `openai.reasoning_effort` | `OPENAI_REASONING_EFFORT` | `none` | Optional provider hint |
| `openai.temperature` | `OPENAI_TEMPERATURE` | Unset | Optional sampling temperature |
| `openai.timeout` | `OPENAI_TIMEOUT` | `60s` | Timeout per completion request |
| `openai.max_retries` | `OPENAI_MAX_RETRIES` | `3` | Configured value; retries are not implemented yet |
| `concurrency.max_requests` | `JEB_MAX_REQUESTS` | `1` | Maximum simultaneous model requests |

For providers that reject `reasoning_effort`, set `openai.reasoning_effort: ""`
in your YAML config. For providers that need an API key, supply it through an
environment variable or a local config file kept out of version control.
The current `.gitignore` does **not** exclude `config.yaml`.

CLI flags include `--base-url`, `--api-key`, `--model` (`-m`), `--max-tokens`,
`--reasoning-effort`, `--timeout`, `--max-retries`, and `--max-requests`.
`serve` also accepts `--host` (`-H`) and `--port` (`-p`). Run `jeb --help`
or `jeb serve --help` for the full flag list.

## Container and releases

Build and run the image locally:

```sh
docker build -t jeb .
docker run --rm -p 127.0.0.1:6102:6102 \
  --env OPENAI_BASE_URL \
  --env OPENAI_MODEL \
  --env OPENAI_API_KEY \
  jeb
```

Set those variables in your shell first. A container cannot use its own
`localhost` to reach a provider on the host; configure a provider URL reachable
from inside the container.

GitHub Actions builds Linux `amd64` and `arm64` images for GHCR on pushes to
`main` and `v*` tags. It also builds Linux, macOS, and Windows binaries for
`amd64` and `arm64`. Tagged builds publish archives and `checksums.txt` to a
GitHub release. Binaries and images record the version tag (or `dev`), commit
SHA, and UTC build date; inspect a binary with `jeb version`.

## How Jeb calculates decisions

For each question, Jeb sends a system prompt and a user prompt containing the
state, instructions, and numbered options. It asks the provider for one short
answer and top log probabilities for the first generated token. Choice and
Score normalize the probabilities for the recognized numeric labels; Score
then calculates a weighted average. Noul derives the chance of `Yes` from its
reported token probability. The configured concurrency limit controls how many
question requests are in flight at once.

These values depend on the provider's tokenizer, token ranking, and
`top_logprobs` cap. If a valid option is absent from the returned top tokens,
the normalized distribution is incomplete and may be misleading. Jeb currently
does not calibrate those probabilities against outcome data. For decisions
with real consequences, evaluate the chosen model on your own labeled cases
and keep application rules or human review in control of the final action.

## Troubleshooting

- **Missing token log probabilities:** Confirm the provider supports both
  `logprobs` and `top_logprobs` on chat completions for the selected model.
- **Unexpected option or low confidence:** Inspect the provider's first token
  and its top log probabilities. Choice and Score expect one numeric option
  label; Noul expects `Yes` or `No`.
- **Provider rejects a request field:** Check its support for
  `reasoning_effort`, `temperature`, `top_logprobs`, and `max_tokens`. Configure
  an empty reasoning effort in YAML when that field is unsupported.
- **The container cannot reach a local provider:** Replace `localhost` in
  `OPENAI_BASE_URL` with an address reachable from the container.
- **HTTP 502:** The handler uses 502 for processing failures as well as
  upstream failures. Check the response body and server logs.

## Development

```sh
go test ./...
go vet ./...
go build ./...
```

The [Justfile](Justfile) provides `just build`, `just serve`, and other local
tasks. There are currently no Go test files; the CI checks compile packages
and run `go vet`. The request examples in [`examples/`](examples/README.md)
provide manual integration cases for a compatible provider.
