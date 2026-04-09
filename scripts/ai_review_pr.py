#!/usr/bin/env python3
import argparse
import concurrent.futures
import json
import os
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path


SCRIPT_REPO_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SCRIPT_REPO_ROOT
PROMPT_PATH = SCRIPT_REPO_ROOT / "scripts" / "ai_review_prompt.md"
SCHEMA_PATH = SCRIPT_REPO_ROOT / "scripts" / "ai_review_schema.json"

REVIEW_EXTENSIONS = {
    ".go",
    ".sql",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".py",
    ".sh",
    ".yml",
    ".yaml",
}
REVIEW_NAMES = {
    "Dockerfile",
    ".dockerignore",
    ".gitignore",
    "go.mod",
    "package.json",
    "package-lock.json",
}
SKIP_PATH_PARTS = {
    "node_modules",
    "dist",
    "build",
    "coverage",
    ".git",
}
SKIP_SUFFIXES = (
    ".sum",
    ".lock",
    ".csv",
    ".parquet",
    ".zip",
    ".gz",
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".ico",
    ".pdf",
)


@dataclass(frozen=True)
class AddedLine:
    line: int
    code: str


@dataclass(frozen=True)
class FileDiff:
    path: str
    diff: str
    added_lines: tuple[AddedLine, ...]


def run(cmd, *, input_text=None, timeout=120):
    return subprocess.run(
        cmd,
        cwd=REPO_ROOT,
        input=input_text,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )


def load_diff(base):
    result = run(["git", "diff", "--no-ext-diff", "--unified=20", f"{base}...HEAD"], timeout=60)
    if result.returncode != 0:
        raise RuntimeError(f"git diff failed: {result.stderr.strip()}")
    return result.stdout


def parse_diff(diff_text):
    files = []
    current_path = None
    buffer = []
    added = []
    new_line = None

    def flush():
        if current_path and buffer:
            files.append(FileDiff(current_path, "\n".join(buffer) + "\n", tuple(added)))

    for raw_line in diff_text.splitlines():
        if raw_line.startswith("diff --git "):
            flush()
            current_path = raw_line.split(" b/", 1)[-1]
            buffer = [raw_line]
            added = []
            new_line = None
            continue

        if current_path is None:
            continue

        buffer.append(raw_line)

        if raw_line.startswith("+++ ") or raw_line.startswith("--- "):
            continue

        if raw_line.startswith("@@ "):
            # Example: @@ -10,6 +20,9 @@ func name() {
            header_parts = raw_line.split(" ")
            new_range = next((part for part in header_parts if part.startswith("+")), None)
            if not new_range:
                new_line = None
                continue
            start = new_range[1:].split(",", 1)[0]
            new_line = int(start)
            continue

        if new_line is None:
            continue

        if raw_line.startswith("+"):
            added.append(AddedLine(new_line, raw_line[1:]))
            new_line += 1
        elif raw_line.startswith("-"):
            continue
        else:
            new_line += 1

    flush()
    return files


def should_review(file_diff):
    path = file_diff.path
    parts = set(Path(path).parts)
    name = Path(path).name
    suffix = Path(path).suffix

    if not file_diff.added_lines:
        return False
    if parts & SKIP_PATH_PARTS:
        return False
    if path.startswith("data/") or path.startswith("research/dataset/"):
        return False
    if path.endswith(SKIP_SUFFIXES) and name not in REVIEW_NAMES:
        return False
    return suffix in REVIEW_EXTENSIONS or name in REVIEW_NAMES or path.startswith(".github/workflows/")


def trim_diff(text, max_chars):
    if len(text) <= max_chars:
        return text
    return text[:max_chars] + "\n\n[diff truncated by ai_review_pr.py]\n"


def build_review_prompt(file_diff, max_diff_chars):
    base_prompt = PROMPT_PATH.read_text(encoding="utf-8")
    candidate_lines = "\n".join(
        f"ADD line={line.line}: {line.code}" for line in file_diff.added_lines
    )
    return (
        f"{base_prompt}\n\n"
        f"Current file: {file_diff.path}\n\n"
        f"ADD candidate lines:\n{candidate_lines}\n\n"
        f"Full file diff:\n```diff\n{trim_diff(file_diff.diff, max_diff_chars)}```\n"
    )


