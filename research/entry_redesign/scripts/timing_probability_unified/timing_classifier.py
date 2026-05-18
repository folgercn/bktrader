"""3-Regime Timing Classification — DT3/DT4 训练与 LOOCV 选择"""

from __future__ import annotations

import sys
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.tree import DecisionTreeClassifier

# Ensure pre_breakout_timing is importable
_SCRIPTS_DIR = Path(__file__).resolve().parents[1]
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from pre_breakout_timing.delay_simulator import DelayResult  # noqa: E402
from pre_breakout_timing.timing_classifier import extract_rules_text  # noqa: E402


@dataclass
class TimingClassifierResult:
    """Timing 分类器训练结果"""

    selected_depth: int  # 选定的 max_depth (3 or 4)
    dt3_loocv_calendar_sum: float  # DT3 LOOCV calendar_sum
    dt4_loocv_calendar_sum: float  # DT4 LOOCV calendar_sum
    test_calendar_sum: float  # test set calendar_sum
    regime_distribution: dict[str, int]  # {skip: N, fast: N, slow: N}
    rules_text: str  # 决策规则文本
    classifier: object  # 训练好的 sklearn 分类器
    train_predictions: np.ndarray  # train set 预测
    test_predictions: np.ndarray  # test set 预测


# ---------------------------------------------------------------------------
# 3-Regime Label Generation
# ---------------------------------------------------------------------------


def select_best_depth(dt3_loocv_cs: float, dt4_loocv_cs: float) -> int:
    """Select best depth based on LOOCV calendar_sum. Prefer DT3 if equal.

    Parameters
    ----------
    dt3_loocv_cs : float
        LOOCV calendar_sum score for DT3 (max_depth=3).
    dt4_loocv_cs : float
        LOOCV calendar_sum score for DT4 (max_depth=4).

    Returns
    -------
    int
        Selected depth: 3 or 4.
    """
    if dt4_loocv_cs > dt3_loocv_cs:
        return 4
    return 3  # DT3 wins or tie (prefer simpler model)


def _loocv_calendar_sum_3regime(
    classifier_factory,
    features: pd.DataFrame,
    labels: pd.Series,
    delay_results: list[list[DelayResult]],
    events: pd.DataFrame,
) -> float:
    """LOOCV calendar_sum for 3-regime classifier (skip/fast/slow).

    Each iteration leaves one event out, trains on the rest, predicts the
    left-out event's regime, and uses get_selected_delay_pnl() to determine
    the PnL for that prediction. Finally computes silo-based calendar_sum.

    Parameters
    ----------
    classifier_factory : callable
        A callable that returns a fresh (unfitted) classifier instance.
    features : pd.DataFrame
        Feature matrix for all training events.
    labels : pd.Series
        3-regime labels ("skip", "fast", "slow") for all training events.
    delay_results : list[list[DelayResult]]
        Delay results for all training events.
    events : pd.DataFrame
        Events DataFrame (must contain 'symbol' and 'touch_time').

    Returns
    -------
    float
        LOOCV calendar_sum (percentage).
    """
    n = len(features)
    pnls = np.zeros(n, dtype=np.float64)

    for i in range(n):
        # Train on all except event i
        train_mask = np.ones(n, dtype=bool)
        train_mask[i] = False

        X_train = features.iloc[train_mask]
        y_train = labels.iloc[train_mask]
        X_test = features.iloc[[i]]

        clf = classifier_factory()
        clf.fit(X_train, y_train)

        # Predict regime for left-out event
        predicted_label = clf.predict(X_test)[0]

        # Get PnL for this prediction
        _, pnl = get_selected_delay_pnl(predicted_label, delay_results[i])
        pnls[i] = pnl

    # Compute silo-based calendar_sum
    return _silo_calendar_sum(pnls, events)


