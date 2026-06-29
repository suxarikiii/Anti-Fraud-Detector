"""
Scenario validation / model evaluation for the rule-based refund scoring.

What it does:
  1. Re-implements the EXACT scoring logic of the Kotlin scoring-service
     (backend/scoring-service, ScoringService.kt) in Python, so validation can
     run locally and in CI without booting the backend, and without secrets or
     external services.
  2. Scores every refund approval in data/clean_refund_dataset.csv.
  3. Baseline check: compares a fixed set of demo return IDs against
     data/expected_scores.csv (the same demo IDs the scoring-service tests are
     pinned to). Any change to their risk level/reasons fails the run.
  4. Model metrics: over the full labelled dataset, reports total cases,
     matched expected levels, false positives among normal cases, and missed
     suspicious cases (plus precision/recall at the flag threshold).

Usage:
    # validate: score, check baseline, print metrics, exit non-zero on regression
    python scripts/evaluate_scenarios.py

    # (re)generate the expected baseline from the current scoring + dataset
    python scripts/evaluate_scenarios.py --write-expected

    python scripts/evaluate_scenarios.py --data-dir data
"""

from __future__ import annotations

import argparse
import csv
import sys
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Tuple


# --------------------------------------------------------------------------- #
# Scoring logic — faithful port of backend/scoring-service ScoringService.kt
#
# Each constant maps to one rule in calculateReasons(). The score is the sum of
# the matched rules' impacts, clamped to 0..100, then bucketed into a risk level.
# --------------------------------------------------------------------------- #

HIGH_VALUE_RATIO = 0.70          # refund/order ratio >=  -> HIGH_VALUE_REFUND (+20)
FULL_AMOUNT_RATIO = 0.95         # refund/order ratio >=  -> FULL_AMOUNT_REFUND (+15)
FAST_APPROVAL_MINUTES = 3        # decision time (min) <=  -> FAST_APPROVAL      (+15)
AGENT_MIN_DECISIONS = 5          # agent needs >= this many decisions for the rate rule to apply
AGENT_HIGH_RATE = 0.85           # agent approval rate >   -> AGENT_HIGH_APPROVAL_RATE (+30)
CUSTOMER_FREQUENT = 5            # customer return count >= -> CUSTOMER_FREQUENT_RETURNS (+20)
REPEATED_PAIR = 3                # same agent+customer count >= -> REPEATED_AGENT_CUSTOMER_PAIR (+25)
CLUSTER_SIZE = 5                 # max(customer returns, pair count) >= -> SUSPICIOUS_CLUSTER (+25)

# A record is considered "flagged / suspicious" at this score (RiskLevel >= MEDIUM).
SUSPICIOUS_THRESHOLD = 31


@dataclass
class Record:
    """One refund approval row from clean_refund_dataset.csv."""
    order_id: str
    customer_id: str
    return_id: str
    support_agent_id: str
    order_amount: float
    refund_amount: float
    product_category: str
    return_reason: str
    evidence_provided: bool
    decision: str
    manual_override: bool
    decision_time_minutes: int
    timestamp: str


def _to_bool(value: str) -> bool:
    """Match the scoring service's boolean parsing (true/yes/1 -> True)."""
    return str(value).strip().lower() in {"true", "yes", "1"}


def load_records(path: Path) -> List[Record]:
    """Read the clean dataset into typed Record objects."""
    records: List[Record] = []
    with path.open(newline="", encoding="utf-8") as fh:
        for row in csv.DictReader(fh):
            records.append(Record(
                order_id=row["order_id"].strip(),
                customer_id=row["customer_id"].strip(),
                return_id=row["return_id"].strip(),
                support_agent_id=row["support_agent_id"].strip(),
                order_amount=float(row["order_amount"]),
                refund_amount=float(row["refund_amount"]),
                product_category=row["product_category"].strip(),
                return_reason=row["return_reason"].strip(),
                evidence_provided=_to_bool(row["evidence_provided"]),
                decision=row["decision"].strip().upper(),
                manual_override=_to_bool(row["manual_override"]),
                decision_time_minutes=int(row["decision_time_minutes"]),
                timestamp=row["timestamp"].strip(),
            ))
    return records


