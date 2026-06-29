"""
Generate a clean synthetic e-commerce refund dataset for the
Fraud & Abuse Detection System MVP.

The MVP domain is detecting suspicious refund approvals in e-commerce
customer support workflows.

This generator produces two files:

1. clean_refund_dataset.csv
   - Main backend-ready dataset (only the normalized business columns).
   - ~5000 rows by default, with a realistic class balance:
     60% normal refund handling, 40% suspicious/fraud cases.
   - Contains BOTH APPROVED and DECLINED decisions, so that agent
     approval-rate signals are actually meaningful.

2. dataset_labels.csv
   - Ground-truth evaluation file (NOT consumed by backend services).
   - Maps every order/return to its scenario, a binary label
     (normal/fraud) and the scoring rules the case is designed to trigger.
   - Use this to measure precision/recall of the scoring service.

Design goals that fix the previous version:
- Real volume (~5000) instead of 45 rows.
- Realistic 60/40 normal/fraud balance instead of an inverted one.
- DECLINED decisions exist, so AGENT_HIGH_APPROVAL_RATE is detectable.
- Ground-truth labels for model evaluation.
- Graph density: customers place many orders, frequent returners and
  agent-customer rings recur, instead of a rigid 1:1 order<->customer map.
- Interleaved timestamps across a date window instead of one-scenario-per-day.
- Every scoring rule is guaranteed to be triggered (checked by the
  validation harness).

Usage:
    python generate_clean_refund_dataset.py

Optional:
    python generate_clean_refund_dataset.py \
        --output-dir data --total-rows 5000 --fraud-ratio 0.40 --seed 42
"""

from __future__ import annotations

import argparse
import csv
import io
import random
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from pathlib import Path
from typing import Callable, Dict, List, Optional, Tuple


# --------------------------------------------------------------------------- #
# Output schemas
# --------------------------------------------------------------------------- #

CLEAN_COLUMNS = [
    "order_id",
    "customer_id",
    "return_id",
    "support_agent_id",
    "order_amount",
    "refund_amount",
    "product_category",
    "return_reason",
    "evidence_provided",
    "decision",
    "manual_override",
    "decision_time_minutes",
    "timestamp",
]

LABEL_COLUMNS = [
    "order_id",
    "return_id",
    "customer_id",
    "support_agent_id",
    "scenario_id",
    "scenario_name",
    "label",            # "normal" | "fraud"
    "expected_rules",   # ";"-separated rule names this case is designed to fire
    "decision",
]

# --------------------------------------------------------------------------- #
# Scoring thresholds (must match the scoring service configuration)
# Documented here so the generator can guarantee each rule is triggerable.
# --------------------------------------------------------------------------- #

@dataclass(frozen=True)
class Thresholds:
    high_value_refund: float = 500.0     # refund_amount >= -> HIGH_VALUE_REFUND
    full_amount_ratio: float = 0.95      # refund/order  >= -> FULL_AMOUNT_REFUND
    fast_approval_minutes: int = 5       # decision_time <= -> FAST_APPROVAL


THRESHOLDS = Thresholds()


# --------------------------------------------------------------------------- #
# Scenarios
# --------------------------------------------------------------------------- #

SCENARIOS: Dict[int, str] = {
    1: "Normal refund approval with evidence and reasonable timing",
    2: "High-value refund approved without evidence",
    3: "Full-amount refund approved for expensive order",
    4: "Very fast approval by support agent",
    5: "Manual override on high-value refund",
    6: "Customer with frequent refund requests",
    7: "Support agent with unusually high approval rate",
    8: "Repeated agent-customer approval pattern",
    9: "Suspicious cluster: same agent, frequent customer returns, manual overrides",
}

# Rules each scenario is designed to trigger (see docs/scoring-rules.md).
EXPECTED_RULES: Dict[int, Tuple[str, ...]] = {
    1: (),
    2: ("NO_EVIDENCE", "HIGH_VALUE_REFUND"),
    3: ("FULL_AMOUNT_REFUND", "HIGH_VALUE_REFUND"),
    4: ("FAST_APPROVAL",),
    5: ("MANUAL_OVERRIDE", "HIGH_VALUE_REFUND"),
    6: ("CUSTOMER_FREQUENT_RETURNS",),
    7: ("AGENT_HIGH_APPROVAL_RATE",),
    8: ("REPEATED_AGENT_CUSTOMER_PAIR",),
    9: ("SUSPICIOUS_CLUSTER", "NO_EVIDENCE", "MANUAL_OVERRIDE", "HIGH_VALUE_REFUND"),
}