def train_and_select(
    train_features: pd.DataFrame,
    train_labels: pd.Series,
    delay_results_train: list[list[DelayResult]],
    test_features: pd.DataFrame,
    test_labels: pd.Series,
    delay_results_test: list[list[DelayResult]],
    train_events: pd.DataFrame | None = None,
    test_events: pd.DataFrame | None = None,
) -> TimingClassifierResult:
    """训练 DT3 和 DT4，通过 LOOCV calendar_sum 选择最优。

    复用 pre_breakout_timing/timing_classifier.py 的 extract_rules_text()。
    使用内部 _loocv_calendar_sum_3regime() 计算 LOOCV calendar_sum
    （适配 3-regime skip/fast/slow 标签）。

    Parameters
    ----------
    train_features : pd.DataFrame
        Training feature matrix.
    train_labels : pd.Series
        Training 3-regime labels ("skip", "fast", "slow").
    delay_results_train : list[list[DelayResult]]
        Delay results for training events.
    test_features : pd.DataFrame
        Test feature matrix.
    test_labels : pd.Series
        Test 3-regime labels (for reference, not used in training).
    delay_results_test : list[list[DelayResult]]
        Delay results for test events.
    train_events : pd.DataFrame | None
        Training events DataFrame (must contain 'symbol' and 'touch_time').
        If None, a minimal DataFrame is constructed from delay_results.
    test_events : pd.DataFrame | None
        Test events DataFrame (must contain 'symbol' and 'touch_time').
        If None, a minimal DataFrame is constructed from delay_results.

    Returns
    -------
    TimingClassifierResult
        Contains selected depth, LOOCV scores, test calendar_sum,
        regime distribution, rules text, classifier, and predictions.
    """
    # Build minimal events DataFrames if not provided
    if train_events is None:
        train_events = _build_minimal_events(train_features, delay_results_train)
    if test_events is None:
        test_events = _build_minimal_events(test_features, delay_results_test)

    # --- Train DT3 and DT4 ---
    dt3_factory = lambda: DecisionTreeClassifier(max_depth=3, random_state=42)  # noqa: E731
    dt4_factory = lambda: DecisionTreeClassifier(max_depth=4, random_state=42)  # noqa: E731

    # --- Compute LOOCV calendar_sum for each ---
    dt3_loocv_cs = _loocv_calendar_sum_3regime(
        dt3_factory, train_features, train_labels, delay_results_train, train_events
    )
    dt4_loocv_cs = _loocv_calendar_sum_3regime(
        dt4_factory, train_features, train_labels, delay_results_train, train_events
    )

    # --- Select best depth ---
    selected_depth = select_best_depth(dt3_loocv_cs, dt4_loocv_cs)

    # --- Train final classifier on full training set ---
    if selected_depth == 3:
        clf = DecisionTreeClassifier(max_depth=3, random_state=42)
    else:
        clf = DecisionTreeClassifier(max_depth=4, random_state=42)
    clf.fit(train_features, train_labels)

    # --- Get predictions ---
    train_predictions = clf.predict(train_features)
    test_predictions = clf.predict(test_features)

    # --- Compute test calendar_sum ---
    test_calendar_sum = evaluate_timing_predictions(
        test_predictions, delay_results_test, test_events
    )

    # --- Regime distribution (on test set) ---
    regime_distribution: dict[str, int] = {}
    for label in ["skip", "fast", "slow"]:
        regime_distribution[label] = int(np.sum(test_predictions == label))

    # --- Extract rules text ---
    feature_names = list(train_features.columns)
    rules_text = extract_rules_text(clf, feature_names)

    return TimingClassifierResult(
        selected_depth=selected_depth,
        dt3_loocv_calendar_sum=dt3_loocv_cs,
        dt4_loocv_calendar_sum=dt4_loocv_cs,
        test_calendar_sum=test_calendar_sum,
        regime_distribution=regime_distribution,
        rules_text=rules_text,
        classifier=clf,
        train_predictions=train_predictions,
        test_predictions=test_predictions,
    )


def _build_minimal_events(
    features: pd.DataFrame,
    delay_results: list[list[DelayResult]],
) -> pd.DataFrame:
    """Build a minimal events DataFrame from delay_results for calendar_sum.

    Extracts event_id (to infer symbol) and entry_time (for month silo).
    Falls back to a generic DataFrame if delay_results lack needed info.
    """
    n = len(features)
    symbols = []
    touch_times = []

    for i in range(n):
        event_delays = delay_results[i]
        # Infer symbol from event_id
        event_id = event_delays[0].event_id if event_delays else f"unknown_{i}"
        eid_upper = event_id.upper()
        if "BTC" in eid_upper:
            symbol = "BTCUSDT"
        elif "ETH" in eid_upper:
            symbol = "ETHUSDT"
        else:
            symbol = "UNKNOWN"
        symbols.append(symbol)

        # Use entry_time from first traded delay, or a placeholder
        entry_time = None
        for dr in event_delays:
            if dr.entry_time is not None:
                entry_time = dr.entry_time
                break
        if entry_time is None:
            # Fallback: use a generic timestamp
            entry_time = pd.Timestamp("2025-01-01", tz="UTC") + pd.Timedelta(hours=i)
        touch_times.append(entry_time)

    return pd.DataFrame({"symbol": symbols, "touch_time": touch_times})


