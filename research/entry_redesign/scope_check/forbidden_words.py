"""Forbidden words ripgrep scanner for entry_redesign scope check.

Scans requirements.md / design.md / tasks.md / PR diff for forbidden literal
strings defined in Requirements 5.4, 7.2, 7.3, 7.5, 7.6, 7.8, 7.9, and 1.4/1.5.

Case-insensitive fixed-string matching (equivalent to `rg -F -i -n`).
"""

from __future__ import annotations

import subprocess
import shutil
from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class ScanHit:
    """A single forbidden word match."""

    file_path: str
    line_number: int
    matched_word: str
    line_content: str


# ---------------------------------------------------------------------------
# Forbidden words collected from requirements
# ---------------------------------------------------------------------------

# Requirement 1.4: closed-set breakout semantics (4 literals)
_REQ_1_4_FORBIDDEN: list[str] = [
    "闭合 signal bar 收盘确认",
    "1h close confirm",
    "close > prev_high_2 后入场",
    "1h K 线收盘确认突破",
]

# Requirement 1.5: removed Go live-aligned replay module (4 literals)
_REQ_1_5_FORBIDDEN: list[str] = [
    "go-live-replay",
    "live_aligned_replay",
    "LiveAlignedReplay",
    "GoLiveAlignedReplay",
]

# Requirement 5.4: R0 ripgrep pattern expanded to fixed-string literals
# Original regex: (live[_ -]?migration|control[_ -]?reset|auto[_ -]?dispatch|
#   sleeve[_ -]?multiplier|dispatchMode|session\.config|
#   gate[_ -]?改造|event[_ -]?source[_ -]?替换|R[3-5]\b)
# We expand each regex alternation into its literal variants for -F matching.
_REQ_5_4_FORBIDDEN: list[str] = [
    "live_migration",
    "live-migration",
    "live migration",
    "livemigration",
    "control_reset",
    "control-reset",
    "control reset",
    "controlreset",
    "auto_dispatch",
    "auto-dispatch",
    "auto dispatch",
    "autodispatch",
    "sleeve_multiplier",
    "sleeve-multiplier",
    "sleeve multiplier",
    "sleevemultiplier",
    "dispatchMode",
    "session.config",
    "gate_改造",
    "gate-改造",
    "gate 改造",
    "gate改造",
    "event_source_替换",
    "event-source-替换",
    "event source 替换",
    "eventsource替换",
    "event_source替换",
    "event-source替换",
]

# Requirement 7.2: gate modification forbidden
_REQ_7_2_FORBIDDEN: list[str] = [
    "candidate_001 gate 阈值调整",
    "rolling regime augment",
    "gate_sensitivity_grid 再设计",
    "candidate_002",
]

# Requirement 7.3: ML model forbidden
_REQ_7_3_FORBIDDEN: list[str] = [
    "GRU model training",
    "Transformer entry filter",
    "sequence_model_training",
    "LSTM entry model",
]

# Requirement 7.5: event source replacement forbidden
_REQ_7_5_FORBIDDEN: list[str] = [
    "prev_high_8 entry",
    "prev_low_8 entry",
    "donchian_confirm entry",
    "custom breakout shape replacing original_t2",
]

# Requirement 7.6: feature engineering forbidden
_REQ_7_6_FORBIDDEN: list[str] = [
    "tick-level order flow features",
    "L2 orderbook reconstruction",
    "news sentiment features",
    "on-chain flow features",
]

# Requirement 7.8: removed replay module forbidden
_REQ_7_8_FORBIDDEN: list[str] = [
    "go-live-replay",
    "live_aligned_replay",
    "LiveAlignedReplay",
    "GoLiveAlignedReplay",
]

# Requirement 7.9: WS/REST/auto_resume forbidden
_REQ_7_9_FORBIDDEN: list[str] = [
    "WS 重连触发",
    "REST 对账触发",
    "auto_resume",
    "auto-resume",
]