# Relative weights for distributing the fraud budget across scenarios 2..9.
FRAUD_SCENARIO_WEIGHTS: Dict[int, float] = {
    2: 0.14,
    3: 0.14,
    4: 0.14,
    5: 0.14,
    6: 0.12,
    7: 0.12,
    8: 0.10,
    9: 0.10,
}


# --------------------------------------------------------------------------- #
# Legacy fixture block
#
# The scoring-service tests are pinned to these exact historical records
# (specific order/return/agent ids and values). They are embedded verbatim and
# always emitted first so that regenerating the large dataset never breaks the
# backend tests. The rows are 5 per scenario, in scenario order 1..9, so the
# scenario id can be derived from the row index.
#
# NOTE: the special entities here (agent_777, agent_888, agent_999,
# customer_888, customer_900, customer_999) are intentionally NOT reused by the
# synthetic generator, so their aggregate signals (approval rate, frequency,
# repeated pairs, cluster) stay identical to the original fixture.
# --------------------------------------------------------------------------- #

LEGACY_DATASET_CSV = """\
order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,manual_override,decision_time_minutes,timestamp
order_1001,customer_200,return_3001,agent_001,203.84,199.57,clothing,changed_mind,True,APPROVED,False,64,2026-06-01T09:06:00Z
order_1002,customer_201,return_3002,agent_005,143.16,131.27,sports,item_not_as_described,True,APPROVED,False,35,2026-06-01T09:22:00Z
order_1003,customer_202,return_3003,agent_001,41.07,15.85,beauty,changed_mind,True,APPROVED,False,42,2026-06-01T09:47:00Z
order_1004,customer_203,return_3004,agent_002,81.12,73.92,beauty,allergic_reaction,True,APPROVED,False,80,2026-06-01T10:08:00Z
order_1005,customer_204,return_3005,agent_002,585.04,374.1,home,item_not_as_described,True,APPROVED,False,86,2026-06-01T10:23:00Z
order_1006,customer_300,return_3006,agent_101,2968.28,1855.12,luxury,item_not_as_described,False,APPROVED,False,30,2026-06-02T09:02:00Z
order_1007,customer_301,return_3007,agent_005,1570.12,708.57,appliances,damaged_item,False,APPROVED,False,30,2026-06-02T09:27:00Z
order_1008,customer_302,return_3008,agent_002,1568.21,1321.3,electronics,damaged_item,False,APPROVED,False,36,2026-06-02T09:49:00Z
order_1009,customer_303,return_3009,agent_001,910.49,736.87,appliances,damaged_item,False,APPROVED,False,16,2026-06-02T10:15:00Z
order_1010,customer_304,return_3010,agent_005,651.98,479.91,luxury,damaged_item,False,APPROVED,False,35,2026-06-02T10:40:00Z
order_1011,customer_400,return_3011,agent_101,1213.2,1213.2,electronics,missing_accessory,False,APPROVED,False,25,2026-06-03T09:00:00Z
order_1012,customer_401,return_3012,agent_005,1146.04,1146.04,appliances,damaged_item,True,APPROVED,True,61,2026-06-03T09:32:00Z
order_1013,customer_402,return_3013,agent_005,1803.3,1803.3,luxury,changed_mind,False,APPROVED,False,60,2026-06-03T10:08:00Z
order_1014,customer_403,return_3014,agent_001,2157.69,2157.69,luxury,changed_mind,True,APPROVED,False,39,2026-06-03T10:25:00Z
order_1015,customer_404,return_3015,agent_005,949.24,949.24,appliances,damaged_item,False,APPROVED,False,38,2026-06-03T11:01:00Z
order_1016,customer_500,return_3016,agent_101,472.24,299.92,fashion,item_not_as_described,False,APPROVED,False,2,2026-06-04T09:01:00Z
order_1017,customer_501,return_3017,agent_102,462.92,420.02,home,item_not_as_described,True,APPROVED,False,6,2026-06-04T09:17:00Z
order_1018,customer_502,return_3018,agent_102,163.81,135.58,clothing,changed_mind,True,APPROVED,False,1,2026-06-04T09:31:00Z
order_1019,customer_503,return_3019,agent_101,2605.08,2552.17,luxury,damaged_item,True,APPROVED,False,5,2026-06-04T09:47:00Z
order_1020,customer_504,return_3020,agent_101,1058.6,896.76,electronics,missing_accessory,False,APPROVED,False,3,2026-06-04T10:04:00Z
order_1021,customer_600,return_3021,agent_005,1170.27,772.55,luxury,damaged_item,False,APPROVED,True,4,2026-06-05T09:00:00Z
order_1022,customer_601,return_3022,agent_002,2318.51,1564.59,luxury,changed_mind,False,APPROVED,True,11,2026-06-05T09:29:00Z
order_1023,customer_602,return_3023,agent_102,1808.83,1331.79,electronics,item_not_as_described,False,APPROVED,True,7,2026-06-05T09:50:00Z
order_1024,customer_603,return_3024,agent_004,1697.4,1345.97,electronics,damaged_item,False,APPROVED,True,26,2026-06-05T10:11:00Z
order_1025,customer_604,return_3025,agent_102,1596.67,1330.79,electronics,missing_accessory,False,APPROVED,True,25,2026-06-05T10:30:00Z
order_1026,customer_900,return_3026,agent_003,28.76,14.42,clothing,wrong_size,True,APPROVED,False,20,2026-06-06T09:08:00Z
order_1027,customer_900,return_3027,agent_002,601.06,430.7,fashion,changed_mind,False,APPROVED,False,31,2026-06-07T09:17:00Z
order_1028,customer_900,return_3028,agent_003,131.37,125.29,fashion,item_not_as_described,True,APPROVED,False,37,2026-06-08T09:27:00Z
order_1029,customer_900,return_3029,agent_004,307.12,209.57,electronics,missing_accessory,True,APPROVED,True,65,2026-06-09T09:42:00Z
order_1030,customer_900,return_3030,agent_102,56.28,35.61,books,damaged_item,False,APPROVED,False,45,2026-06-10T09:52:00Z
order_1031,customer_700,return_3031,agent_777,1065.95,881.61,appliances,damaged_item,False,APPROVED,True,22,2026-06-11T09:03:00Z
order_1032,customer_701,return_3032,agent_777,16.66,16.26,books,damaged_item,True,APPROVED,False,31,2026-06-11T09:23:00Z
order_1033,customer_702,return_3033,agent_777,854.18,492.36,home,damaged_item,False,APPROVED,False,13,2026-06-11T09:37:00Z
order_1034,customer_703,return_3034,agent_777,1373.6,1141.21,electronics,item_not_as_described,True,APPROVED,False,35,2026-06-11T10:02:00Z
order_1035,customer_704,return_3035,agent_777,282.07,135.4,sports,wrong_size,True,APPROVED,False,5,2026-06-11T10:13:00Z
order_1036,customer_888,return_3036,agent_888,665.92,607.0,sports,item_not_as_described,False,APPROVED,False,27,2026-06-12T09:06:00Z
order_1037,customer_888,return_3037,agent_888,1773.38,1161.28,electronics,item_not_as_described,False,APPROVED,False,10,2026-06-13T09:16:00Z
order_1038,customer_888,return_3038,agent_888,1331.41,1158.5,electronics,not_working,True,APPROVED,False,4,2026-06-14T09:21:00Z
order_1039,customer_888,return_3039,agent_888,1364.69,875.48,electronics,not_working,False,APPROVED,True,16,2026-06-15T09:32:00Z
order_1040,customer_888,return_3040,agent_888,430.66,428.9,sports,item_not_as_described,False,APPROVED,False,24,2026-06-16T09:43:00Z
order_1041,customer_999,return_3041,agent_999,1168.27,1019.25,electronics,item_not_as_described,False,APPROVED,True,4,2026-06-17T09:01:00Z
order_1042,customer_999,return_3042,agent_999,1182.59,968.11,luxury,item_not_as_described,False,APPROVED,True,5,2026-06-18T09:10:00Z
order_1043,customer_999,return_3043,agent_999,872.0,777.01,appliances,not_working,False,APPROVED,True,5,2026-06-19T09:19:00Z
order_1044,customer_999,return_3044,agent_999,916.25,831.78,appliances,missing_part,False,APPROVED,True,8,2026-06-20T09:28:00Z
order_1045,customer_999,return_3045,agent_999,1099.08,996.79,appliances,not_working,False,APPROVED,True,2,2026-06-21T09:37:00Z
"""