def generate_3regime_label_from_pnls(
    d0_pnl: float,
    d5_pnl: float,
    d10_pnl: float,
    d15_pnl: float,
    pullback_pnl: float,
    tolerance_bps: float = 5.0,
) -> str:
    """Generate a single 3-regime label from raw PnL values.

    规则：
    - fast_pnl = max(d0_pnl, d5_pnl)
    - slow_pnl = max(d10_pnl, d15_pnl, pullback_pnl)
    - IF fast_pnl < 0 AND slow_pnl < 0 → "skip"
    - ELSE IF fast_pnl >= slow_pnl → "fast"
    - ELSE IF (slow_pnl - fast_pnl) < tolerance → "fast" (容差优先 fast)
    - ELSE → "slow"

    Parameters
    ----------
    d0_pnl : float
        D0 delay PnL percentage.
    d5_pnl : float
        D5 delay PnL percentage.
    d10_pnl : float
        D10 delay PnL percentage.
    d15_pnl : float
        D15 delay PnL percentage.
    pullback_pnl : float
        Pullback delay PnL percentage.
    tolerance_bps : float
        Tolerance in basis points. Default 5.0 (= 0.0005 in pnl_pct units).

    Returns
    -------
    str
        One of "skip", "fast", "slow".
    """
    tolerance = tolerance_bps / 10000.0  # 5 bps = 0.0005

    fast_pnl = max(d0_pnl, d5_pnl)
    slow_pnl = max(d10_pnl, d15_pnl, pullback_pnl)

    if fast_pnl < 0 and slow_pnl < 0:
        return "skip"
    elif fast_pnl >= slow_pnl:
        return "fast"
    elif (slow_pnl - fast_pnl) < tolerance:
        # 差距 < 5bps 时优先 fast
        return "fast"
    else:
        return "slow"


def generate_3regime_labels(
    delay_results: list[list[DelayResult]],
    tolerance_bps: float = 5.0,
) -> pd.Series:
    """基于 delay_results 生成 3-regime 标签 {skip, fast, slow}。

    规则：
    - fast_pnl = max(D0_pnl, D5_pnl)
    - slow_pnl = max(D10_pnl, D15_pnl, pullback_pnl)
    - IF fast_pnl < 0 AND slow_pnl < 0 → skip
    - ELSE IF fast_pnl > slow_pnl (差距 < 5bps 时优先 fast) → fast
    - ELSE → slow

    For untradeable delays (traded=False or pnl_pct is None), the PnL is
    treated as a large negative value (-inf) so that delay is effectively
    excluded from the max computation. If ALL 5 delays are untradeable,
    the label is "skip".

    Parameters
    ----------
    delay_results : list[list[DelayResult]]
        Outer list: one entry per event.
        Inner list: 5 DelayResult objects (D0, D5, D10, D15, pullback).
    tolerance_bps : float
        Tolerance in basis points for the "prefer fast" rule. Default 5.0.

    Returns
    -------
    pd.Series
        Series of labels with values in {"skip", "fast", "slow"}.
    """
    labels: list[str] = []

    for event_delays in delay_results:
        # Build a mapping from delay_label to pnl_pct
        pnl_map: dict[str, float] = {}
        for dr in event_delays:
            if dr.traded and dr.pnl_pct is not None:
                pnl_map[dr.delay_label] = dr.pnl_pct
            else:
                # Untradeable delay → treat as -inf (will lose to any real pnl)
                pnl_map[dr.delay_label] = float("-inf")

        # Extract PnLs for fast and slow groups
        d0_pnl = pnl_map.get("D0", float("-inf"))
        d5_pnl = pnl_map.get("D5", float("-inf"))
        d10_pnl = pnl_map.get("D10", float("-inf"))
        d15_pnl = pnl_map.get("D15", float("-inf"))
        pullback_pnl = pnl_map.get("pullback", float("-inf"))

        # If all delays are -inf (all untradeable), label as skip
        all_pnls = [d0_pnl, d5_pnl, d10_pnl, d15_pnl, pullback_pnl]
        if all(p == float("-inf") for p in all_pnls):
            labels.append("skip")
            continue

        # Replace -inf with a large negative for the label logic
        # (so that max() works correctly when at least one is finite)
        def _safe(v: float) -> float:
            return v if v != float("-inf") else -1e10

        label = generate_3regime_label_from_pnls(
            d0_pnl=_safe(d0_pnl),
            d5_pnl=_safe(d5_pnl),
            d10_pnl=_safe(d10_pnl),
            d15_pnl=_safe(d15_pnl),
            pullback_pnl=_safe(pullback_pnl),
            tolerance_bps=tolerance_bps,
        )
        labels.append(label)

    return pd.Series(labels, dtype="object")


