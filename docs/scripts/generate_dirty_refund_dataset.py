"""
Generate "dirty" business refund datasets from the clean normalized dataset.

A dirty dataset simulates a raw CSV export from an external e-commerce
company. The point of these files is to exercise the normalization layer:
different companies use different column names (see the alias table in
docs/data-format.md) AND different, messy value formats.

The previous version only renamed columns and mapped yes/no, leaving the
values perfectly clean. That tested almost nothing. This version produces
several company profiles, each with its own:

  - column naming scheme (drawn from the documented alias table)
  - boolean encoding (yes/no, Y/N, 1/0, true/false ...)
  - decision/status vocabulary (approved/ok/accepted, declined/rejected ...)
  - date format (ISO, US slash, dotted EU, unix epoch ...)
  - money format (plain, $ + thousands separator, comma decimal ...)
  - controlled "mess": missing cells, stray whitespace, case noise,
    duplicate rows and shuffled column order.

The normalization service is expected to map every one of these back to the
internal clean refund-approval format.

Usage:
    python generate_dirty_refund_dataset.py
    python generate_dirty_refund_dataset.py --input data/clean_refund_dataset.csv \
        --output-dir data --seed 42 --mess 0.06
"""

from __future__ import annotations

import argparse
import csv
import random
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Callable, Dict, List, Optional


ISO_FORMAT = "%Y-%m-%dT%H:%M:%SZ"


# --------------------------------------------------------------------------- #
# Value formatters
# --------------------------------------------------------------------------- #

def parse_iso(value: str) -> datetime:
    return datetime.strptime(value, ISO_FORMAT)


def fmt_date_business(dt: datetime) -> str:        # 2026-06-01 09:08:00
    return dt.strftime("%Y-%m-%d %H:%M:%S")


def fmt_date_us(dt: datetime) -> str:              # 06/01/2026 09:08
    return dt.strftime("%m/%d/%Y %H:%M")


def fmt_date_eu(dt: datetime) -> str:              # 01.06.2026 09:08
    return dt.strftime("%d.%m.%Y %H:%M")


def fmt_date_epoch(dt: datetime) -> str:           # unix seconds
    return str(int(dt.timestamp()))


def fmt_money_plain(value: float) -> str:          # 1234.50
    return f"{value:.2f}"


def fmt_money_usd(value: float) -> str:            # $1,234.50
    return "${:,.2f}".format(value)


def fmt_money_eu(value: float) -> str:             # 1.234,50
    s = "{:,.2f}".format(value)
    return s.replace(",", "X").replace(".", ",").replace("X", ".")


# --------------------------------------------------------------------------- #
# Company profiles
# --------------------------------------------------------------------------- #

@dataclass
class CompanyProfile:
    name: str
    # internal clean column -> raw business column name (from the alias table)
    columns: Dict[str, str]
    true_token: str
    false_token: str
    approved_token: str
    declined_token: str
    date_fmt: Callable[[datetime], str]
    money_fmt: Callable[[float], str]
    lowercase_category: bool = False
    uppercase_category: bool = False


# Internal clean columns we map from.
CLEAN_COLUMNS = [
    "order_id", "customer_id", "return_id", "support_agent_id",
    "order_amount", "refund_amount", "product_category", "return_reason",
    "evidence_provided", "decision", "manual_override",
    "decision_time_minutes", "timestamp",
]


COMPANIES: List[CompanyProfile] = [
    # Canonical "business" export (kept for backwards compatibility with docs).
    CompanyProfile(
        name="business",
        columns={
            "order_id": "purchase_id",
            "customer_id": "buyer_id",
            "return_id": "refund_request_id",
            "support_agent_id": "agent_id",
            "order_amount": "purchase_amount",
            "refund_amount": "return_amount",
            "product_category": "category",
            "return_reason": "reason",
            "evidence_provided": "has_photo",
            "decision": "status",
            "manual_override": "override",
            "decision_time_minutes": "resolution_minutes",
            "timestamp": "created_at",
        },
        true_token="yes", false_token="no",
        approved_token="approved", declined_token="declined",
        date_fmt=fmt_date_business, money_fmt=fmt_money_plain,
        lowercase_category=True,
    ),
    # US-style retailer with different aliases and slash dates + USD money.
    CompanyProfile(
        name="shopflow",
        columns={
            "order_id": "purchase_id",
            "customer_id": "client_id",
            "return_id": "refund_request_id",
            "support_agent_id": "support_user_id",
            "order_amount": "purchase_amount",
            "refund_amount": "refund_amount",
            "product_category": "product_category",
            "return_reason": "return_reason",
            "evidence_provided": "proof_provided",
            "decision": "approval_status",
            "manual_override": "manual_override",
            "decision_time_minutes": "decision_time_minutes",
            "timestamp": "decision_time",
        },
        true_token="Y", false_token="N",
        approved_token="Approved", declined_token="Rejected",
        date_fmt=fmt_date_us, money_fmt=fmt_money_usd,
        uppercase_category=True,
    ),
    # EU-style retailer: dotted dates, comma decimals, 1/0 booleans.
    CompanyProfile(
        name="retailhub",
        columns={
            "order_id": "order_id",
            "customer_id": "customer_id",
            "return_id": "return_id",
            "support_agent_id": "agent_id",
            "order_amount": "order_amount",
            "refund_amount": "return_amount",
            "product_category": "category",
            "return_reason": "reason",
            "evidence_provided": "evidence",
            "decision": "decision",
            "manual_override": "override",
            "decision_time_minutes": "decision_time_minutes",
            "timestamp": "created_at",
        },
        true_token="1", false_token="0",
        approved_token="approve", declined_token="reject",
        date_fmt=fmt_date_eu, money_fmt=fmt_money_eu,
        lowercase_category=True,
    ),
]


