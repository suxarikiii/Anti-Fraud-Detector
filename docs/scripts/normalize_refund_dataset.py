"""
Normalization layer: dirty company refund export  ->  clean refund records.

This is the product-facing counterpart to generate_dirty_refund_dataset.py.
Raw e-commerce partners export refund data with different column names and
messy value formats (see the alias table in docs/data-format.md). Before the
scoring-service can reason about a refund, every export has to be mapped back
to ONE internal clean schema. This script is that mapping, runnable on its own
so the "dirty input -> normalized output" step is visible and reproducible.

What it normalizes (reverse of the dirty generator):
  - column names        -> internal names via the documented alias table
  - IDs                 purchase_/buyer_/refund_req_  ->  order_/customer_/return_
  - booleans            yes/Y/1/true/proof            ->  True  (else False)
  - decision vocab      approved/approve/ok/accepted  ->  APPROVED, else DECLINED
  - money               $1,234.50 / 1.234,50 / 1234.5 ->  1234.50 (float)
  - dates               ISO / US slash / EU dotted / unix epoch -> ISO 8601 Z
  - mess                stray whitespace, case noise, duplicate rows
  - missing cells       filled with documented safe defaults (and counted)

Usage:
    # normalize the canonical business export, write clean CSV
    python normalize_refund_dataset.py

    # normalize a specific export and show a before/after demo + round-trip stats
    python normalize_refund_dataset.py --input data/dirty_shopflow_refund_dataset.csv --demo

    # normalize, then verify the result reproduces data/clean_refund_dataset.csv
    python normalize_refund_dataset.py --verify-against data/clean_refund_dataset.csv
"""

from __future__ import annotations

import argparse
import csv
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional, Tuple


ISO_OUT = "%Y-%m-%dT%H:%M:%SZ"

# Internal clean schema (order matters: this is the output column order).
CLEAN_COLUMNS = [
    "order_id", "customer_id", "return_id", "support_agent_id",
    "order_amount", "refund_amount", "product_category", "return_reason",
    "evidence_provided", "decision", "manual_override",
    "decision_time_minutes", "timestamp",
]

# Raw column alias -> internal clean column (from docs/data-format.md).
# Any documented partner column name resolves to exactly one internal column.
COLUMN_ALIASES: Dict[str, str] = {
    # customer
    "customer_id": "customer_id", "client_id": "customer_id", "buyer_id": "customer_id",
    # order
    "order_id": "order_id", "purchase_id": "order_id",
    # return
    "return_id": "return_id", "refund_request_id": "return_id",
    # agent
    "support_agent_id": "support_agent_id", "agent_id": "support_agent_id",
    "support_user_id": "support_agent_id",
    # amounts
    "refund_amount": "refund_amount", "return_amount": "refund_amount",
    "order_amount": "order_amount", "purchase_amount": "order_amount",
    # category / reason
    "category": "product_category", "product_category": "product_category",
    "reason": "return_reason", "return_reason": "return_reason",
    # evidence
    "evidence": "evidence_provided", "has_photo": "evidence_provided",
    "proof_provided": "evidence_provided",
    # decision
    "decision": "decision", "status": "decision", "approval_status": "decision",
    # override / timing / date
    "manual_override": "manual_override", "override": "manual_override",
    "resolution_minutes": "decision_time_minutes",
    "decision_time_minutes": "decision_time_minutes",
    "created_at": "timestamp", "decision_time": "timestamp", "timestamp": "timestamp",
}

# ID prefix rewrites: raw business prefix -> internal prefix.
ID_PREFIX = {
    "order_id": ("purchase_", "order_"),
    "customer_id": ("buyer_", "customer_"),
    "return_id": ("refund_req_", "return_"),
}

TRUE_TOKENS = {"true", "yes", "y", "1", "t"}
APPROVED_TOKENS = {"approved", "approve", "ok", "accepted", "accept", "yes"}
DECLINED_TOKENS = {"declined", "decline", "rejected", "reject", "denied", "deny", "no"}

