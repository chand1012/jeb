# Example requests

Each JSON file is one request for the default CLI mode. With Ollama running and the configured model available, run an example from the repository root:

```sh
go run . < examples/choice.json
go run . -m qwen3.5:9b < examples/mixed.json
```

The output is one JSON object with named entries under `answers` and aggregated token counts under `usage`. Model answers and probabilities can vary between runs. The `mixed.json` example exercises all three question types in one request. `noul_with_criteria.json` exercises the optional labeled Yes/No criteria.