# --------------------------------------------------------------------------- #
# Profiles
# --------------------------------------------------------------------------- #

@dataclass(frozen=True)
class ProductCategoryProfile:
    name: str
    min_amount: float
    max_amount: float
    common_reasons: Tuple[str, ...]
    evidence_probability: float


@dataclass(frozen=True)
class AgentProfile:
    agent_id: str
    approval_probability: float
    manual_override_probability: float
    min_decision_time: int
    max_decision_time: int


PRODUCT_PROFILES: List[ProductCategoryProfile] = [
    ProductCategoryProfile("electronics", 120, 2200,
                           ("not_working", "damaged_item", "missing_accessory", "item_not_as_described"), 0.55),
    ProductCategoryProfile("clothing", 20, 260,
                           ("wrong_size", "changed_mind", "item_not_as_described"), 0.75),
    ProductCategoryProfile("home", 35, 600,
                           ("damaged_item", "missing_part", "item_not_as_described"), 0.70),
    ProductCategoryProfile("beauty", 15, 180,
                           ("allergic_reaction", "damaged_item", "changed_mind"), 0.65),
    ProductCategoryProfile("sports", 25, 450,
                           ("wrong_size", "damaged_item", "item_not_as_described"), 0.72),
    ProductCategoryProfile("books", 8, 90,
                           ("wrong_item_received", "damaged_item", "changed_mind"), 0.85),
    ProductCategoryProfile("appliances", 180, 1800,
                           ("not_working", "damaged_item", "missing_part"), 0.50),
    ProductCategoryProfile("luxury", 350, 3000,
                           ("item_not_as_described", "changed_mind", "damaged_item"), 0.45),
    ProductCategoryProfile("fashion", 40, 650,
                           ("wrong_size", "changed_mind", "item_not_as_described"), 0.68),
]