class ScoringEngine:
    """Mirrors ScoringService.buildFeatures + calculateReasons + resolveRiskLevel.

    The relational rules (agent approval rate, customer frequency, repeated
    pair, cluster) depend on the whole dataset, so we pre-aggregate those counts
    once in __init__ and reuse them when scoring each record — same as the Kotlin
    service computing them over findAll().
    """

    def __init__(self, records: List[Record]) -> None:
        self.records = records

        # Dataset-wide signals, computed once.
        self._customer_returns: Counter = Counter()   # customer_id -> number of returns
        self._agent_total: Counter = Counter()        # agent_id -> number of decisions
        self._agent_approved: Counter = Counter()     # agent_id -> number of APPROVED decisions
        self._pair: Counter = Counter()               # (agent_id, customer_id) -> count
        for r in records:
            self._customer_returns[r.customer_id] += 1
            self._agent_total[r.support_agent_id] += 1
            if r.decision == "APPROVED":
                self._agent_approved[r.support_agent_id] += 1
            self._pair[(r.support_agent_id, r.customer_id)] += 1

    def reasons(self, r: Record) -> List[Tuple[str, int]]:
        """Return the (rule_type, score_impact) pairs that fire for this record."""
        # Per-record derived values.
        ratio = (r.refund_amount / r.order_amount) if r.order_amount != 0 else 0.0
        agent_decisions = self._agent_total[r.support_agent_id]
        agent_rate = (self._agent_approved[r.support_agent_id] / agent_decisions
                      if agent_decisions else 0.0)
        customer_returns = self._customer_returns[r.customer_id]
        pair_count = self._pair[(r.support_agent_id, r.customer_id)]
        cluster_size = max(customer_returns, pair_count)

        out: List[Tuple[str, int]] = []

        # Per-record rules.
        if r.decision == "APPROVED" and not r.evidence_provided:
            out.append(("NO_EVIDENCE", 25))
        if ratio >= HIGH_VALUE_RATIO:
            out.append(("HIGH_VALUE_REFUND", 20))
        if ratio >= FULL_AMOUNT_RATIO:
            out.append(("FULL_AMOUNT_REFUND", 15))
        if r.decision_time_minutes <= FAST_APPROVAL_MINUTES:
            # NOTE: mirrors the service — this fires regardless of decision.
            out.append(("FAST_APPROVAL", 15))
        if r.manual_override:
            out.append(("MANUAL_OVERRIDE", 20))

        # Relational rules (use the pre-aggregated dataset signals).
        if agent_decisions >= AGENT_MIN_DECISIONS and agent_rate > AGENT_HIGH_RATE:
            out.append(("AGENT_HIGH_APPROVAL_RATE", 30))
        if customer_returns >= CUSTOMER_FREQUENT:
            out.append(("CUSTOMER_FREQUENT_RETURNS", 20))
        if pair_count >= REPEATED_PAIR:
            out.append(("REPEATED_AGENT_CUSTOMER_PAIR", 25))
        if cluster_size >= CLUSTER_SIZE:
            out.append(("SUSPICIOUS_CLUSTER", 25))
        return out

    @staticmethod
    def score(reasons: List[Tuple[str, int]]) -> int:
        """Sum the impacts and clamp to the 0..100 range (as the service does)."""
        return max(0, min(100, sum(impact for _, impact in reasons)))

    @staticmethod
    def level(score: int) -> str:
        """Bucket a score into a risk level: LOW / MEDIUM / HIGH / CRITICAL."""
        if score >= 81:
            return "CRITICAL"
        if score >= 61:
            return "HIGH"
        if score >= 31:
            return "MEDIUM"
        return "LOW"

    def evaluate(self, r: Record) -> Tuple[int, str, List[str]]:
        """Score a record -> (score, risk_level, sorted-or-ordered reason types)."""
        reasons = self.reasons(r)
        score = self.score(reasons)
        return score, self.level(score), [t for t, _ in reasons]