# --------------------------------------------------------------------------- #
# ID rewriting (make IDs look like raw business system IDs)
# --------------------------------------------------------------------------- #

ID_REWRITES = {
    "order_id": ("order_", "purchase_"),
    "customer_id": ("customer_", "buyer_"),
    "return_id": ("return_", "refund_req_"),
}


# --------------------------------------------------------------------------- #
# Dirtying helpers
# --------------------------------------------------------------------------- #

def maybe_pad(rng: random.Random, value: str, probability: float) -> str:
    """Occasionally add stray leading/trailing whitespace."""
    if rng.random() < probability:
        pad = " " * rng.randint(1, 2)
        return f"{pad}{value}{pad}"
    return value


def maybe_case_noise(rng: random.Random, value: str, probability: float) -> str:
    """Occasionally randomize casing of a token (e.g. APPROVED / Approved)."""
    if rng.random() < probability:
        choice = rng.choice(("upper", "lower", "title"))
        if choice == "upper":
            return value.upper()
        if choice == "lower":
            return value.lower()
        return value.title()
    return value


# --------------------------------------------------------------------------- #
# Core transform
# --------------------------------------------------------------------------- #

def to_bool(raw: object) -> bool:
    return str(raw).strip().lower() in {"true", "1", "yes", "y"}


def rewrite_id(internal_col: str, value: str) -> str:
    if internal_col in ID_REWRITES:
        old, new = ID_REWRITES[internal_col]
        return value.replace(old, new)
    return value


def build_dirty_row(clean_row: Dict[str, str],
                    company: CompanyProfile,
                    rng: random.Random,
                    mess: float) -> Dict[str, str]:
    """Convert one clean row into one messy company-specific row."""
    out: Dict[str, str] = {}

    for internal_col in CLEAN_COLUMNS:
        raw = clean_row[internal_col]
        target_col = company.columns[internal_col]

        if internal_col in ("order_id", "customer_id", "return_id"):
            value = rewrite_id(internal_col, str(raw))

        elif internal_col in ("order_amount", "refund_amount"):
            value = company.money_fmt(float(raw))

        elif internal_col == "product_category":
            value = str(raw)
            if company.lowercase_category:
                value = value.lower()
            if company.uppercase_category:
                value = value.upper()
            value = maybe_case_noise(rng, value, mess)

        elif internal_col in ("evidence_provided", "manual_override"):
            value = company.true_token if to_bool(raw) else company.false_token

        elif internal_col == "decision":
            value = company.approved_token if str(raw).upper() == "APPROVED" \
                else company.declined_token
            value = maybe_case_noise(rng, value, mess)

        elif internal_col == "timestamp":
            value = company.date_fmt(parse_iso(str(raw)))

        else:  # support_agent_id, return_reason, decision_time_minutes
            value = str(raw)

        # Sprinkle stray whitespace on free-text-ish fields.
        if internal_col in ("product_category", "return_reason", "decision"):
            value = maybe_pad(rng, value, mess)

        out[target_col] = value

    # Inject missing values on a few optional fields.
    for internal_col in ("evidence_provided", "return_reason", "decision_time_minutes"):
        if rng.random() < mess:
            out[company.columns[internal_col]] = ""

    return out


def write_company_csv(path: Path,
                      clean_rows: List[Dict[str, str]],
                      company: CompanyProfile,
                      rng: random.Random,
                      mess: float) -> int:
    dirty_rows = [build_dirty_row(r, company, rng, mess) for r in clean_rows]

    # Duplicate ~1% of rows (a classic raw-export artifact).
    duplicates = [dict(r) for r in dirty_rows if rng.random() < 0.01]
    dirty_rows.extend(duplicates)
    rng.shuffle(dirty_rows)

    # Shuffle column order per company export.
    header = [company.columns[c] for c in CLEAN_COLUMNS]
    rng.shuffle(header)

    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=header)
        writer.writeheader()
        writer.writerows(dirty_rows)

    return len(dirty_rows)


# --------------------------------------------------------------------------- #
# CLI
# --------------------------------------------------------------------------- #

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate dirty business refund datasets from the clean dataset."
    )
    parser.add_argument("--input", type=str, default="data/clean_refund_dataset.csv",
                        help="Path to the clean normalized dataset.")
    parser.add_argument("--output-dir", type=str, default="data",
                        help="Directory where dirty CSV files are saved.")
    parser.add_argument("--mess", type=float, default=0.06,
                        help="Mess intensity (0-1): probability of dirt per cell/field.")
    parser.add_argument("--seed", type=int, default=42,
                        help="Random seed for reproducible dirtying.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    input_path = Path(args.input)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    if not input_path.exists():
        raise FileNotFoundError(
            f"Clean dataset not found: {input_path}. "
            f"Run generate_clean_refund_dataset.py first."
        )

    with input_path.open(newline="", encoding="utf-8") as fh:
        clean_rows = list(csv.DictReader(fh))

    rng = random.Random(args.seed)

    for company in COMPANIES:
        out_path = output_dir / f"dirty_{company.name}_refund_dataset.csv"
        n = write_company_csv(out_path, clean_rows, company, rng, args.mess)
        print(f"Created {out_path}  ({n} rows, schema='{company.name}')")

    print(f"\nGenerated {len(COMPANIES)} dirty company datasets from {len(clean_rows)} clean rows.")


if __name__ == "__main__":
    main()