HIGH_VALUE_CATEGORIES = ("electronics", "appliances", "luxury", "fashion")


# --------------------------------------------------------------------------- #
# Helpers
# --------------------------------------------------------------------------- #

def money(value: float) -> float:
    return round(value, 2)


def chance(rng: random.Random, probability: float) -> bool:
    return rng.random() < probability


def choose_category(rng: random.Random,
                    allowed_names: Optional[Tuple[str, ...]] = None) -> ProductCategoryProfile:
    if allowed_names is None:
        return rng.choice(PRODUCT_PROFILES)
    allowed = [p for p in PRODUCT_PROFILES if p.name in allowed_names]
    return rng.choice(allowed)


def order_amount(rng: random.Random,
                 category: ProductCategoryProfile,
                 force_high_value: bool = False) -> float:
    lo, hi = category.min_amount, category.max_amount
    if force_high_value:
        # Guarantee that a refund of >=60% of the order clears the
        # HIGH_VALUE_REFUND threshold.
        lo = max(lo, 900.0)
        hi = max(hi, lo + 400.0)
    return money(rng.uniform(lo, hi))


def refund_amount(rng: random.Random,
                  base_order_amount: float,
                  min_ratio: float,
                  max_ratio: float,
                  full_refund: bool = False) -> float:
    if full_refund:
        return money(base_order_amount)
    return money(base_order_amount * rng.uniform(min_ratio, max_ratio))


# --------------------------------------------------------------------------- #
# Row container
# --------------------------------------------------------------------------- #

@dataclass
class GeneratedRow:
    data: Dict[str, object]
    scenario_id: int
    label: str


# --------------------------------------------------------------------------- #
# Generator
# --------------------------------------------------------------------------- #

