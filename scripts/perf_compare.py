#!/usr/bin/env python3
"""Compare C djpeg and djpeg-go CLI throughput on random100 JPEGs."""

from __future__ import annotations

import argparse
import json
import os
import statistics
import subprocess
import sys
import time
from pathlib import Path


MODES = {
    "default": {
        "c_args": ["-dct", "int", "-ppm"],
        "go_args": ["--dct", "int", "--ppm"],
    },
    "nosmooth": {
        "c_args": ["-dct", "int", "-nosmooth", "-ppm"],
        "go_args": ["--dct", "int", "--nosmooth", "--ppm"],
    },
}


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def c_reference(root: Path) -> tuple[Path, dict[str, str]]:
    lib_dir = root / "jpeg-9f" / ".libs"
    tool = lib_dir / "djpeg"
    if not tool.exists():
        raise SystemExit(f"C reference not found: {tool}")
    env = os.environ.copy()
    current = env.get("LD_LIBRARY_PATH", "")
    env["LD_LIBRARY_PATH"] = str(lib_dir) if not current else f"{lib_dir}{os.pathsep}{current}"
    return tool, env


def run_batch(
    tool: Path,
    args: list[str],
    images: list[Path],
    root: Path,
    env: dict[str, str] | None,
) -> float:
    start = time.perf_counter()
    for image in images:
        subprocess.run(
            [str(tool), *args, str(image.relative_to(root))],
            cwd=root,
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=True,
        )
    return time.perf_counter() - start


def summarize(times: list[float]) -> dict[str, float]:
    return {
        "min_s": min(times),
        "median_s": statistics.median(times),
        "mean_s": statistics.mean(times),
        "max_s": max(times),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go-djpeg", default="/tmp/djpeg_go_perf", help="path to built Go djpeg")
    parser.add_argument("--images", default="tests/testdata/random100/*.jpg", help="glob relative to repo root")
    parser.add_argument("--mode", choices=("default", "nosmooth", "both"), default="both")
    parser.add_argument("--warmup", type=int, default=1)
    parser.add_argument("--repeat", type=int, default=5)
    parser.add_argument("--json", help="write machine-readable results")
    args = parser.parse_args()

    root = repo_root()
    c_tool, c_env = c_reference(root)
    go_tool = Path(args.go_djpeg)
    if not go_tool.exists():
        raise SystemExit(f"Go djpeg not found: {go_tool}")

    images = sorted(root.glob(args.images))
    if not images:
        raise SystemExit(f"no images matched: {args.images}")

    modes = ["default", "nosmooth"] if args.mode == "both" else [args.mode]
    payload = {
        "c_djpeg": str(c_tool),
        "go_djpeg": str(go_tool),
        "image_count": len(images),
        "warmup": args.warmup,
        "repeat": args.repeat,
        "modes": [],
    }

    print(f"C djpeg:  {c_tool}")
    print(f"Go djpeg: {go_tool}")
    print(f"Images:   {args.images} ({len(images)} found)")

    for mode_name in modes:
        mode = MODES[mode_name]
        for _ in range(args.warmup):
            run_batch(c_tool, mode["c_args"], images, root, c_env)
            run_batch(go_tool, mode["go_args"], images, root, None)

        c_times = []
        go_times = []
        for _ in range(args.repeat):
            c_times.append(run_batch(c_tool, mode["c_args"], images, root, c_env))
            go_times.append(run_batch(go_tool, mode["go_args"], images, root, None))

        c_summary = summarize(c_times)
        go_summary = summarize(go_times)
        ratio = go_summary["median_s"] / c_summary["median_s"]
        result = {
            "mode": mode_name,
            "c_times_s": c_times,
            "go_times_s": go_times,
            "c": c_summary,
            "go": go_summary,
            "go_vs_c_median_ratio": ratio,
        }
        payload["modes"].append(result)

        print()
        print(f"Mode: {mode_name}")
        print(f"  C median:  {c_summary['median_s']:.6f}s")
        print(f"  Go median: {go_summary['median_s']:.6f}s")
        print(f"  Go/C:      {ratio:.2f}x")

    if args.json:
        Path(args.json).write_text(json.dumps(payload, indent=2), encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