# Combined deduplicated forbidden words list (preserves insertion order)
FORBIDDEN_WORDS: list[str] = list(dict.fromkeys(
    _REQ_1_4_FORBIDDEN
    + _REQ_1_5_FORBIDDEN
    + _REQ_5_4_FORBIDDEN
    + _REQ_7_2_FORBIDDEN
    + _REQ_7_3_FORBIDDEN
    + _REQ_7_5_FORBIDDEN
    + _REQ_7_6_FORBIDDEN
    + _REQ_7_8_FORBIDDEN
    + _REQ_7_9_FORBIDDEN
))


class ForbiddenWordsScanner:
    """Scans files for forbidden literal strings.

    Uses `rg -F -i -n` (ripgrep fixed-string, case-insensitive, line numbers)
    when available; falls back to pure-Python case-insensitive matching.
    """

    def __init__(
        self,
        forbidden_words: Optional[list[str]] = None,
    ) -> None:
        self._forbidden_words = forbidden_words or FORBIDDEN_WORDS
        self._rg_path: Optional[str] = shutil.which("rg")

    def scan(self, file_paths: list[str]) -> list[ScanHit]:
        """Scan the given files for forbidden words.

        Args:
            file_paths: List of absolute or relative file paths to scan.

        Returns:
            List of ScanHit instances. Empty list means no violations (pass).
            Non-empty means at least one forbidden word was found (violation).
        """
        if not file_paths:
            return []

        # Filter to files that actually exist
        existing_paths = [p for p in file_paths if _file_exists(p)]
        if not existing_paths:
            return []

        if self._rg_path is not None:
            return self._scan_with_ripgrep(existing_paths)
        return self._scan_pure_python(existing_paths)

    def _scan_with_ripgrep(self, file_paths: list[str]) -> list[ScanHit]:
        """Use ripgrep for scanning (preferred, faster)."""
        hits: list[ScanHit] = []

        for word in self._forbidden_words:
            try:
                result = subprocess.run(
                    [
                        self._rg_path,  # type: ignore[arg-type]
                        "-F",           # fixed-string (no regex)
                        "-i",           # case-insensitive
                        "-n",           # line numbers
                        "--no-heading",
                        "--color=never",
                        "--",
                        word,
                    ]
                    + file_paths,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
            except (subprocess.TimeoutExpired, OSError):
                # Fallback to pure Python for this word on error
                hits.extend(self._scan_word_pure_python(file_paths, word))
                continue

            if result.returncode == 0 and result.stdout.strip():
                hits.extend(self._parse_rg_output(result.stdout, word))

        return hits

    def _parse_rg_output(self, output: str, matched_word: str) -> list[ScanHit]:
        """Parse ripgrep output lines in format: file_path:line_number:content."""
        hits: list[ScanHit] = []
        for line in output.strip().splitlines():
            # Format: path:lineno:content
            parts = line.split(":", 2)
            if len(parts) >= 3:
                file_path = parts[0]
                try:
                    line_number = int(parts[1])
                except ValueError:
                    continue
                line_content = parts[2].rstrip("\n\r")
                hits.append(ScanHit(
                    file_path=file_path,
                    line_number=line_number,
                    matched_word=matched_word,
                    line_content=line_content,
                ))
        return hits

    def _scan_pure_python(self, file_paths: list[str]) -> list[ScanHit]:
        """Pure Python fallback when ripgrep is not available."""
        hits: list[ScanHit] = []
        for word in self._forbidden_words:
            hits.extend(self._scan_word_pure_python(file_paths, word))
        return hits

    def _scan_word_pure_python(
        self, file_paths: list[str], word: str
    ) -> list[ScanHit]:
        """Scan for a single word across files using pure Python."""
        hits: list[ScanHit] = []
        word_lower = word.lower()

        for file_path in file_paths:
            try:
                with open(file_path, "r", encoding="utf-8", errors="replace") as f:
                    for line_number, line in enumerate(f, start=1):
                        if word_lower in line.lower():
                            hits.append(ScanHit(
                                file_path=file_path,
                                line_number=line_number,
                                matched_word=word,
                                line_content=line.rstrip("\n\r"),
                            ))
            except OSError:
                continue

        return hits


def _file_exists(path: str) -> bool:
    """Check if a file exists (avoids importing os.path at module level)."""
    import os
    return os.path.isfile(path)