class RefundDatasetGenerator:
    """Synthetic refund dataset generator with scenario + label guarantees."""

    def __init__(self, total_rows: int = 5000, fraud_ratio: float = 0.40,
                 seed: int = 42) -> None:
        if not 0.0 < fraud_ratio < 1.0:
            raise ValueError("fraud_ratio must be between 0 and 1 (exclusive).")
        if total_rows < 200:
            raise ValueError("total_rows should be at least 200 for meaningful coverage.")

        self.total_rows = total_rows
        self.fraud_ratio = fraud_ratio
        self.rng = random.Random(seed)

        self.window_start = datetime(2026, 4, 1, 8, 0, 0)
        self.window_days = 80

        self.order_counter = 100000
        self.return_counter = 300000

        self.rows: List[GeneratedRow] = []

        # Entity pools (built deterministically from the seed).
        self.normal_agents = self._build_normal_agents(30)
        self.fast_agents = [
            AgentProfile("agent_101", 0.92, 0.10, 1, 5),
            AgentProfile("agent_102", 0.90, 0.12, 1, 4),
            AgentProfile("agent_103", 0.93, 0.08, 1, 5),
        ]
        self.high_approval_agents = [
            AgentProfile("agent_701", 0.99, 0.16, 5, 35),
            AgentProfile("agent_702", 0.98, 0.14, 6, 40),
            AgentProfile("agent_703", 0.99, 0.18, 4, 30),
        ]
        # Repeated agent<->customer pairs (scenario 8).
        self.repeated_pairs = [
            (AgentProfile("agent_801", 0.96, 0.12, 4, 28), "customer_8001"),
            (AgentProfile("agent_802", 0.95, 0.14, 5, 30), "customer_8002"),
            (AgentProfile("agent_803", 0.97, 0.10, 4, 26), "customer_8003"),
        ]
        # Suspicious clusters (scenario 9): one bad agent + a small ring of customers.
        self.clusters = [
            (AgentProfile("agent_901", 0.99, 0.85, 1, 8),
             ["customer_9901", "customer_9902", "customer_9903"]),
            (AgentProfile("agent_902", 0.99, 0.80, 1, 9),
             ["customer_9904", "customer_9905", "customer_9906"]),
        ]
        # Frequent returners (scenario 6).
        self.frequent_customers = [f"customer_90{idx:02d}" for idx in range(1, 9)]
        # Large normal customer pool (reused -> many orders per customer).
        self.normal_customers = [f"customer_{idx:04d}" for idx in range(1, 601)]

    # ---- pools ---------------------------------------------------------- #

    def _build_normal_agents(self, count: int) -> List[AgentProfile]:
        agents: List[AgentProfile] = []
        for i in range(1, count + 1):
            agents.append(
                AgentProfile(
                    agent_id=f"agent_{i:03d}",
                    approval_probability=self.rng.uniform(0.55, 0.82),
                    manual_override_probability=self.rng.uniform(0.02, 0.08),
                    min_decision_time=self.rng.randint(20, 45),
                    max_decision_time=self.rng.randint(70, 130),
                )
            )
        return agents

    # ---- ids / time ----------------------------------------------------- #

    def _next_order_id(self) -> str:
        self.order_counter += 1
        return f"order_{self.order_counter}"

    def _next_return_id(self) -> str:
        self.return_counter += 1
        return f"return_{self.return_counter}"

    def _random_timestamp(self, day_lo: int = 0, day_hi: Optional[int] = None) -> str:
        day_hi = self.window_days if day_hi is None else day_hi
        day = self.rng.randint(day_lo, day_hi)
        minute = self.rng.randint(0, 24 * 60 - 1)
        ts = self.window_start + timedelta(days=day, minutes=minute)
        return ts.strftime("%Y-%m-%dT%H:%M:%SZ")

    # ---- row builder ---------------------------------------------------- #

    def _emit(self, *, scenario_id: int, label: str, customer_id: str,
              agent: AgentProfile, category: ProductCategoryProfile,
              order_amt: float, refund_amt: float, return_reason: str,
              evidence_provided: bool, decision: str, manual_override: bool,
              decision_time_minutes: int, timestamp: str) -> None:
        order_id = self._next_order_id()
        return_id = self._next_return_id()
        row = {
            "order_id": order_id,
            "customer_id": customer_id,
            "return_id": return_id,
            "support_agent_id": agent.agent_id,
            "order_amount": money(order_amt),
            "refund_amount": money(refund_amt),
            "product_category": category.name,
            "return_reason": return_reason,
            "evidence_provided": evidence_provided,
            "decision": decision,
            "manual_override": manual_override,
            "decision_time_minutes": decision_time_minutes,
            "timestamp": timestamp,
        }
        self.rows.append(GeneratedRow(data=row, scenario_id=scenario_id, label=label))

    # ---- scenario 1: normal background (APPROVED + DECLINED) ------------ #

    def generate_normal(self, count: int) -> None:
        for _ in range(count):
            category = choose_category(self.rng)
            agent = self.rng.choice(self.normal_agents)
            customer = self.rng.choice(self.normal_customers)

            # ~10% of legitimate orders are genuinely high value (realistic noise).
            high_value = chance(self.rng, 0.10)
            amt = order_amount(self.rng, category, force_high_value=high_value)
            ref = refund_amount(self.rng, amt, 0.20, 0.85)

            approved = chance(self.rng, agent.approval_probability)
            decision = "APPROVED" if approved else "DECLINED"

            # Declined requests often lack evidence; approved ones usually have it.
            if approved:
                evidence = chance(self.rng, max(0.6, category.evidence_probability))
            else:
                evidence = chance(self.rng, category.evidence_probability * 0.5)

            self._emit(
                scenario_id=1,
                label="normal",
                customer_id=customer,
                agent=agent,
                category=category,
                order_amt=amt,
                refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=evidence,
                decision=decision,
                manual_override=chance(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(agent.min_decision_time,
                                                        agent.max_decision_time),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 2: high-value refund approved without evidence -------- #

    def generate_high_value_no_evidence(self, count: int) -> None:
        for _ in range(count):
            category = choose_category(self.rng, ("electronics", "appliances", "luxury"))
            agent = self.rng.choice(self.normal_agents + self.fast_agents)
            amt = order_amount(self.rng, category, force_high_value=True)
            ref = refund_amount(self.rng, amt, 0.60, 0.95)
            self._emit(
                scenario_id=2, label="fraud",
                customer_id=self.rng.choice(self.normal_customers),
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=False, decision="APPROVED",
                manual_override=chance(self.rng, 0.10),
                decision_time_minutes=self.rng.randint(8, 45),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 3: full-amount refund on expensive order -------------- #

    def generate_full_expensive(self, count: int) -> None:
        for _ in range(count):
            category = choose_category(self.rng, ("electronics", "luxury", "appliances", "fashion"))
            agent = self.rng.choice(self.normal_agents + self.fast_agents)
            amt = order_amount(self.rng, category, force_high_value=True)
            self._emit(
                scenario_id=3, label="fraud",
                customer_id=self.rng.choice(self.normal_customers),
                agent=agent, category=category, order_amt=amt,
                refund_amt=amt,  # full refund
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, 0.45), decision="APPROVED",
                manual_override=chance(self.rng, 0.12),
                decision_time_minutes=self.rng.randint(10, 65),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 4: very fast approval --------------------------------- #

    def generate_very_fast(self, count: int) -> None:
        for _ in range(count):
            category = choose_category(self.rng)
            agent = self.rng.choice(self.fast_agents)
            amt = order_amount(self.rng, category)
            ref = refund_amount(self.rng, amt, 0.50, 1.0)
            self._emit(
                scenario_id=4, label="fraud",
                customer_id=self.rng.choice(self.normal_customers),
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, category.evidence_probability),
                decision="APPROVED",
                manual_override=chance(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(1, THRESHOLDS.fast_approval_minutes),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 5: manual override on high-value refund --------------- #

    def generate_manual_override(self, count: int) -> None:
        for _ in range(count):
            category = choose_category(self.rng, ("electronics", "luxury", "appliances"))
            agent = self.rng.choice(self.normal_agents + self.fast_agents)
            amt = order_amount(self.rng, category, force_high_value=True)
            ref = refund_amount(self.rng, amt, 0.65, 1.0)
            self._emit(
                scenario_id=5, label="fraud",
                customer_id=self.rng.choice(self.normal_customers),
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, 0.35), decision="APPROVED",
                manual_override=True,
                decision_time_minutes=self.rng.randint(4, 35),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 6: customer with frequent refund requests ------------- #

    def generate_frequent_customer(self, count: int) -> None:
        # Spread the budget across the frequent-returner pool, in time bursts.
        for i in range(count):
            customer = self.frequent_customers[i % len(self.frequent_customers)]
            category = choose_category(self.rng)
            agent = self.rng.choice(self.normal_agents + self.fast_agents)
            high_value = chance(self.rng, 0.25)
            amt = order_amount(self.rng, category, force_high_value=high_value)
            ref = refund_amount(self.rng, amt, 0.50, 1.0)
            # Lower evidence rate than legitimate customers.
            evidence_p = max(0.05, category.evidence_probability - 0.25)
            self._emit(
                scenario_id=6, label="fraud",
                customer_id=customer,
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, evidence_p), decision="APPROVED",
                manual_override=chance(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(8, 70),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 7: agent with unusually high approval rate ------------ #

    def generate_high_approval_agent(self, count: int) -> None:
        for i in range(count):
            agent = self.high_approval_agents[i % len(self.high_approval_agents)]
            category = choose_category(self.rng)
            amt = order_amount(self.rng, category, force_high_value=chance(self.rng, 0.25))
            ref = refund_amount(self.rng, amt, 0.40, 1.0)
            self._emit(
                scenario_id=7, label="fraud",
                customer_id=self.rng.choice(self.normal_customers),
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, category.evidence_probability * 0.7),
                decision="APPROVED",
                manual_override=chance(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(agent.min_decision_time,
                                                        agent.max_decision_time),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 8: repeated agent-customer approval pattern ----------- #

    def generate_repeated_pair(self, count: int) -> None:
        for i in range(count):
            agent, customer = self.repeated_pairs[i % len(self.repeated_pairs)]
            category = choose_category(self.rng, ("fashion", "electronics", "beauty", "sports"))
            amt = order_amount(self.rng, category, force_high_value=chance(self.rng, 0.25))
            ref = refund_amount(self.rng, amt, 0.60, 1.0)
            self._emit(
                scenario_id=8, label="fraud",
                customer_id=customer,
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=chance(self.rng, category.evidence_probability * 0.6),
                decision="APPROVED",
                manual_override=chance(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(agent.min_decision_time,
                                                        agent.max_decision_time),
                timestamp=self._random_timestamp(),
            )

    # ---- scenario 9: suspicious cluster --------------------------------- #

    def generate_cluster(self, count: int) -> None:
        for i in range(count):
            agent, ring = self.clusters[i % len(self.clusters)]
            customer = self.rng.choice(ring)
            category = choose_category(self.rng, ("electronics", "luxury", "appliances"))
            amt = order_amount(self.rng, category, force_high_value=True)
            # Occasional over-refund (refund > order) as a strong fraud signal.
            if chance(self.rng, 0.15):
                ref = money(amt * self.rng.uniform(1.0, 1.05))
            else:
                ref = refund_amount(self.rng, amt, 0.85, 1.0)
            self._emit(
                scenario_id=9, label="fraud",
                customer_id=customer,
                agent=agent, category=category, order_amt=amt, refund_amt=ref,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=False, decision="APPROVED",
                manual_override=True,
                decision_time_minutes=self.rng.randint(agent.min_decision_time,
                                                        agent.max_decision_time),
                timestamp=self._random_timestamp(),
            )

    # ---- orchestration -------------------------------------------------- #

    def _fraud_counts(self, fraud_total: int) -> Dict[int, int]:
        counts: Dict[int, int] = {}
        assigned = 0
        scenario_ids = sorted(FRAUD_SCENARIO_WEIGHTS)
        for sid in scenario_ids[:-1]:
            n = round(fraud_total * FRAUD_SCENARIO_WEIGHTS[sid])
            counts[sid] = n
            assigned += n
        # Give the remainder to the last scenario so the totals match exactly.
        counts[scenario_ids[-1]] = fraud_total - assigned
        return counts

    def _build_legacy_rows(self) -> List[GeneratedRow]:
        """Parse the embedded legacy fixture into typed rows.

        Rows are 5 per scenario in scenario order, so scenario_id = index // 5 + 1.
        """
        reader = csv.DictReader(io.StringIO(LEGACY_DATASET_CSV.strip()))
        rows: List[GeneratedRow] = []
        for index, raw in enumerate(reader):
            scenario_id = index // 5 + 1
            data = {
                "order_id": raw["order_id"],
                "customer_id": raw["customer_id"],
                "return_id": raw["return_id"],
                "support_agent_id": raw["support_agent_id"],
                "order_amount": money(float(raw["order_amount"])),
                "refund_amount": money(float(raw["refund_amount"])),
                "product_category": raw["product_category"],
                "return_reason": raw["return_reason"],
                "evidence_provided": raw["evidence_provided"].strip().lower() == "true",
                "decision": raw["decision"].strip().upper(),
                "manual_override": raw["manual_override"].strip().lower() == "true",
                "decision_time_minutes": int(raw["decision_time_minutes"]),
                "timestamp": raw["timestamp"],
            }
            label = "normal" if scenario_id == 1 else "fraud"
            rows.append(GeneratedRow(data=data, scenario_id=scenario_id, label=label))
        return rows

    def generate(self) -> None:
        # Legacy fixture rows are always included (backend tests depend on them).
        legacy_rows = self._build_legacy_rows()
        synthetic_total = max(0, self.total_rows - len(legacy_rows))

        fraud_total = round(synthetic_total * self.fraud_ratio)
        normal_total = synthetic_total - fraud_total
        counts = self._fraud_counts(fraud_total)

        self.rows.extend(legacy_rows)
        self.generate_normal(normal_total)
        self.generate_high_value_no_evidence(counts[2])
        self.generate_full_expensive(counts[3])
        self.generate_very_fast(counts[4])
        self.generate_manual_override(counts[5])
        self.generate_frequent_customer(counts[6])
        self.generate_high_approval_agent(counts[7])
        self.generate_repeated_pair(counts[8])
        self.generate_cluster(counts[9])

        # Interleave rows so scenarios are not trivially separable by position,
        # then sort by timestamp to mimic a real chronological export.
        self.rng.shuffle(self.rows)
        self.rows.sort(key=lambda r: r.data["timestamp"])


# --------------------------------------------------------------------------- #
# Output
# --------------------------------------------------------------------------- #

def write_clean(path: Path, rows: List[GeneratedRow]) -> None:
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=CLEAN_COLUMNS)
        writer.writeheader()
        for row in rows:
            writer.writerow(row.data)


def write_labels(path: Path, rows: List[GeneratedRow]) -> None:
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.DictWriter(fh, fieldnames=LABEL_COLUMNS)
        writer.writeheader()
        for row in rows:
            writer.writerow({
                "order_id": row.data["order_id"],
                "return_id": row.data["return_id"],
                "customer_id": row.data["customer_id"],
                "support_agent_id": row.data["support_agent_id"],
                "scenario_id": row.scenario_id,
                "scenario_name": SCENARIOS[row.scenario_id],
                "label": row.label,
                "expected_rules": ";".join(EXPECTED_RULES[row.scenario_id]),
                "decision": row.data["decision"],
            })


def print_summary(rows: List[GeneratedRow]) -> None:
    by_scenario: Dict[int, int] = {}
    by_label: Dict[str, int] = {}
    by_decision: Dict[str, int] = {}
    for row in rows:
        by_scenario[row.scenario_id] = by_scenario.get(row.scenario_id, 0) + 1
        by_label[row.label] = by_label.get(row.label, 0) + 1
        d = str(row.data["decision"])
        by_decision[d] = by_decision.get(d, 0) + 1

    total = len(rows)
    print(f"\nTotal rows: {total}")
    print("\nLabel balance:")
    for label, n in sorted(by_label.items()):
        print(f"  {label:7s}: {n:5d} ({n / total:.0%})")
    print("\nDecision balance:")
    for decision, n in sorted(by_decision.items()):
        print(f"  {decision:9s}: {n:5d} ({n / total:.0%})")
    print("\nScenario coverage:")
    for sid in sorted(by_scenario):
        print(f"  [{sid}] {SCENARIOS[sid][:55]:55s} {by_scenario[sid]:5d}")


# --------------------------------------------------------------------------- #
# CLI
# --------------------------------------------------------------------------- #

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate clean synthetic refund dataset.")
    parser.add_argument("--output-dir", type=str, default="data",
                        help="Directory where generated CSV files are saved.")
    parser.add_argument("--total-rows", type=int, default=5000,
                        help="Approximate total number of generated rows.")
    parser.add_argument("--fraud-ratio", type=float, default=0.40,
                        help="Fraction of rows that are suspicious/fraud (0-1).")
    parser.add_argument("--seed", type=int, default=42,
                        help="Random seed for reproducible generation.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    generator = RefundDatasetGenerator(
        total_rows=args.total_rows,
        fraud_ratio=args.fraud_ratio,
        seed=args.seed,
    )
    generator.generate()

    dataset_path = output_dir / "clean_refund_dataset.csv"
    labels_path = output_dir / "dataset_labels.csv"

    write_clean(dataset_path, generator.rows)
    write_labels(labels_path, generator.rows)

    print(f"Created clean dataset:   {dataset_path}")
    print(f"Created labels file:     {labels_path}")
    print_summary(generator.rows)


if __name__ == "__main__":
    main()