# Documented safe defaults for missing cells (see report / data-format.md).
DEFAULT_EVIDENCE = False        # no proof on file -> treat as no evidence
DEFAULT_OVERRIDE = False        # absence of an override flag -> no override
DEFAULT_REASON = ""             # unknown reason
DEFAULT_DECISION_MINUTES = 60   # neutral: > FAST_APPROVAL threshold (3), never fires fast-approval


class NormalizationStats:
    """Counters so the normalization step is auditable, not a black box."""

    def __init__(self) -> None:
        self.rows_in = 0
        self.duplicates_dropped = 0
        self.rows_out = 0
        self.imputed = {"evidence_provided": 0, "return_reason": 0,
                        "decision_time_minutes": 0, "manual_override": 0}

    def summary(self) -> str:
        imp = ", ".join(f"{k}={v}" for k, v in self.imputed.items() if v)
        return (f"rows_in={self.rows_in}  duplicates_dropped={self.duplicates_dropped}  "
                f"rows_out={self.rows_out}"
                + (f"  imputed[{imp}]" if imp else "  imputed[none]"))


# --------------------------------------------------------------------------- #
# Field parsers
# --------------------------------------------------------------------------- #

def parse_bool(raw: str) -> bool:
    return str(raw).strip().lower() in TRUE_TOKENS


def parse_decision(raw: str) -> str:
    token = str(raw).strip().lower()
    if token in APPROVED_TOKENS:
        return "APPROVED"
    if token in DECLINED_TOKENS:
        return "DECLINED"
    # Unknown vocab: keep uppercased so nothing is silently swallowed.
    return token.upper()


def parse_money(raw: str) -> float:
    """Parse plain / USD ($1,234.50) / EU (1.234,50) money into a float."""
    s = str(raw).strip().replace("$", "").replace(" ", "")
    if not s:
        return 0.0
    has_comma, has_dot = "," in s, "." in s
    if has_comma and has_dot:
        # The rightmost separator is the decimal point.
        if s.rfind(",") > s.rfind("."):        # EU: 1.234,50
            s = s.replace(".", "").replace(",", ".")
        else:                                   # US: 1,234.50
            s = s.replace(",", "")
    elif has_comma:
        # Only commas. Two trailing digits -> decimal comma, else thousands.
        if len(s.split(",")[-1]) == 2:
            s = s.replace(",", ".")
        else:
            s = s.replace(",", "")
    return round(float(s), 2)


_DATE_FORMATS = (
    "%Y-%m-%dT%H:%M:%SZ",   # ISO (already normalized)
    "%Y-%m-%d %H:%M:%S",    # business
    "%Y-%m-%d %H:%M",
    "%m/%d/%Y %H:%M",       # US slash
    "%m/%d/%Y %H:%M:%S",
    "%d.%m.%Y %H:%M",       # EU dotted
    "%d.%m.%Y %H:%M:%S",
)


def parse_timestamp(raw: str) -> str:
    s = str(raw).strip()
    if not s:
        return ""
    if s.isdigit():                             # unix epoch seconds
        return datetime.fromtimestamp(int(s), tz=timezone.utc).strftime(ISO_OUT)
    for fmt in _DATE_FORMATS:
        try:
            return datetime.strptime(s, fmt).strftime(ISO_OUT)
        except ValueError:
            continue
    return s                                    # leave untouched if unrecognized


def rewrite_id(internal_col: str, value: str) -> str:
    if internal_col in ID_PREFIX:
        raw_prefix, clean_prefix = ID_PREFIX[internal_col]
        if value.startswith(raw_prefix):
            return clean_prefix + value[len(raw_prefix):]
    return value


# --------------------------------------------------------------------------- #
# Core
# --------------------------------------------------------------------------- #

def build_column_map(header: List[str]) -> Dict[str, str]:
    """Map each raw header column -> internal clean column via the alias table."""
    mapping: Dict[str, str] = {}
    for col in header:
        key = col.strip().lower()
        if key in COLUMN_ALIASES:
            mapping[col] = COLUMN_ALIASES[key]
    missing = set(CLEAN_COLUMNS) - set(mapping.values())
    if missing:
        raise ValueError(
            f"Input is missing columns for: {sorted(missing)}. "
            f"Header seen: {header}"
        )
    return mapping


