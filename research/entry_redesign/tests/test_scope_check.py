"""Unit tests for entry_redesign scope_check modules.

Tests:
  - ForbiddenWordsScanner: clean pass (compliant file), forbidden word hit (rejection)
  - OutOfScopePathsScanner: clean diff (no forbidden paths), forbidden path hit
  - ApprovalLiteralChecker: with/without approval literal

Uses tempfile for test fixtures.

Requirements: 7.11
"""

from __future__ import annotations

import tempfile
import pathlib

import pytest

from research.entry_redesign.scope_check.forbidden_words import (
    ForbiddenWordsScanner,
    FORBIDDEN_WORDS,
)
from research.entry_redesign.scope_check.out_of_scope_paths import (
    OutOfScopePathsScanner,
)
from research.entry_redesign.scope_check.approval_literal import (
    ApprovalLiteralChecker,
)


# ===========================================================================
# ForbiddenWordsScanner tests
# ===========================================================================


class TestForbiddenWordsScannerCleanPass:
    """Positive fixture: compliant file → scope_check passes (no hits)."""

    def test_clean_file_returns_empty_hits(self, tmp_path: pathlib.Path) -> None:
        """A file with no forbidden words should produce zero hits."""
        content = (
            "# Entry Redesign Requirements\n"
            "\n"
            "This spec modifies the entry layer only.\n"
            "Research baseline: dir2_zero_initial=true.\n"
            "Intrabar breakout semantic preserved.\n"
            "No live changes, no gate modifications.\n"
        )
        f = tmp_path / "requirements.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert hits == []

    def test_empty_file_returns_empty_hits(self, tmp_path: pathlib.Path) -> None:
        """An empty file should produce zero hits."""
        f = tmp_path / "empty.md"
        f.write_text("", encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert hits == []

    def test_nonexistent_file_returns_empty_hits(self) -> None:
        """Non-existent file paths should be silently skipped."""
        scanner = ForbiddenWordsScanner()
        hits = scanner.scan(["/nonexistent/path/file.md"])

        assert hits == []

    def test_empty_file_list_returns_empty_hits(self) -> None:
        """Empty file list should produce zero hits."""
        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([])

        assert hits == []


class TestForbiddenWordsScannerNegative:
    """Negative fixture: each forbidden word set has at least one hit → rejection."""

    def test_req_1_4_breakout_semantics_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 1.4 forbidden: '闭合 signal bar 收盘确认' triggers rejection."""
        content = "entry 使用闭合 signal bar 收盘确认来判断突破\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "闭合 signal bar 收盘确认" for h in hits)

    def test_req_1_4_close_confirm_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 1.4 forbidden: '1h close confirm' triggers rejection."""
        content = "We use 1h close confirm for breakout validation.\n"
        f = tmp_path / "tasks.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "1h close confirm" for h in hits)

    def test_req_1_5_replay_module_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 1.5 forbidden: 'LiveAlignedReplay' triggers rejection."""
        content = "Referencing LiveAlignedReplay module for comparison.\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "LiveAlignedReplay" for h in hits)

    def test_req_5_4_auto_dispatch_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 5.4 forbidden: 'auto-dispatch' triggers rejection."""
        content = "The system uses auto-dispatch for order routing.\n"
        f = tmp_path / "requirements.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "auto-dispatch" for h in hits)

    def test_req_5_4_control_reset_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 5.4 forbidden: 'control-reset' triggers rejection."""
        content = "Run control-reset to fix stuck sessions.\n"
        f = tmp_path / "tasks.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "control-reset" for h in hits)

    def test_req_7_2_gate_modification_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 7.2 forbidden: 'candidate_002' triggers rejection."""
        content = "Introducing candidate_002 as a new gate variant.\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "candidate_002" for h in hits)

    def test_req_7_3_ml_model_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 7.3 forbidden: 'GRU model training' triggers rejection."""
        content = "We plan GRU model training for entry filtering.\n"
        f = tmp_path / "requirements.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "GRU model training" for h in hits)

    def test_req_7_5_event_source_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 7.5 forbidden: 'prev_high_8 entry' triggers rejection."""
        content = "Switch to prev_high_8 entry for wider breakout.\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "prev_high_8 entry" for h in hits)

    def test_req_7_6_feature_engineering_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 7.6 forbidden: 'tick-level order flow features' triggers rejection."""
        content = "Adding tick-level order flow features to the model.\n"
        f = tmp_path / "tasks.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "tick-level order flow features" for h in hits)

    def test_req_7_9_ws_rest_hit(self, tmp_path: pathlib.Path) -> None:
        """Requirement 7.9 forbidden: 'auto_resume' triggers rejection."""
        content = "Enable auto_resume after WS reconnect.\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "auto_resume" for h in hits)

    def test_case_insensitive_matching(self, tmp_path: pathlib.Path) -> None:
        """Forbidden words should match case-insensitively."""
        content = "Using DISPATCHMODE for configuration.\n"
        f = tmp_path / "design.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 1
        assert any(h.matched_word == "dispatchMode" for h in hits)

    def test_multiple_hits_in_single_file(self, tmp_path: pathlib.Path) -> None:
        """Multiple forbidden words in one file should all be reported."""
        content = (
            "Line 1: auto-dispatch enabled\n"
            "Line 2: control-reset triggered\n"
            "Line 3: session.config updated\n"
        )
        f = tmp_path / "tasks.md"
        f.write_text(content, encoding="utf-8")

        scanner = ForbiddenWordsScanner()
        hits = scanner.scan([str(f)])

        assert len(hits) >= 3
        matched_words = {h.matched_word for h in hits}
        assert "auto-dispatch" in matched_words
        assert "control-reset" in matched_words
        assert "session.config" in matched_words


# ===========================================================================
# OutOfScopePathsScanner tests
# ===========================================================================


class TestOutOfScopePathsScannerCleanDiff:
    """Positive fixture: clean diff (only research/ paths) → no violations."""

    def test_clean_diff_returns_empty_violations(self) -> None:
        """A diff touching only research/ files should produce zero violations."""
        diff_text = (
            "diff --git a/research/entry_redesign/spec/entry_candidate_spec.py "
            "b/research/entry_redesign/spec/entry_candidate_spec.py\n"
            "--- a/research/entry_redesign/spec/entry_candidate_spec.py\n"
            "+++ b/research/entry_redesign/spec/entry_candidate_spec.py\n"
            "@@ -1,3 +1,4 @@\n"
            " # Entry candidate spec\n"
            "+# Added new validation\n"
            " from dataclasses import dataclass\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert violations == []

    def test_empty_diff_returns_empty_violations(self) -> None:
        """An empty diff should produce zero violations."""
        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff("")

        assert violations == []

    def test_diff_with_kiro_spec_paths_passes(self) -> None:
        """A diff touching .kiro/specs/ should produce zero violations."""
        diff_text = (
            "diff --git a/.kiro/specs/original-t2-entry-logic-redesign/design.md "
            "b/.kiro/specs/original-t2-entry-logic-redesign/design.md\n"
            "--- a/.kiro/specs/original-t2-entry-logic-redesign/design.md\n"
            "+++ b/.kiro/specs/original-t2-entry-logic-redesign/design.md\n"
            "@@ -1,2 +1,3 @@\n"
            " # Design\n"
            "+## New section\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert violations == []

    def test_backtick_exempt_literal_passes(self) -> None:
        """Forbidden path inside exempt backtick literal should not trigger violation."""
        # Backtick literal <= 120 chars, no code snippet, no line number → exempt
        diff_text = (
            "+This spec does not touch `internal/service/live_session.go` (AGENTS §3).\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert violations == []


class TestOutOfScopePathsScannerNegative:
    """Negative fixture: forbidden path hit → violation reported."""

    def test_live_go_file_in_diff_header(self) -> None:
        """Diff header touching internal/service/live_session.go → violation."""
        diff_text = (
            "diff --git a/internal/service/live_session.go "
            "b/internal/service/live_session.go\n"
            "--- a/internal/service/live_session.go\n"
            "+++ b/internal/service/live_session.go\n"
            "@@ -10,3 +10,4 @@\n"
            " func startSession() {\n"
            "+    // new logic\n"
            " }\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(v.pattern_matched == "internal/service/live*.go" for v in violations)

    def test_execution_strategy_in_diff_header(self) -> None:
        """Diff header touching internal/service/execution_strategy.go → violation."""
        diff_text = (
            "--- a/internal/service/execution_strategy.go\n"
            "+++ b/internal/service/execution_strategy.go\n"
            "@@ -1,2 +1,3 @@\n"
            " package service\n"
            "+// modified\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(
            v.pattern_matched == "internal/service/execution_strategy.go"
            for v in violations
        )

    def test_deployments_path_in_diff(self) -> None:
        """Diff touching deployments/ → violation."""
        diff_text = (
            "--- a/deployments/docker-compose.yml\n"
            "+++ b/deployments/docker-compose.yml\n"
            "@@ -1,2 +1,3 @@\n"
            " version: '3'\n"
            "+  # new service\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(v.pattern_matched == "deployments/" for v in violations)

    def test_github_workflows_in_diff(self) -> None:
        """Diff touching .github/workflows/ → violation."""
        diff_text = (
            "--- a/.github/workflows/ci.yml\n"
            "+++ b/.github/workflows/ci.yml\n"
            "@@ -5,3 +5,4 @@\n"
            " jobs:\n"
            "+  new-job:\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(v.pattern_matched == ".github/workflows/" for v in violations)

    def test_cmd_path_in_diff(self) -> None:
        """Diff touching cmd/ → violation."""
        diff_text = (
            "--- a/cmd/platform-api/main.go\n"
            "+++ b/cmd/platform-api/main.go\n"
            "@@ -1,2 +1,3 @@\n"
            " package main\n"
            "+// updated\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(v.pattern_matched == "cmd/" for v in violations)

    def test_web_path_in_diff(self) -> None:
        """Diff touching web/ → violation."""
        diff_text = (
            "--- a/web/console/src/App.tsx\n"
            "+++ b/web/console/src/App.tsx\n"
            "@@ -1,2 +1,3 @@\n"
            " import React from 'react';\n"
            "+// new component\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1
        assert any(v.pattern_matched == "web/" for v in violations)

    def test_non_exempt_backtick_with_code_snippet(self) -> None:
        """Backtick literal containing code snippet keywords is NOT exempt."""
        # Contains 'func ' which is a code snippet keyword → not exempt
        diff_text = (
            "+Modified `func startLiveSession() in internal/service/live_runner.go` here.\n"
        )

        scanner = OutOfScopePathsScanner()
        violations = scanner.scan_diff(diff_text)

        assert len(violations) >= 1


# ===========================================================================
# ApprovalLiteralChecker tests
# ===========================================================================


class TestApprovalLiteralCheckerWithApproval:
    """Positive fixture: PR description contains valid approval → passes."""

    def test_approval_present_with_scope_violations(self) -> None:
        """Approval present + scope violations → approved=True."""
        pr_description = (
            "## Changes\n"
            "Updated live session logic.\n"
            "\n"
            "AGENTS §3 high-risk approval: wuyaocheng 2025-06-15\n"
        )

        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=True)

        assert result.approved is True
        assert result.missing_approval is False
        assert result.approval_text is not None
        assert "wuyaocheng" in result.approval_text
        assert "2025-06-15" in result.approval_text

    def test_no_scope_violations_no_approval_needed(self) -> None:
        """No scope violations → approved=True regardless of approval presence."""
        pr_description = "Simple research-only change.\n"

        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=False)

        assert result.approved is True

    def test_no_scope_violations_with_approval_still_passes(self) -> None:
        """No scope violations + approval present → approved=True."""
        pr_description = (
            "Research change.\n"
            "AGENTS §3 high-risk approval: folgercn 2025-07-01\n"
        )

        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=False)

        assert result.approved is True
        assert result.approval_text is not None


class TestApprovalLiteralCheckerWithoutApproval:
    """Negative fixture: PR description missing approval + scope violations → rejected."""

    def test_missing_approval_with_scope_violations(self) -> None:
        """Scope violations + no approval → approved=False (rejected)."""
        pr_description = (
            "## Changes\n"
            "Updated live session logic.\n"
            "No approval provided.\n"
        )

        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=True)

        assert result.approved is False
        assert result.missing_approval is True
        assert result.approval_text is None

    def test_malformed_approval_date_rejected(self) -> None:
        """Approval with malformed date (not YYYY-MM-DD) → treated as missing."""
        pr_description = (
            "AGENTS §3 high-risk approval: wuyaocheng 2025/06/15\n"
        )

        checker = ApprovalLiteralChecker()
        result = checker.check(pr_description, has_scope_violations=True)

        assert result.approved is False
        assert result.missing_approval is True

    def test_empty_pr_description_rejected(self) -> None:
        """Empty PR description + scope violations → rejected."""
        checker = ApprovalLiteralChecker()
        result = checker.check("", has_scope_violations=True)

        assert result.approved is False
        assert result.missing_approval is True

    def test_has_high_risk_path_hits_detects_live_go(self) -> None:
        """has_high_risk_path_hits() detects internal/service/live*.go in diff."""
        pr_diff = (
            "--- a/internal/service/live_session.go\n"
            "+++ b/internal/service/live_session.go\n"
        )

        assert ApprovalLiteralChecker.has_high_risk_path_hits(pr_diff) is True

    def test_has_high_risk_path_hits_clean_diff(self) -> None:
        """has_high_risk_path_hits() returns False for research-only diff."""
        pr_diff = (
            "--- a/research/entry_redesign/spec/entry_candidate_spec.py\n"
            "+++ b/research/entry_redesign/spec/entry_candidate_spec.py\n"
        )

        assert ApprovalLiteralChecker.has_high_risk_path_hits(pr_diff) is False
