#!/usr/bin/env python3
"""Check the versioned project memory without importing application code.

This is documentation consistency tooling. It intentionally uses only the Python standard
library and read-only git/filesystem operations.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[1]
MEMORY = ROOT / "docs" / "memory"
REQUIRED_FILES = {
    "README.md",
    "PROJECT-MEMORY.md",
    "CURRENT-STATE.md",
    "ARCHITECTURE-MAP.md",
    "DECISIONS.md",
    "OPEN-QUESTIONS.md",
    "IMPLEMENTATION-STATUS.md",
    "COMPATIBILITY-MATRIX.md",
    "PRODUCTION-RUNBOOK.md",
    "SECURITY-CONTROLS.md",
    "TEST-EVIDENCE.md",
    "CHANGELOG.md",
}
DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
TASK_RE = re.compile(r"^###\s+(T\d{3})\s+—", re.MULTILINE)
MEMORY_TASK_RE = re.compile(r"^\|\s*(T\d{3})\s*\|", re.MULTILINE)
OQ_HEADING_RE = re.compile(r"^##\s+(OQ-\d{2})\s+—", re.MULTILINE)
MEMORY_OQ_RE = re.compile(
    r"^\|\s*(OQ-\d{2})\s*\|\s*(RESOLVED|DEFERRED-NONBLOCKING|OBSOLETE)\s*\|",
    re.MULTILINE,
)
DECISION_RE = re.compile(r"^\|\s*(D-[A-Z0-9-]+)\s*\|", re.MULTILINE)
PLAN_FILES = (
    ROOT / "docs" / "SETUP-PLAN.md",
    ROOT / "docs" / "PRODUCTION-PLAN.md",
    ROOT / "docs" / "PRODUCTION-EXECUTION-PROMPT.md",
)
PLAN_MARKERS = {
    "SETUP-PLAN.md": ("V0", "V10", "clean-room", "Local/LAN", "Tauri 2"),
    "PRODUCTION-PLAN.md": ("Phase 0", "Phase 3", "Phase 7", "Phase 8", "Local/LAN", "canary"),
    "PRODUCTION-EXECUTION-PROMPT.md": ("AGENTS.md", "Movie Narrator chỉ là research/reference", "Go Product API", "Handoff:"),
}
REQUIRED_SPECS = (
    ROOT / "docs" / "09-INDEPENDENT-IMPLEMENTATION-FROM-REFERENCE.md",
    ROOT / "docs" / "UPSTREAM-REFERENCE-POLICY.md",
    ROOT / "docs" / "UPSTREAM-CAPABILITY-MATRIX.md",
    ROOT / "docs" / "UPSTREAM-MODULE-AUDIT.md",
)


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def frontmatter(path: Path) -> dict[str, str]:
    lines = read(path).splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    try:
        end = lines.index("---", 1)
    except ValueError:
        return {}
    values: dict[str, str] = {}
    for line in lines[1:end]:
        if ":" in line:
            key, value = line.split(":", 1)
            values[key.strip()] = value.strip()
    return values


def git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args], cwd=ROOT, check=True, capture_output=True, text=True
    )
    return result.stdout.strip()


def check_links(errors: list[str], paths: list[Path]) -> None:
    for path in paths:
        for raw_target in LINK_RE.findall(read(path)):
            target = raw_target.strip().strip("<>").split("#", 1)[0]
            if not target or target.startswith(("http://", "https://", "mailto:")):
                continue
            target_path = (path.parent / unquote(target)).resolve()
            if not target_path.exists():
                errors.append(f"{path.relative_to(ROOT)}: missing internal link target {target}")


def table_pipe_count(line: str) -> int:
    """Count Markdown table delimiters while ignoring pipes inside inline code."""
    count = 0
    in_code = False
    index = 0
    while index < len(line):
        if line[index] == "`":
            run = 1
            while index + run < len(line) and line[index + run] == "`":
                run += 1
            in_code = not in_code
            index += run
            continue
        if line[index] == "|" and not in_code and (index == 0 or line[index - 1] != "\\"):
            count += 1
        index += 1
    return count


def check_markdown_tables(errors: list[str], paths: list[Path]) -> None:
    delimiter = re.compile(r"^\s*\|(?:\s*:?-{3,}:?\s*\|)+\s*$")
    for path in paths:
        lines = read(path).splitlines()
        for index, header in enumerate(lines[:-1]):
            if not header.lstrip().startswith("|") or not delimiter.match(lines[index + 1]):
                continue
            expected = table_pipe_count(header)
            row = index + 2
            while row < len(lines) and lines[row].lstrip().startswith("|"):
                actual = table_pipe_count(lines[row])
                if actual != expected:
                    errors.append(
                        f"{path.relative_to(ROOT)}:{row + 1}: Markdown table column mismatch "
                        f"({actual} delimiters, expected {expected})"
                    )
                row += 1


def check_json_fences(errors: list[str], paths: list[Path]) -> None:
    fence = re.compile(r"```(json|jsonl)\s*\n(.*?)\n```", re.DOTALL)
    for path in paths:
        for index, (kind, block) in enumerate(fence.findall(read(path)), 1):
            payloads = [block] if kind == "json" else [line for line in block.splitlines() if line.strip()]
            for payload in payloads:
                try:
                    json.loads(payload)
                except json.JSONDecodeError as exc:
                    errors.append(
                        f"{path.relative_to(ROOT)}: invalid {kind} fence {index}: {exc}"
                    )


def check_metadata(errors: list[str], paths: list[Path], max_age: int) -> None:
    today = dt.date.today()
    required = {"last_verified", "source", "owner"}
    for path in paths:
        meta = frontmatter(path)
        missing = required - meta.keys()
        if missing:
            errors.append(f"{path.relative_to(ROOT)}: missing metadata {', '.join(sorted(missing))}")
            continue
        value = meta["last_verified"]
        if not DATE_RE.match(value):
            errors.append(f"{path.relative_to(ROOT)}: invalid last_verified {value!r}")
            continue
        verified = dt.date.fromisoformat(value)
        age = (today - verified).days
        if age > max_age:
            errors.append(f"{path.relative_to(ROOT)}: stale memory ({age} days old)")
        if not meta["source"] or not meta["owner"]:
            errors.append(f"{path.relative_to(ROOT)}: source and owner must be non-empty")


def check_unique(errors: list[str], values: list[str], label: str) -> None:
    duplicates = sorted({value for value in values if values.count(value) > 1})
    if duplicates:
        errors.append(f"duplicate {label}: {', '.join(duplicates)}")


def check_oqs(errors: list[str]) -> None:
    root = read(ROOT / "docs" / "OPEN-QUESTIONS.md")
    mirror = read(MEMORY / "OPEN-QUESTIONS.md")
    root_ids = OQ_HEADING_RE.findall(root)
    mirror_rows = dict(MEMORY_OQ_RE.findall(mirror))
    expected = sorted(set(root_ids))
    if len(root_ids) != len(expected):
        errors.append(f"root OQ IDs contain duplicates: {root_ids}")
    if sorted(mirror_rows) != expected:
        errors.append(f"memory OQ mirror does not contain exactly {expected}")
    for oq_id in expected:
        section = root.split(f"## {oq_id} —", 1)
        if len(section) != 2:
            continue
        status_match = re.search(r"\*\*Decision status\*\*:\s*([A-Z-]+)", section[1])
        if status_match and mirror_rows.get(oq_id) != status_match.group(1):
            errors.append(f"OQ status mismatch for {oq_id}: root={status_match.group(1)}")


def check_tasks(errors: list[str]) -> None:
    source = read(ROOT / "docs" / "IMPLEMENTATION-ORDER.md")
    memory = read(MEMORY / "IMPLEMENTATION-STATUS.md")
    source_tasks = TASK_RE.findall(source)
    memory_tasks = MEMORY_TASK_RE.findall(memory)
    if len(source_tasks) != len(set(source_tasks)):
        errors.append("implementation order contains duplicate task IDs")
    if len(memory_tasks) != len(set(memory_tasks)):
        errors.append("memory implementation ledger contains duplicate task IDs")
    if source_tasks != memory_tasks:
        missing = sorted(set(source_tasks) - set(memory_tasks))
        extra = sorted(set(memory_tasks) - set(source_tasks))
        errors.append(f"task ledger mismatch; missing={missing}, extra={extra}")
    memory_dependencies: dict[str, set[str]] = {}
    for line_number, line in enumerate(memory.splitlines(), 1):
        if not line.startswith("| T"):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != 6 or not cells[2] or not cells[5]:
            errors.append(
                f"IMPLEMENTATION-STATUS.md:{line_number}: task row lacks dependency/status or next-task data"
            )
            continue
        memory_dependencies[cells[0]] = set(
            re.findall(r"(?:T\d{3}|OQ-\d{2})", cells[2])
        )
    matches = list(TASK_RE.finditer(source))
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(source)
        section = source[match.start():end]
        dependency = re.search(r"^- \*\*Dependencies:\*\*\s*(.+)$", section, re.MULTILINE)
        if not dependency:
            errors.append(f"{match.group(1)} has no dependency declaration")
            continue
        source_dependencies = set(
            re.findall(r"(?:T\d{3}|OQ-\d{2})", dependency.group(1))
        )
        if memory_dependencies.get(match.group(1), set()) != source_dependencies:
            errors.append(
                f"{match.group(1)} dependency mirror mismatch; "
                f"source={sorted(source_dependencies)}, "
                f"memory={sorted(memory_dependencies.get(match.group(1), set()))}"
            )
        for dependency_id in re.findall(r"T\d{3}", dependency.group(1)):
            if int(dependency_id[1:]) >= int(match.group(1)[1:]):
                errors.append(
                    f"{match.group(1)} depends on non-prior task {dependency_id}; review task order"
                )


def check_canonical_terms(errors: list[str]) -> None:
    text = read(MEMORY / "PROJECT-MEMORY.md")
    section = text.split("## Canonical vocabulary", 1)
    if len(section) != 2:
        errors.append("PROJECT-MEMORY.md has no canonical vocabulary section")
        return
    body = section[1].split("## ", 1)[0]
    terms = [
        match.group(1).strip()
        for match in re.finditer(r"^\|\s*([^|]+?)\s*\|", body, re.MULTILINE)
        if match.group(1).strip() != "Term"
    ]
    check_unique(errors, terms, "canonical terms")


def check_plans(errors: list[str], max_age: int) -> None:
    missing = [str(path.relative_to(ROOT)) for path in PLAN_FILES if not path.exists()]
    if missing:
        errors.append(f"missing plan files: {', '.join(missing)}")
        return
    check_metadata(errors, list(PLAN_FILES), max_age)
    check_links(errors, list(PLAN_FILES))
    for path in PLAN_FILES:
        content = read(path)
        for marker in PLAN_MARKERS[path.name]:
            if marker not in content:
                errors.append(f"{path.relative_to(ROOT)}: missing required marker {marker!r}")


def check_branch_commit(errors: list[str]) -> None:
    branch = git("branch", "--show-current")
    state = read(MEMORY / "CURRENT-STATE.md")
    if branch != "docs/architecture-spec":
        errors.append(f"expected current branch docs/architecture-spec, got {branch!r}")
    commit_match = re.search(r"\| Baseline commit \| `([0-9a-f]{7,40})`", state)
    if not commit_match:
        errors.append("CURRENT-STATE.md has no valid baseline commit")
        return
    try:
        git("cat-file", "-e", f"{commit_match.group(1)}^{{commit}}")
    except subprocess.CalledProcessError:
        errors.append(f"baseline commit does not exist: {commit_match.group(1)}")


def check_sensitive_content(errors: list[str], paths: list[Path]) -> None:
    secret_assignment = re.compile(
        r"(?i)\b(?:api[_-]?key|password|secret|token|access[_-]?key)\s*[:=]\s*['\"]?[A-Za-z0-9+/=_-]{12,}"
    )
    local_path = re.compile(r"(?<![A-Za-z0-9])/(?:home|root|tmp|var|mnt|workspace)/")
    for path in paths:
        for number, line in enumerate(read(path).splitlines(), 1):
            if secret_assignment.search(line):
                errors.append(f"{path.relative_to(ROOT)}:{number}: possible secret assignment")
            if local_path.search(line):
                errors.append(f"{path.relative_to(ROOT)}:{number}: durable local path found")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--max-age", type=int, default=30, help="maximum memory age in days")
    args = parser.parse_args()

    errors: list[str] = []
    if not MEMORY.is_dir():
        print("ERROR: docs/memory does not exist", file=sys.stderr)
        return 1
    actual = {path.name for path in MEMORY.glob("*.md")}
    if actual != REQUIRED_FILES:
        errors.append(
            f"memory file set mismatch; missing={sorted(REQUIRED_FILES - actual)}, "
            f"extra={sorted(actual - REQUIRED_FILES)}"
        )
    paths = sorted(MEMORY.glob("*.md"))
    missing_specs = [str(path.relative_to(ROOT)) for path in REQUIRED_SPECS if not path.exists()]
    if missing_specs:
        errors.append(f"missing independent-architecture specs: {', '.join(missing_specs)}")
    if (ROOT / "docs" / "09-MIGRATION-FROM-UPSTREAM.md").exists():
        errors.append("obsolete docs/09-MIGRATION-FROM-UPSTREAM.md still exists")
    check_metadata(errors, paths, args.max_age)
    markdown_paths = sorted(ROOT.glob("*.md")) + sorted((ROOT / "docs").rglob("*.md"))
    check_links(errors, markdown_paths)
    check_markdown_tables(errors, markdown_paths)
    check_json_fences(errors, markdown_paths)
    check_oqs(errors)
    check_tasks(errors)
    check_canonical_terms(errors)
    check_plans(errors, args.max_age)
    check_branch_commit(errors)
    check_sensitive_content(errors, paths)
    check_unique(errors, DECISION_RE.findall(read(MEMORY / "DECISIONS.md")), "decision IDs")

    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1
    print(
        f"memory check passed: {len(paths)} files, {len(OQ_HEADING_RE.findall(read(ROOT / 'docs' / 'OPEN-QUESTIONS.md')))} OQs, "
        f"{len(MEMORY_TASK_RE.findall(read(MEMORY / 'IMPLEMENTATION-STATUS.md')))} tasks"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
