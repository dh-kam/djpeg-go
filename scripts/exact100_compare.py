#!/usr/bin/env python3
"""Compare C djpeg and Go djpeg output on the random100 corpus.

The comparison parses raw P5/P6 PNM and compares pixel payloads instead of
whole files, so harmless header formatting differences do not affect parity.
"""

from __future__ import annotations

import argparse
import json
import os
import shlex
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class PNM:
    magic: bytes
    width: int
    height: int
    maxval: int
    channels: int
    pixels: bytes

    @property
    def pixel_count(self) -> int:
        return self.width * self.height


@dataclass(frozen=True)
class Tool:
    argv: list[str]
    env: dict[str, str] | None = None
    description: str = ""


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


def is_executable(path: Path) -> bool:
    return path.is_file() and os.access(path, os.X_OK)


def reject_known_contaminated(path: Path, root: Path) -> None:
    try:
        resolved = path.resolve()
        contaminated = (root / "jpeg-6b" / "djpeg").resolve()
    except OSError:
        return
    if resolved == contaminated:
        raise SystemExit(
            f"Refusing to use {path}: jpeg-6b/djpeg is known to contaminate stdout."
        )


def select_c_djpeg(root: Path, explicit: str | None) -> Tool:
    if explicit:
        path = Path(explicit)
        reject_known_contaminated(path, root)
        if not is_executable(path):
            raise SystemExit(f"C djpeg is not executable: {path}")
        return Tool([str(path)], description=str(path))

    local_9f = root / "jpeg-9f" / ".libs" / "djpeg"
    if is_executable(local_9f):
        libdir = local_9f.parent
        env = os.environ.copy()
        old_ld = env.get("LD_LIBRARY_PATH")
        env["LD_LIBRARY_PATH"] = f"{libdir}:{old_ld}" if old_ld else str(libdir)
        return Tool(
            [str(local_9f)],
            env=env,
            description=f"{local_9f} (LD_LIBRARY_PATH={libdir})",
        )

    system = shutil.which("djpeg")
    if not system:
        raise SystemExit(
            "No usable C djpeg found. Build jpeg-9f/.libs/djpeg or install djpeg."
        )
    system_path = Path(system)
    reject_known_contaminated(system_path, root)
    return Tool([system], description=system)


def select_go_djpeg(root: Path, explicit: str | None) -> Tool:
    if explicit:
        argv = shlex.split(explicit)
        if not argv:
            raise SystemExit("--go-djpeg cannot be empty")
        return Tool(argv, description=explicit)

    local = root / "djpeg_go"
    if is_executable(local):
        return Tool([str(local)], description=str(local))

    if shutil.which("go"):
        return Tool(["go", "run", "./cmd/djpeg"], description="go run ./cmd/djpeg")

    raise SystemExit(
        "No Go djpeg found. Build ./djpeg_go or install Go for 'go run ./cmd/djpeg'."
    )


