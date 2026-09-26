# Run Jeb on JevBench

From the Jeb repository, with the adjacent `jevbench` checkout and Ollama running:

```sh
python3 benchmarks/run_jevbench.py --pilot
```

The pilot sends seven public decisions per model: two choice, two Noul, two score, and one structured-state case. The default models are `qwen3.5:0.8b`, `qwen3.5:9b`, and `qwen3.8:27b-mlx`. To run all 231 public decisions per model:

```sh
python3 benchmarks/run_jevbench.py
```

Use `--models MODEL [MODEL ...]` to select models, `--jevbench-root PATH` for another checkout, and `--timeout-s N` for slow hardware. The script builds Jeb, starts a separate localhost Jeb server for each model, uses JevBench's runner and scoring, and stops each server after its run. Results go to a unique ignored directory under `benchmarks/results/`. Each model directory contains raw requests and responses, per-task records, `summary.json`, `manifest.json`, a Jeb log, and the local zero-reservation ledger. No API key is required for the default Ollama endpoint.

Jeb accepts state as text, so structured JevBench states are serialized to compact JSON with sorted keys. Jeb's zero-based choice probability keys are mapped to the corresponding option names sorted by name. Missing probability entries are left missing so JevBench can flag an invalid distribution. Cost is recorded as unmetered local compute, rather than zero dollars. These public-only runs are diagnostic and are not an official JevBench leaderboard score; the held-out tasks and official cost basis are unavailable here.
