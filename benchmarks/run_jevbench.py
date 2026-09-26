#!/usr/bin/env python3
"""Run Jeb against JevBench's public tasks using local Ollama models."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid

JEB_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_JEVBENCH_ROOT = JEB_ROOT.parent / "jevbench"
DEFAULT_MODELS = ("qwen3.5:0.8b", "qwen3.5:9b", "qwen3.8:27b-mlx")


def load_jevbench(root: Path):
    if not (root / "jevbench" / "runner.py").is_file():
        raise FileNotFoundError(f"JevBench checkout not found at {root}")
    sys.path.insert(0, str(root))
    from jevbench.adapters.typesafe import TypeSafeAdapter
    from jevbench.budget import Ledger
    from jevbench.runner import Runner
    from jevbench.summarize import summarize
    from jevbench.tasks import dataset_hash, load_jsonl
    return TypeSafeAdapter, Ledger, Runner, summarize, dataset_hash, load_jsonl


def make_adapter(base_class):
    class JebAdapter(base_class):
        name = "jeb"
        cost_basis = "local_compute_not_metered"

        def __init__(self, endpoint: str, model: str, timeout_s: float):
            super().__init__(endpoint=endpoint, model=model, key_env="", timeout_s=timeout_s)

        def build_request(self, task) -> dict:
            body = super().build_request(task)
            if not isinstance(body["state"], str):
                body["state"] = json.dumps(body["state"], ensure_ascii=False,
                                           sort_keys=True, separators=(",", ":"))
            return body

        def run(self, task):
            result = super().run(task)
            if not result.ok:
                return result
            if result.model != self.model:
                result.ok = False
                result.error = f"server model {result.model!r} differs from {self.model!r}"
                return result
            if task.question["type"] == "choice":
                labels = sorted(task.question["criteria"])
                mapping = {str(i): label for i, label in enumerate(labels)}
                result.probs = {mapping.get(key, key): value
                                for key, value in result.probs.items()}
            return result

        def reserve_estimate(self, task) -> float:
            return 0.0

    return JebAdapter


def git_revision(root: Path) -> str:
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root,
                                   text=True).strip()


def git_dirty(root: Path) -> bool:
    return bool(subprocess.check_output(["git", "status", "--porcelain"],
                                        cwd=root, text=True).strip())


def select_tasks(tasks: list, pilot: bool) -> list:
    if not pilot:
        return tasks
    selected = []
    for kind in ("choice", "noul", "score"):
        selected.extend([t for t in tasks if t.question["type"] == kind][:2])
    structured = next((t for t in tasks if not isinstance(t.state, str)), None)
    if structured is not None and structured.id not in {t.id for t in selected}:
        selected.append(structured)
    return selected


def free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def wait_for_server(port: int, process: subprocess.Popen, log_path: Path) -> None:
    url = f"http://127.0.0.1:{port}/v1/systemone"
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"Jeb exited early; see {log_path}")
        try:
            urllib.request.urlopen(url, timeout=1)
        except urllib.error.HTTPError as error:
            if error.code == 405:
                return
        except (urllib.error.URLError, TimeoutError):
            pass
        time.sleep(0.2)
    raise TimeoutError(f"Jeb did not start; see {log_path}")


def write_json(path: Path, value: dict) -> None:
    path.write_text(json.dumps(value, indent=2, sort_keys=True, ensure_ascii=False)
                    + "\n", encoding="utf-8")


def run_model(args, model, tasks, run_root, classes, revisions, data_hash):
    TypeSafeAdapter, Ledger, Runner, summarize, _, _ = classes
    JebAdapter = make_adapter(TypeSafeAdapter)
    model_dir = run_root / model.replace(":", "_").replace("/", "_")
    model_dir.mkdir()
    port = free_port()
    command = [str(run_root / "jeb"), "--model", model,
               "--base-url", args.base_url, "--max-tokens", str(args.max_tokens),
               "--reasoning-effort", args.reasoning_effort,
               "--timeout", f"{args.timeout_s}s", "--max-retries", "0",
               "--max-requests", "1", "serve", "--host", "127.0.0.1",
               "--port", str(port)]
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    log_path = model_dir / "jeb.log"
    print(f"[{model}] starting Jeb on port {port}", flush=True)
    with log_path.open("w", encoding="utf-8") as log:
        process = subprocess.Popen(command, cwd=JEB_ROOT, stdout=log,
                                   stderr=subprocess.STDOUT)
        try:
            wait_for_server(port, process, log_path)
            adapter = JebAdapter(f"http://127.0.0.1:{port}", model,
                                 args.timeout_s + 10)
            ledger = Ledger(model_dir / "ledger.jsonl", cap_usd=0)
            runner = Runner(adapter, ledger, raw_dir=model_dir / "raw",
                            default_reserve_usd=0)
            records = runner.run_all(tasks, results_path=model_dir / "results.jsonl",
                                     progress_every=1 if args.pilot else 10)
            summary = summarize(tasks, records, ledger_charged=ledger.charged)
            write_json(model_dir / "summary.json", summary)
            manifest = {
                "adapter": "jeb", "model": model, "mode": "pilot" if args.pilot else "full",
                "dataset_hash": data_hash, "n_planned": len(tasks),
                "n_attempted": len(records),
                "structured_states_serialized": sum(not isinstance(t.state, str) for t in tasks),
                "state_serialization": "compact JSON, UTF-8, sorted keys",
                "choice_probability_mapping": "Jeb numeric keys map to sorted criteria names; missing keys remain missing",
                "jeb_revision": revisions["jeb"], "jevbench_revision": revisions["jevbench"],
                "jeb_worktree_dirty": revisions["jeb_dirty"],
                "jevbench_worktree_dirty": revisions["jevbench_dirty"],
                "harness_sha256": revisions["harness_sha256"],
                "base_url": args.base_url, "max_tokens": args.max_tokens,
                "reasoning_effort": args.reasoning_effort, "timeout_s": args.timeout_s,
                "max_retries": 0, "max_requests": 1,
                "cost_basis": adapter.cost_basis,
                "started_utc": started,
                "finished_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
            }
            write_json(model_dir / "manifest.json", manifest)
            print(f"[{model}] {len(records)}/{len(tasks)} attempted; "
                  f"{summary['n_valid']} valid; accuracy={summary['accuracy']}; "
                  f"results: {model_dir}", flush=True)
            return len(records) == len(tasks)
        finally:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--jevbench-root", type=Path, default=DEFAULT_JEVBENCH_ROOT)
    parser.add_argument("--models", nargs="+", default=DEFAULT_MODELS)
    parser.add_argument("--pilot", action="store_true",
                        help="run two tasks per type plus a structured-state case")
    parser.add_argument("--output-dir", type=Path, default=JEB_ROOT / "benchmarks/results")
    parser.add_argument("--base-url", default="http://127.0.0.1:11434/v1")
    parser.add_argument("--max-tokens", type=int, default=10)
    parser.add_argument("--reasoning-effort", default="none")
    parser.add_argument("--timeout-s", type=int, default=120)
    args = parser.parse_args()
    if args.timeout_s <= 0 or args.max_tokens <= 0:
        parser.error("--timeout-s and --max-tokens must be positive")
    root = args.jevbench_root.resolve()
    classes = load_jevbench(root)
    _, _, _, _, dataset_hash, load_jsonl = classes
    files = [root / "datasets/public" / f"{name}.jsonl"
             for name in ("easy", "original", "hard")]
    tasks = select_tasks([task for path in files for task in load_jsonl(str(path))],
                         args.pilot)
    if len({t.id for t in tasks}) != len(tasks):
        raise ValueError("duplicate task IDs in selected datasets")
    revisions = {
        "jeb": git_revision(JEB_ROOT), "jevbench": git_revision(root),
        "jeb_dirty": git_dirty(JEB_ROOT), "jevbench_dirty": git_dirty(root),
        "harness_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
    }
    run_root = args.output_dir.resolve() / (dt.datetime.now(dt.timezone.utc).strftime(
        "%Y%m%dT%H%M%SZ") + "-" + uuid.uuid4().hex[:8])
    run_root.mkdir(parents=True)
    print(f"Building Jeb; {len(tasks)} JevBench tasks per model; output: {run_root}",
          flush=True)
    subprocess.run(["go", "build", "-o", str(run_root / "jeb"), "."],
                   cwd=JEB_ROOT, check=True)
    data_hash = dataset_hash(tasks)
    complete = []
    for model in args.models:
        complete.append(run_model(args, model, tasks, run_root, classes,
                                  revisions, data_hash))
    return 0 if all(complete) else 1


if __name__ == "__main__":
    sys.exit(main())