def run_tool(tool: Tool, args: list[str], cwd: Path) -> bytes:
    proc = subprocess.run(
        [*tool.argv, *args],
        cwd=cwd,
        env=tool.env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if proc.returncode != 0:
        stderr = proc.stderr.decode("utf-8", errors="replace").strip()
        cmd = shlex.join([*tool.argv, *args])
        raise RuntimeError(f"{cmd} exited {proc.returncode}: {stderr}")
    return proc.stdout


def skip_ws_and_comments(data: bytes, pos: int) -> int:
    while pos < len(data):
        c = data[pos]
        if c in b" \t\r\n\f\v":
            pos += 1
            continue
        if c == ord("#"):
            newline = data.find(b"\n", pos)
            if newline == -1:
                return len(data)
            pos = newline + 1
            continue
        break
    return pos


def read_token(data: bytes, pos: int) -> tuple[bytes, int]:
    pos = skip_ws_and_comments(data, pos)
    start = pos
    while pos < len(data) and data[pos] not in b" \t\r\n\f\v#":
        pos += 1
    if start == pos:
        raise ValueError("missing PNM header token")
    return data[start:pos], pos


def parse_pnm(data: bytes) -> PNM:
    magic, pos = read_token(data, 0)
    if magic not in (b"P5", b"P6"):
        preview = data[:24].decode("utf-8", errors="replace")
        raise ValueError(f"expected P5/P6 PNM output, got {preview!r}")

    width_b, pos = read_token(data, pos)
    height_b, pos = read_token(data, pos)
    maxval_b, pos = read_token(data, pos)
    try:
        width = int(width_b)
        height = int(height_b)
        maxval = int(maxval_b)
    except ValueError as exc:
        raise ValueError("PNM width, height, and maxval must be integers") from exc

    if width <= 0 or height <= 0:
        raise ValueError(f"invalid PNM dimensions {width}x{height}")
    if not (0 < maxval <= 255):
        raise ValueError(f"unsupported PNM maxval {maxval}; only 8-bit PNM is supported")

    if pos >= len(data) or data[pos] not in b" \t\r\n\f\v":
        raise ValueError("PNM header is not followed by required whitespace")
    pos += 1

    channels = 1 if magic == b"P5" else 3
    expected = width * height * channels
    pixels = data[pos : pos + expected]
    if len(pixels) != expected:
        raise ValueError(f"truncated PNM pixel data: got {len(pixels)}, want {expected}")
    if len(data) != pos + expected:
        raise ValueError(
            f"PNM output has {len(data) - (pos + expected)} trailing byte(s)"
        )

    return PNM(magic, width, height, maxval, channels, pixels)


def compare_pixels(c_pnm: PNM, go_pnm: PNM) -> tuple[bool, str, dict[str, object]]:
    c_shape = (c_pnm.magic, c_pnm.width, c_pnm.height, c_pnm.maxval)
    go_shape = (go_pnm.magic, go_pnm.width, go_pnm.height, go_pnm.maxval)
    if c_shape != go_shape:
        stats = {
            "exact": False,
            "error": "shape mismatch",
            "c_shape": {
                "magic": c_pnm.magic.decode(),
                "width": c_pnm.width,
                "height": c_pnm.height,
                "maxval": c_pnm.maxval,
            },
            "go_shape": {
                "magic": go_pnm.magic.decode(),
                "width": go_pnm.width,
                "height": go_pnm.height,
                "maxval": go_pnm.maxval,
            },
        }
        return False, (
            "shape mismatch: "
            f"C {c_pnm.magic.decode()} {c_pnm.width}x{c_pnm.height} max={c_pnm.maxval}, "
            f"Go {go_pnm.magic.decode()} {go_pnm.width}x{go_pnm.height} max={go_pnm.maxval}"
        ), stats

    channels = c_pnm.channels
    total_pixels = c_pnm.pixel_count
    matched_pixels = 0
    first_diff = -1

    for idx in range(total_pixels):
        start = idx * channels
        end = start + channels
        if c_pnm.pixels[start:end] == go_pnm.pixels[start:end]:
            matched_pixels += 1
        elif first_diff == -1:
            first_diff = idx

    matched_bytes = sum(
        1 for c_byte, go_byte in zip(c_pnm.pixels, go_pnm.pixels) if c_byte == go_byte
    )
    exact = matched_pixels == total_pixels
    summary = (
        f"pixels {matched_pixels}/{total_pixels}, "
        f"bytes {matched_bytes}/{len(c_pnm.pixels)}"
    )
    if first_diff != -1:
        x = first_diff % c_pnm.width
        y = first_diff // c_pnm.width
        start = first_diff * channels
        end = start + channels
        summary += f", first diff x={x} y={y} C={list(c_pnm.pixels[start:end])} Go={list(go_pnm.pixels[start:end])}"
        first_diff_record = {
            "x": x,
            "y": y,
            "c": list(c_pnm.pixels[start:end]),
            "go": list(go_pnm.pixels[start:end]),
        }
    else:
        first_diff_record = None

    stats = {
        "width": c_pnm.width,
        "height": c_pnm.height,
        "channels": channels,
        "exact": exact,
        "matched_pixels": matched_pixels,
        "total_pixels": total_pixels,
        "matched_samples": matched_bytes,
        "total_samples": len(c_pnm.pixels),
        "first_diff": first_diff_record,
    }
    return exact, summary, stats


def image_inputs(root: Path, pattern: str) -> list[Path]:
    matches = sorted(root.glob(pattern))
    if not matches:
        raise SystemExit(f"No images matched {pattern!r}")
    return matches


def empty_summary() -> dict[str, int]:
    return {
        "images": 0,
        "exact_images": 0,
        "failed": 0,
        "matched_pixels": 0,
        "total_pixels": 0,
        "matched_samples": 0,
        "total_samples": 0,
    }


def run_mode(
    mode_name: str,
    c_tool: Tool,
    go_tool: Tool,
    images: list[Path],
    root: Path,
    quiet_ok: bool,
) -> tuple[dict[str, object], int]:
    mode = MODES[mode_name]
    summary = empty_summary()
    results: list[dict[str, object]] = []

    print(f"Mode:     {mode_name}")
    print("-----------------------------------")

    for index, image in enumerate(images, start=1):
        rel = image.relative_to(root)
        record: dict[str, object] = {
            "index": index,
            "image": str(rel),
        }
        try:
            c_out = run_tool(c_tool, [*mode["c_args"], str(rel)], root)
            go_out = run_tool(go_tool, [*mode["go_args"], str(rel)], root)
            c_pnm = parse_pnm(c_out)
            go_pnm = parse_pnm(go_out)
            exact, detail, stats = compare_pixels(c_pnm, go_pnm)
            record.update(stats)
        except Exception as exc:
            summary["failed"] += 1
            record.update({"exact": False, "error": str(exc)})
            results.append(record)
            print(f"FAIL {index:03d} {rel}: {exc}")
            continue

        summary["images"] += 1
        summary["matched_pixels"] += int(record["matched_pixels"])
        summary["total_pixels"] += int(record["total_pixels"])
        summary["matched_samples"] += int(record["matched_samples"])
        summary["total_samples"] += int(record["total_samples"])

        if exact:
            summary["exact_images"] += 1
            if not quiet_ok or summary["exact_images"] % 10 == 0:
                print(f"OK   {index:03d} {rel}: {detail}")
        else:
            summary["failed"] += 1
            print(f"DIFF {index:03d} {rel}: {detail}")

        results.append(record)

    print("-----------------------------------")
    print(f"Parity Test Summary ({mode_name})")
    print(f"Total images:     {len(images)}")
    print(f"Exact pixel match:{summary['exact_images']:6d}")
    print(f"Failed/different: {summary['failed']}")
    print("")

    return {
        "name": mode_name,
        "c_args": mode["c_args"],
        "go_args": mode["go_args"],
        "summary": summary,
        "results": results,
    }, summary["failed"]


def main() -> int:
    root = repo_root()
    parser = argparse.ArgumentParser(
        description="Exact-100 pixel comparison harness for C djpeg vs Go djpeg."
    )
    parser.add_argument(
        "--images",
        default="tests/testdata/random100/*.jpg",
        help="glob of JPEG inputs, relative to the repo root by default",
    )
    parser.add_argument(
        "--expected-count",
        type=int,
        default=100,
        help="expected image count for exact-100 parity",
    )
    parser.add_argument("--c-djpeg", help="explicit C djpeg executable")
    parser.add_argument(
        "--go-djpeg",
        help="explicit Go djpeg command; quote if it contains arguments",
    )
    parser.add_argument(
        "--mode",
        choices=("default", "nosmooth", "both"),
        default="nosmooth",
        help="comparison mode; default keeps the historical nosmooth behavior",
    )
    parser.add_argument(
        "--json",
        dest="json_path",
        help="write machine-readable summary and per-image results to this path",
    )
    parser.add_argument(
        "--quiet-ok",
        action="store_true",
        help="only print mismatches and every tenth successful image",
    )
    args = parser.parse_args()

    c_tool = select_c_djpeg(root, args.c_djpeg)
    go_tool = select_go_djpeg(root, args.go_djpeg)
    images = image_inputs(root, args.images)

    print(f"C djpeg:  {c_tool.description}")
    print(f"Go djpeg: {go_tool.description}")
    print(f"Images:   {args.images} ({len(images)} found)")

    mode_names = ["default", "nosmooth"] if args.mode == "both" else [args.mode]
    mode_results = []
    failed = 0
    for mode_name in mode_names:
        result, mode_failed = run_mode(
            mode_name, c_tool, go_tool, images, root, args.quiet_ok
        )
        mode_results.append(result)
        failed += mode_failed

    payload = {
        "c_djpeg": c_tool.description,
        "go_djpeg": go_tool.description,
        "images": args.images,
        "expected_count": args.expected_count,
        "modes": mode_results,
    }
    if args.json_path:
        Path(args.json_path).write_text(
            json.dumps(payload, indent=2, sort_keys=True), encoding="utf-8"
        )

    all_exact = all(
        result["summary"]["exact_images"] == args.expected_count
        for result in mode_results
    )
    if all_exact and len(images) == args.expected_count and failed == 0:
        print("SUCCESS: exact-100 pixel parity achieved.")
        return 0

    print("FAILURE: exact-100 pixel parity not met.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