def call_codex(file_diff, args):
    prompt = build_review_prompt(file_diff, args.max_diff_chars)
    allowed_lines = {line.line for line in file_diff.added_lines}

    with tempfile.TemporaryDirectory(prefix="ai-review-") as tmpdir:
        output_path = Path(tmpdir) / "last-message.json"
        cmd = [
            "codex",
            "exec",
            "--ephemeral",
            "--sandbox",
            "read-only",
            "--color",
            "never",
            "--output-schema",
            str(SCHEMA_PATH),
            "--output-last-message",
            str(output_path),
            "-",
        ]
        result = run(cmd, input_text=prompt, timeout=args.codex_timeout)
        if result.returncode != 0:
            return [], f"{file_diff.path}: codex failed: {result.stderr.strip() or result.stdout.strip()}"
        if not output_path.exists():
            return [], f"{file_diff.path}: codex did not write an output message"

        try:
            payload = json.loads(output_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            return [], f"{file_diff.path}: invalid JSON from codex: {exc}"

    comments = []
    for item in payload.get("comments", []):
        line = item.get("line")
        message = str(item.get("message", "")).strip()
        severity = str(item.get("severity", "warning")).strip().lower()
        if line not in allowed_lines:
            continue
        if not message:
            continue
        if severity not in {"critical", "warning", "suggestion"}:
            severity = "warning"
        comments.append(
            {
                "path": file_diff.path,
                "line": line,
                "side": "RIGHT",
                "severity": severity,
                "message": message,
            }
        )
    return comments[: args.max_comments_per_file], None


def review_file(file_diff, args):
    if args.dry_run:
        return [], None
    return call_codex(file_diff, args)


def write_json(path, payload):
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_summary(path, reviewed_files, skipped, comments, warnings):
    reviewed_count = len(reviewed_files)
    candidate_line_count = sum(len(file_diff.added_lines) for file_diff in reviewed_files)
    comment_count = len(comments)
    conclusion = (
        f"已生成 {comment_count} 条通过 diff 校验的行级评论。"
        if comment_count
        else "本轮未发现需要发布行级评论的高置信度问题。"
    )
    lines = [
        "## AI 代码审查摘要",
        "",
        f"- 结论：{conclusion}",
        f"- 已审查文件：{reviewed_count}",
        f"- 候选新增行：{candidate_line_count}",
        f"- 已跳过文件：{skipped}",
        f"- 行级评论：{comment_count}",
        f"- 审查方式：按文件调用 Codex，只发布绑定到 PR 新增行的评论",
    ]
    if reviewed_files:
        lines.append("")
        lines.append("### 本轮审查范围")
        lines.extend(f"- `{file_diff.path}`" for file_diff in reviewed_files[:20])
        if reviewed_count > 20:
            lines.append(f"- ... 另有 {reviewed_count - 20} 个文件")
    if warnings:
        lines.append("")
        lines.append("### 审查器警告")
        lines.extend(f"- {warning}" for warning in warnings[:10])
    Path(path).parent.mkdir(parents=True, exist_ok=True)
    Path(path).write_text("\n".join(lines) + "\n", encoding="utf-8")


def parse_args():
    parser = argparse.ArgumentParser(description="Run project-specific Codex inline review for a PR.")
    parser.add_argument("--repo-root", default=os.environ.get("AI_REVIEW_REPO_ROOT", str(REPO_ROOT)))
    parser.add_argument("--base", default=os.environ.get("AI_REVIEW_BASE", "origin/main"))
    parser.add_argument("--output", default=".ai-review/review-comments.json")
    parser.add_argument("--summary", default=".ai-review/summary.md")
    parser.add_argument("--max-workers", type=int, default=int(os.environ.get("AI_REVIEW_MAX_WORKERS", "1")))
    parser.add_argument("--max-files", type=int, default=int(os.environ.get("AI_REVIEW_MAX_FILES", "30")))
    parser.add_argument("--max-total-comments", type=int, default=int(os.environ.get("AI_REVIEW_MAX_TOTAL_COMMENTS", "20")))
    parser.add_argument("--max-comments-per-file", type=int, default=int(os.environ.get("AI_REVIEW_MAX_COMMENTS_PER_FILE", "5")))
    parser.add_argument("--max-diff-chars", type=int, default=int(os.environ.get("AI_REVIEW_MAX_DIFF_CHARS", "18000")))
    parser.add_argument("--codex-timeout", type=int, default=int(os.environ.get("AI_REVIEW_CODEX_TIMEOUT", "240")))
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main():
    global REPO_ROOT
    args = parse_args()
    REPO_ROOT = Path(args.repo_root).resolve()
    args.max_workers = max(1, min(args.max_workers, 2))

    diff_text = load_diff(args.base)
    all_file_diffs = parse_diff(diff_text)
    reviewable = [file for file in all_file_diffs if should_review(file)]
    reviewable = reviewable[: args.max_files]

    comments = []
    warnings = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.max_workers) as executor:
        future_map = {executor.submit(review_file, file_diff, args): file_diff for file_diff in reviewable}
        for future in concurrent.futures.as_completed(future_map):
            file_comments, warning = future.result()
            comments.extend(file_comments)
            if warning:
                warnings.append(warning)

    comments = sorted(comments, key=lambda item: (item["path"], item["line"]))[: args.max_total_comments]
    skipped = max(0, len(all_file_diffs) - len(reviewable))

    write_json(args.output, {"comments": comments, "warnings": warnings})
    write_summary(args.summary, reviewable, skipped, comments, warnings)

    print(f"reviewed_files={len(reviewable)}")
    print(f"skipped_files={skipped}")
    print(f"comments={len(comments)}")
    print(f"warnings={len(warnings)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