# --------------------------------------------------------------------------- #
# Demo baseline (expected_scores.csv)
#
# The legacy demo return IDs (return_3001..return_3045) are the records the
# scoring-service tests are pinned to and the ones agreed with Ernest. They are
# 5 per scenario, in scenario order, so the scenario id is derivable from the id.
# Their expected risk level + reasons are frozen in data/expected_scores.csv and
# act as a regression guard.
# --------------------------------------------------------------------------- #

EXPECTED_COLUMNS = ["return_id", "scenario_id", "scenario", "expected_risk_level", "expected_reason_types"]

DEMO_RETURN_IDS = [f"return_{3000 + i}" for i in range(1, 46)]

SCENARIO_NAMES = {
    1: "normal",
    2: "high_value_no_evidence",
    3: "full_amount_expensive",
    4: "fast_approval",
    5: "manual_override_high_value",
    6: "frequent_customer",
    7: "high_approval_agent",
    8: "repeated_agent_customer_pair",
    9: "suspicious_cluster",
}


def demo_scenario_id(return_id: str) -> int:
    """return_3001..3005 -> 1, 3006..3010 -> 2, ... (5 demo cases per scenario)."""
    n = int(return_id.rsplit("_", 1)[1]) - 3001
    return n // 5 + 1


def build_expected(engine: ScoringEngine, by_return: Dict[str, Record]) -> List[Dict[str, str]]:
    """Score the demo cases and produce the expected-baseline rows."""
    rows: List[Dict[str, str]] = []
    for return_id in DEMO_RETURN_IDS:
        record = by_return.get(return_id)
        if record is None:
            continue
        _, level, reasons = engine.evaluate(record)
        sid = demo_scenario_id(return_id)
        rows.append({
            "return_id": return_id,
            "scenario_id": str(sid),
            "scenario": SCENARIO_NAMES[sid],
            "expected_risk_level": level,
            # Sorted so the baseline comparison is order-independent.
            "expected_reason_types": ";".join(sorted(reasons)),
        })
    return rows