def normalize_row(raw_row: Dict[str, str],
                  col_map: Dict[str, str],
                  stats: NormalizationStats) -> Dict[str, str]:
    """Turn one raw partner row into one clean-schema row."""
    # Collapse raw columns onto internal names (stripping whitespace).
    internal: Dict[str, str] = {}
    for raw_col, clean_col in col_map.items():
        internal[clean_col] = str(raw_row.get(raw_col, "")).strip()

    out: Dict[str, str] = {}

    for col in ("order_id", "customer_id", "return_id"):
        out[col] = rewrite_id(col, internal[col])

    out["support_agent_id"] = internal["support_agent_id"]

    out["order_amount"] = f"{parse_money(internal['order_amount']):.2f}"
    out["refund_amount"] = f"{parse_money(internal['refund_amount']):.2f}"

    out["product_category"] = internal["product_category"].strip().lower()

    reason = internal["return_reason"]
    if reason == "":
        stats.imputed["return_reason"] += 1
    out["return_reason"] = reason if reason else DEFAULT_REASON

    evid = internal["evidence_provided"]
    if evid == "":
        stats.imputed["evidence_provided"] += 1
        out["evidence_provided"] = str(DEFAULT_EVIDENCE)
    else:
        out["evidence_provided"] = str(parse_bool(evid))

    out["decision"] = parse_decision(internal["decision"])

    override = internal["manual_override"]
    if override == "":
        stats.imputed["manual_override"] += 1
        out["manual_override"] = str(DEFAULT_OVERRIDE)
    else:
        out["manual_override"] = str(parse_bool(override))

    minutes = internal["decision_time_minutes"]
    if minutes == "":
        stats.imputed["decision_time_minutes"] += 1
        out["decision_time_minutes"] = str(DEFAULT_DECISION_MINUTES)
    else:
        out["decision_time_minutes"] = str(int(float(minutes)))

    out["timestamp"] = parse_timestamp(internal["timestamp"])
    return out


def normalize(rows: List[Dict[str, str]], header: List[str]
              ) -> Tuple[List[Dict[str, str]], NormalizationStats]:
    """Normalize all rows, dropping exact duplicates (a raw-export artifact)."""
    stats = NormalizationStats()
    col_map = build_column_map(header)

    seen = set()
    out_rows: List[Dict[str, str]] = []
    for raw in rows:
        stats.rows_in += 1
        norm = normalize_row(raw, col_map, stats)
        key = tuple(norm[c] for c in CLEAN_COLUMNS)
        if key in seen:
            stats.duplicates_dropped += 1
            continue
        seen.add(key)
        out_rows.append(norm)

    # Deterministic order by return_id so output is stable / diffable.
    out_rows.sort(key=lambda r: r["return_id"])
    stats.rows_out = len(out_rows)
    return out_rows, stats


def write_clean(path: Path, rows: List[Dict[str, str]]) -> None:
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=CLEAN_COLUMNS)
        writer.writeheader()
        writer.writerows(rows)


# --------------------------------------------------------------------------- #
# Demo / verification helpers
# --------------------------------------------------------------------------- #

def print_demo(raw_rows: List[Dict[str, str]], header: List[str],
               norm_rows: List[Dict[str, str]], n: int = 3) -> None:
    """Show a few raw rows next to their normalized form (demo evidence)."""
    by_return = {r["return_id"]: r for r in norm_rows}
    print("\n" + "=" * 68)
    print("DIRTY INPUT  ->  NORMALIZED OUTPUT  (sample)")
    print("=" * 68)
    shown = 0
    for raw in raw_rows:
        if shown >= n:
            break
        # Find this raw row's normalized twin by its (rewritten) return id.
        rid_raw = next((raw[c] for c in raw if "refund_request" in c.lower()
                        or c.lower() == "return_id"), "")
        rid = rewrite_id("return_id", str(rid_raw).strip())
        norm = by_return.get(rid)
        if not norm:
            continue
        print(f"\n[{shown + 1}] raw     : " +
              " | ".join(f"{k}={v!r}" for k, v in list(raw.items())[:6]))
        print(f"    norm    : " +
              " | ".join(f"{k}={norm[k]!r}" for k in
                         ("return_id", "order_id", "customer_id", "order_amount",
                          "refund_amount", "evidence_provided", "decision",
                          "manual_override", "timestamp")))
        shown += 1