# ---------------------------------------------------------------------------
# Prediction Evaluation Helpers
# ---------------------------------------------------------------------------

# Delay groups for fast and slow predictions
_FAST_DELAYS = {"D0", "D5"}
_SLOW_DELAYS = {"D10", "D15", "pullback"}


def get_selected_delay_pnl(
    prediction: str,
    event_delays: list[DelayResult],
) -> tuple[str, float]:
    """Get the selected delay label and its PnL for a timing prediction.

    For a "fast" prediction, selects the delay with the highest PnL among
    D0 and D5. For "slow", selects among D10, D15, and pullback.
    Untradeable delays (traded=False) are treated as pnl=0.

    Parameters
    ----------
    prediction : str
        One of "skip", "fast", "slow".
    event_delays : list[DelayResult]
        The 5 DelayResult objects for this event.

    Returns
    -------
    tuple[str, float]
        (delay_label, pnl_pct). For "skip", returns ("none", 0.0).
    """
    if prediction == "skip":
        return ("none", 0.0)

    # Determine which delay group to consider
    if prediction == "fast":
        target_delays = _FAST_DELAYS
    else:  # "slow"
        target_delays = _SLOW_DELAYS

    best_label = "none"
    best_pnl = 0.0
    found_any = False

    for dr in event_delays:
        if dr.delay_label not in target_delays:
            continue
        # Untradeable delays → treat pnl as 0
        if not dr.traded or dr.pnl_pct is None:
            pnl = 0.0
        else:
            pnl = dr.pnl_pct

        if not found_any or pnl > best_pnl:
            best_pnl = pnl
            best_label = dr.delay_label
            found_any = True

    return (best_label, best_pnl)


def evaluate_timing_predictions(
    predictions: np.ndarray,
    delay_results: list[list[DelayResult]],
    events: pd.DataFrame,
) -> float:
    """评估 timing 预测的 calendar_sum。

    - fast → 使用 max(D0_pnl, D5_pnl) 对应的 delay
    - slow → 使用 max(D10_pnl, D15_pnl, pullback_pnl) 对应的 delay
    - skip → pnl = 0

    Untradeable delays (traded=False) are treated as pnl=0 for that delay.

    Parameters
    ----------
    predictions : np.ndarray
        Array of prediction strings ("skip", "fast", "slow"), one per event.
    delay_results : list[list[DelayResult]]
        Outer list: one entry per event.
        Inner list: 5 DelayResult objects (D0, D5, D10, D15, pullback).
    events : pd.DataFrame
        Events DataFrame. Used for calendar_sum computation if it contains
        'symbol' and 'touch_time' columns (silo-based). Otherwise uses
        simple sum.

    Returns
    -------
    float
        The calendar_sum (percentage) of the timing predictions.
    """
    n = len(predictions)
    pnls = np.zeros(n, dtype=np.float64)

    for i in range(n):
        _, pnl = get_selected_delay_pnl(predictions[i], delay_results[i])
        pnls[i] = pnl

    # If events has symbol and touch_time, use silo-based calendar_sum
    if "symbol" in events.columns and "touch_time" in events.columns:
        return _silo_calendar_sum(pnls, events)
    else:
        # Simple sum fallback
        return float(np.sum(pnls))


def _silo_calendar_sum(pnls: np.ndarray, events: pd.DataFrame) -> float:
    """Compute silo-based calendar_sum.

    Each (symbol, month) silo starts from a notional 100k base.
    The calendar_sum is the sum of all silo returns.

    Parameters
    ----------
    pnls : np.ndarray
        PnL percentages for each event.
    events : pd.DataFrame
        Must contain 'symbol' and 'touch_time' columns.

    Returns
    -------
    float
        Sum of all silo returns (percentage).
    """
    # Create a working DataFrame
    df = pd.DataFrame(
        {
            "symbol": events["symbol"].values,
            "touch_time": pd.to_datetime(events["touch_time"]),
            "pnl_pct": pnls,
        }
    )
    df["month"] = df["touch_time"].dt.to_period("M")

    # Group by (symbol, month) and sum pnl_pct within each silo
    silo_sums = df.groupby(["symbol", "month"])["pnl_pct"].sum()

    return float(silo_sums.sum())