def write_expected(path: Path, rows: List[Dict[str, str]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=EXPECTED_COLUMNS)
        writer.writeheader()
        writer.writerows(rows)


def read_expected(path: Path) -> Dict[str, Dict[str, str]]:
    with path.open(newline="", encoding="utf-8") as fh:
        return {row["return_id"]: row for row in csv.DictReader(fh)}


# --------------------------------------------------------------------------- #
# Labels (ground truth for the model metrics)
# --------------------------------------------------------------------------- #

def load_labels(path: Path) -> Dict[str, str]:
    """Return return_id -> label ('normal' | 'fraud'). Empty if the file is absent."""
    if not path.exists():
        return {}
    with path.open(newline="", encoding="utf-8") as fh:
        return {row["return_id"]: row["label"] for row in csv.DictReader(fh)}


# --------------------------------------------------------------------------- #
# Main
# --------------------------------------------------------------------------- #

def main() -> int:
    parser = argparse.ArgumentParser(description="Validate rule-based refund scoring against scenarios.")
    parser.add_argument("--data-dir", type=str, default="data")
    parser.add_argument("--write-expected", action="store_true",
                        help="Regenerate the expected baseline file instead of validating.")
    args = parser.parse_args()

    data_dir = Path(args.data_dir)
    clean_path = data_dir / "clean_refund_dataset.csv"
    labels_path = data_dir / "dataset_labels.csv"
    expected_path = data_dir / "expected_scores.csv"

    if not clean_path.exists():
        print(f"ERROR: {clean_path} not found. Generate the dataset first.")
        return 2

    records = load_records(clean_path)
    engine = ScoringEngine(records)
    by_return = {r.return_id: r for r in records}

    # --- mode: regenerate the baseline and exit -------------------------- #
    if args.write_expected:
        rows = build_expected(engine, by_return)
        write_expected(expected_path, rows)
        print(f"Wrote expected baseline for {len(rows)} demo cases -> {expected_path}")
        return 0

    # --- score the whole dataset once ------------------------------------ #
    scored = {r.return_id: engine.evaluate(r) for r in records}  # return_id -> (score, level, reasons)
    labels = load_labels(labels_path)

    print("=" * 64)
    print("SCENARIO VALIDATION - rule-based refund scoring")
    print("=" * 64)

    # --- [1] demo baseline regression check ------------------------------ #
    # Compare each demo case's current level + reasons to the frozen baseline.
    print("\n[1] Demo baseline check (data/expected_scores.csv)")
    failures: List[str] = []
    matched = 0
    expected: Dict[str, Dict[str, str]] = {}
    if not expected_path.exists():
        print("    expected_scores.csv missing - run with --write-expected first.")
        failures.append("expected_scores.csv missing")
    else:
        expected = read_expected(expected_path)
        for return_id, exp in expected.items():
            actual = scored.get(return_id)
            if actual is None:
                failures.append(f"{return_id}: missing from dataset")
                continue
            _, level, reasons = actual
            act_reasons = ";".join(sorted(reasons))
            if level == exp["expected_risk_level"] and act_reasons == exp["expected_reason_types"]:
                matched += 1
            else:
                failures.append(
                    f"{return_id} ({exp['scenario']}): "
                    f"level {level} vs {exp['expected_risk_level']}, "
                    f"reasons [{act_reasons}] vs [{exp['expected_reason_types']}]"
                )
        print(f"    matched {matched}/{len(expected)} demo cases")

    # --- [2] per-scenario behaviour on the demo set ---------------------- #
    # Sanity view: suspicious scenarios should trend HIGH/CRITICAL, normal LOW.
    print("\n[2] Per-scenario demo risk levels")
    scen_levels: Dict[int, Counter] = {}
    for return_id in DEMO_RETURN_IDS:
        if return_id in scored:
            sid = demo_scenario_id(return_id)
            scen_levels.setdefault(sid, Counter())[scored[return_id][1]] += 1
    for sid in sorted(scen_levels):
        dist = ", ".join(f"{lvl}:{n}" for lvl, n in scen_levels[sid].most_common())
        print(f"    [{sid}] {SCENARIO_NAMES[sid]:30s} {dist}")

    # --- [3] model-validation metrics over the full labelled dataset ----- #
    print("\n[3] Model-validation metrics (full dataset)")
    total = len(records)
    # flagged = scored at or above the suspicious threshold.
    flagged = {rid: (s >= SUSPICIOUS_THRESHOLD) for rid, (s, _, _) in scored.items()}

    normal_ids = [rid for rid, lab in labels.items() if lab == "normal"]
    fraud_ids = [rid for rid, lab in labels.items() if lab == "fraud"]

    # FP: a normal case that got flagged. Missed: a fraud case that did not.
    false_positives = sum(1 for rid in normal_ids if flagged.get(rid))
    missed = sum(1 for rid in fraud_ids if not flagged.get(rid))
    matched_levels = f"{matched}/{len(expected)}" if expected else "n/a"

    # Precision/recall of the flag threshold against the scenario labels.
    tp = len(fraud_ids) - missed
    fp = false_positives
    precision = tp / (tp + fp) if (tp + fp) else 0.0
    recall = tp / len(fraud_ids) if fraud_ids else 0.0

    print(f"    total cases ............... {total}")
    print(f"    matched expected levels ... {matched_levels} (demo set)")
    print(f"    normal cases .............. {len(normal_ids)}")
    print(f"    fraud cases ............... {len(fraud_ids)}")
    print(f"    false positives (normal) .. {false_positives}"
          + (f" ({false_positives / len(normal_ids):.1%})" if normal_ids else ""))
    print(f"    missed suspicious ......... {missed}"
          + (f" ({missed / len(fraud_ids):.1%})" if fraud_ids else ""))
    print(f"    precision @>=31 ........... {precision:.1%}")
    print(f"    recall    @>=31 ........... {recall:.1%}")

    # --- result: non-zero exit on baseline regression (for CI) ----------- #
    print("\n" + "=" * 64)
    if failures:
        print(f"BASELINE REGRESSION: {len(failures)} demo case(s) changed:")
        for f in failures[:20]:
            print(f"  - {f}")
        return 1
    print("OK - demo baseline reproduced, metrics reported.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