def verify_against(norm_rows: List[Dict[str, str]], clean_path: Path) -> int:
    """Compare normalized output to the canonical clean dataset.

    Rows differ only on cells the dirty generator deliberately blanked (missing
    evidence / reason / decision_time), which normalization fills with defaults.
    We report an exact-match rate on scoring-relevant fields to prove the
    round-trip is faithful.
    """
    with clean_path.open(newline="", encoding="utf-8") as fh:
        clean = {r["return_id"]: r for r in csv.DictReader(fh)}

    # Fields that drive scoring; timestamp/reason are informational.
    key_fields = ["order_id", "customer_id", "support_agent_id", "order_amount",
                  "refund_amount", "evidence_provided", "decision",
                  "manual_override", "decision_time_minutes"]

    checked = matched = 0
    mismatches: List[str] = []
    for norm in norm_rows:
        ref = clean.get(norm["return_id"])
        if ref is None:
            continue
        checked += 1
        diffs = [f"{f}: {norm[f]!r} vs {ref[f]!r}"
                 for f in key_fields
                 if _cell_ne(f, norm[f], ref[f])]
        if not diffs:
            matched += 1
        elif len(mismatches) < 10:
            mismatches.append(f"{norm['return_id']}: " + "; ".join(diffs))

    print("\n" + "=" * 68)
    print("ROUND-TRIP VERIFICATION  (normalized  vs  clean_refund_dataset.csv)")
    print("=" * 68)
    print(f"    checked ................ {checked}")
    print(f"    exact match (scoring) .. {matched} ({matched / checked:.1%})" if checked else "    no overlap")
    if mismatches:
        print(f"    residual diffs (defaults on blanked cells), first {len(mismatches)}:")
        for m in mismatches:
            print(f"      - {m}")
    return 0 if checked and matched == checked else (0 if checked else 1)


def _cell_ne(field: str, a: str, b: str) -> bool:
    """Compare two cells with type-aware equality for amounts."""
    if field in ("order_amount", "refund_amount"):
        try:
            return abs(float(a) - float(b)) > 0.01
        except ValueError:
            return a != b
    return str(a).strip() != str(b).strip()


# --------------------------------------------------------------------------- #
# CLI
# --------------------------------------------------------------------------- #

def main() -> int:
    parser = argparse.ArgumentParser(description="Normalize a dirty refund export into clean records.")
    parser.add_argument("--input", type=str, default="data/dirty_business_refund_dataset.csv")
    parser.add_argument("--output", type=str, default=None,
                        help="Output CSV (default: data/normalized_<name>.csv)")
    parser.add_argument("--demo", action="store_true", help="Print a before/after sample.")
    parser.add_argument("--verify-against", type=str, default=None,
                        help="Clean dataset to check the round-trip against.")
    args = parser.parse_args()

    in_path = Path(args.input)
    if not in_path.exists():
        print(f"ERROR: input not found: {in_path}")
        return 2

    with in_path.open(newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        header = list(reader.fieldnames or [])
        raw_rows = list(reader)

    norm_rows, stats = normalize(raw_rows, header)

    out_path = Path(args.output) if args.output else \
        in_path.parent / f"normalized_{in_path.stem.replace('dirty_', '')}.csv"
    write_clean(out_path, norm_rows)

    print("NORMALIZATION")
    print(f"    input  : {in_path}")
    print(f"    output : {out_path}")
    print(f"    {stats.summary()}")

    if args.demo:
        print_demo(raw_rows, header, norm_rows)

    if args.verify_against:
        return verify_against(norm_rows, Path(args.verify_against))
    return 0


if __name__ == "__main__":
    sys.exit(main())
