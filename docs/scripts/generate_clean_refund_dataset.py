"""
Generate a clean synthetic e-commerce refund dataset.

The generator creates realistic refund approval cases for the MVP domain:
suspicious refund approvals in e-commerce customer support.

It produces two files:
1. clean_refund_dataset.csv
   - Main backend-ready dataset.
   - Contains only clean normalized columns.

2. scenario_coverage.csv
   - Documentation/helper file.
   - Shows which generated order belongs to which suspicious scenario.

The dataset is generated using probabilistic behavior profiles for customers,
support agents, product categories, and refund decisions. This makes the rows
more diverse than manually hardcoded examples while still guaranteeing that
each required scenario is represented exactly N times.

Usage:
    python generate_clean_refund_dataset.py

Optional:
    python generate_clean_refund_dataset.py --output-dir data --cases-per-scenario 5 --seed 67
"""

from __future__ import annotations

import argparse
import csv
import random
from dataclasses import dataclass
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Tuple


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

COVERAGE_COLUMNS = [
    "scenario_id",
    "scenario_name",
    "order_id",
    "return_id",
]


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


@dataclass(frozen=True)
class CustomerProfile:
    customer_id: str
    frequent_returner: bool
    evidence_probability_modifier: float
    high_value_probability_modifier: float


PRODUCT_PROFILES: List[ProductCategoryProfile] = [
    ProductCategoryProfile(
        name="electronics",
        min_amount=120,
        max_amount=2200,
        common_reasons=("not_working", "damaged_item", "missing_accessory", "item_not_as_described"),
        evidence_probability=0.55,
    ),
    ProductCategoryProfile(
        name="clothing",
        min_amount=20,
        max_amount=260,
        common_reasons=("wrong_size", "changed_mind", "item_not_as_described"),
        evidence_probability=0.75,
    ),
    ProductCategoryProfile(
        name="home",
        min_amount=35,
        max_amount=600,
        common_reasons=("damaged_item", "missing_part", "item_not_as_described"),
        evidence_probability=0.70,
    ),
    ProductCategoryProfile(
        name="beauty",
        min_amount=15,
        max_amount=180,
        common_reasons=("allergic_reaction", "damaged_item", "changed_mind"),
        evidence_probability=0.65,
    ),
    ProductCategoryProfile(
        name="sports",
        min_amount=25,
        max_amount=450,
        common_reasons=("wrong_size", "damaged_item", "item_not_as_described"),
        evidence_probability=0.72,
    ),
    ProductCategoryProfile(
        name="books",
        min_amount=8,
        max_amount=90,
        common_reasons=("wrong_item_received", "damaged_item", "changed_mind"),
        evidence_probability=0.85,
    ),
    ProductCategoryProfile(
        name="appliances",
        min_amount=180,
        max_amount=1800,
        common_reasons=("not_working", "damaged_item", "missing_part"),
        evidence_probability=0.50,
    ),
    ProductCategoryProfile(
        name="luxury",
        min_amount=350,
        max_amount=3000,
        common_reasons=("item_not_as_described", "changed_mind", "damaged_item"),
        evidence_probability=0.45,
    ),
    ProductCategoryProfile(
        name="fashion",
        min_amount=40,
        max_amount=650,
        common_reasons=("wrong_size", "changed_mind", "item_not_as_described"),
        evidence_probability=0.68,
    ),
]


NORMAL_AGENTS: List[AgentProfile] = [
    AgentProfile("agent_001", 0.78, 0.04, 25, 90),
    AgentProfile("agent_002", 0.73, 0.03, 30, 95),
    AgentProfile("agent_003", 0.70, 0.05, 35, 100),
    AgentProfile("agent_004", 0.76, 0.06, 25, 85),
    AgentProfile("agent_005", 0.72, 0.04, 30, 90),
]

FAST_AGENTS: List[AgentProfile] = [
    AgentProfile("agent_101", 0.90, 0.10, 1, 5),
    AgentProfile("agent_102", 0.88, 0.12, 1, 6),
]

HIGH_APPROVAL_AGENT = AgentProfile("agent_777", 0.98, 0.18, 5, 35)
REPEATED_AGENT = AgentProfile("agent_888", 0.95, 0.12, 4, 28)
CLUSTER_AGENT = AgentProfile("agent_999", 0.99, 0.85, 1, 8)


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


def money(value: float) -> float:
    """Round monetary values to two decimal places."""
    return round(value, 2)


def boolean_event(rng: random.Random, probability: float) -> bool:
    """Return True with the given probability."""
    return rng.random() < probability


def choose_category(rng: random.Random, allowed_names: Tuple[str, ...] | None = None) -> ProductCategoryProfile:
    """Choose a product category profile, optionally from a restricted set."""
    if allowed_names is None:
        return rng.choice(PRODUCT_PROFILES)

    allowed = [profile for profile in PRODUCT_PROFILES if profile.name in allowed_names]
    return rng.choice(allowed)


def generate_order_amount(
    rng: random.Random,
    category: ProductCategoryProfile,
    force_high_value: bool = False,
) -> float:
    """Generate an order amount based on product category and scenario constraints."""
    min_amount = category.min_amount
    max_amount = category.max_amount

    if force_high_value:
        min_amount = max(min_amount, 600)
        max_amount = max(max_amount, min_amount + 300)

    return money(rng.uniform(min_amount, max_amount))


def generate_refund_amount(
    rng: random.Random,
    order_amount: float,
    min_ratio: float,
    max_ratio: float,
    full_refund: bool = False,
) -> float:
    """Generate refund amount as a fraction of the order amount."""
    if full_refund:
        return money(order_amount)

    refund_ratio = rng.uniform(min_ratio, max_ratio)
    return money(order_amount * refund_ratio)


def generate_timestamp(base_time: datetime, day_offset: int, minute_offset: int) -> str:
    """Generate ISO-like UTC timestamp."""
    timestamp = base_time + timedelta(days=day_offset, minutes=minute_offset)
    return timestamp.strftime("%Y-%m-%dT%H:%M:%SZ")


def create_row(
    order_id: str,
    return_id: str,
    customer_id: str,
    support_agent_id: str,
    order_amount: float,
    refund_amount: float,
    product_category: str,
    return_reason: str,
    evidence_provided: bool,
    decision: str,
    manual_override: bool,
    decision_time_minutes: int,
    timestamp: str,
) -> Dict[str, object]:
    """Create one clean refund approval row."""
    return {
        "order_id": order_id,
        "customer_id": customer_id,
        "return_id": return_id,
        "support_agent_id": support_agent_id,
        "order_amount": money(order_amount),
        "refund_amount": money(refund_amount),
        "product_category": product_category,
        "return_reason": return_reason,
        "evidence_provided": evidence_provided,
        "decision": decision,
        "manual_override": manual_override,
        "decision_time_minutes": decision_time_minutes,
        "timestamp": timestamp,
    }


class RefundDatasetGenerator:
    """Synthetic refund dataset generator with scenario guarantees."""

    def __init__(self, seed: int = 42) -> None:
        self.rng = random.Random(seed)
        self.base_time = datetime(2026, 6, 1, 9, 0, 0)
        self.order_counter = 1001
        self.return_counter = 3001
        self.rows: List[Dict[str, object]] = []
        self.coverage_rows: List[Dict[str, object]] = []

    def next_order_id(self) -> str:
        order_id = f"order_{self.order_counter}"
        self.order_counter += 1
        return order_id

    def next_return_id(self) -> str:
        return_id = f"return_{self.return_counter}"
        self.return_counter += 1
        return return_id

    def append_case(
        self,
        scenario_id: int,
        customer_id: str,
        agent: AgentProfile,
        category: ProductCategoryProfile,
        order_amount: float,
        refund_amount: float,
        return_reason: str,
        evidence_provided: bool,
        decision: str,
        manual_override: bool,
        decision_time_minutes: int,
        day_offset: int,
        minute_offset: int,
    ) -> None:
        order_id = self.next_order_id()
        return_id = self.next_return_id()

        row = create_row(
            order_id=order_id,
            return_id=return_id,
            customer_id=customer_id,
            support_agent_id=agent.agent_id,
            order_amount=order_amount,
            refund_amount=refund_amount,
            product_category=category.name,
            return_reason=return_reason,
            evidence_provided=evidence_provided,
            decision=decision,
            manual_override=manual_override,
            decision_time_minutes=decision_time_minutes,
            timestamp=generate_timestamp(self.base_time, day_offset, minute_offset),
        )

        self.rows.append(row)
        self.coverage_rows.append(
            {
                "scenario_id": scenario_id,
                "scenario_name": SCENARIOS[scenario_id],
                "order_id": order_id,
                "return_id": return_id,
            }
        )

    def generate_normal_refunds(self, cases: int) -> None:
        """Scenario 1: normal approvals with evidence and reasonable timing."""
        for i in range(cases):
            category = choose_category(self.rng, ("clothing", "home", "books", "beauty", "sports"))
            agent = self.rng.choice(NORMAL_AGENTS)
            order_amount = generate_order_amount(self.rng, category, force_high_value=False)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.25, 1.0)

            self.append_case(
                scenario_id=1,
                customer_id=f"customer_{200 + i}",
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=True,
                decision="APPROVED",
                manual_override=False,
                decision_time_minutes=self.rng.randint(30, 95),
                day_offset=0,
                minute_offset=20 * i + self.rng.randint(0, 8),
            )

    def generate_high_value_no_evidence(self, cases: int) -> None:
        """Scenario 2: high-value refund approved without evidence."""
        for i in range(cases):
            category = choose_category(self.rng, ("electronics", "appliances", "luxury"))
            agent = self.rng.choice(NORMAL_AGENTS + FAST_AGENTS)
            order_amount = generate_order_amount(self.rng, category, force_high_value=True)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.45, 0.9)

            self.append_case(
                scenario_id=2,
                customer_id=f"customer_{300 + i}",
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=False,
                decision="APPROVED",
                manual_override=False,
                decision_time_minutes=self.rng.randint(8, 40),
                day_offset=1,
                minute_offset=24 * i + self.rng.randint(0, 10),
            )

    def generate_full_expensive_refunds(self, cases: int) -> None:
        """Scenario 3: full-amount refund approved for expensive order."""
        for i in range(cases):
            category = choose_category(self.rng, ("electronics", "luxury", "appliances", "fashion"))
            agent = self.rng.choice(NORMAL_AGENTS + FAST_AGENTS)
            order_amount = generate_order_amount(self.rng, category, force_high_value=True)

            self.append_case(
                scenario_id=3,
                customer_id=f"customer_{400 + i}",
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=order_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, 0.45),
                decision="APPROVED",
                manual_override=boolean_event(self.rng, 0.10),
                decision_time_minutes=self.rng.randint(10, 65),
                day_offset=2,
                minute_offset=28 * i + self.rng.randint(0, 12),
            )

    def generate_very_fast_approvals(self, cases: int) -> None:
        """Scenario 4: very fast approval by support agent."""
        for i in range(cases):
            category = choose_category(self.rng)
            agent = self.rng.choice(FAST_AGENTS)
            order_amount = generate_order_amount(self.rng, category, force_high_value=False)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.50, 1.0)

            self.append_case(
                scenario_id=4,
                customer_id=f"customer_{500 + i}",
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, category.evidence_probability),
                decision="APPROVED",
                manual_override=boolean_event(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(agent.min_decision_time, agent.max_decision_time),
                day_offset=3,
                minute_offset=15 * i + self.rng.randint(0, 5),
            )

    def generate_manual_override_high_value(self, cases: int) -> None:
        """Scenario 5: manual override on high-value refund."""
        for i in range(cases):
            category = choose_category(self.rng, ("electronics", "luxury", "appliances"))
            agent = self.rng.choice(NORMAL_AGENTS + FAST_AGENTS)
            order_amount = generate_order_amount(self.rng, category, force_high_value=True)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.65, 1.0)

            self.append_case(
                scenario_id=5,
                customer_id=f"customer_{600 + i}",
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, 0.35),
                decision="APPROVED",
                manual_override=True,
                decision_time_minutes=self.rng.randint(4, 35),
                day_offset=4,
                minute_offset=22 * i + self.rng.randint(0, 8),
            )

    def generate_frequent_customer_returns(self, cases: int) -> None:
        """Scenario 6: same customer repeatedly requests refunds."""
        frequent_customer = CustomerProfile(
            customer_id="customer_900",
            frequent_returner=True,
            evidence_probability_modifier=-0.20,
            high_value_probability_modifier=0.10,
        )

        for i in range(cases):
            category = choose_category(self.rng)
            agent = self.rng.choice(NORMAL_AGENTS + FAST_AGENTS)
            force_high = boolean_event(self.rng, 0.20 + frequent_customer.high_value_probability_modifier)
            order_amount = generate_order_amount(self.rng, category, force_high_value=force_high)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.50, 1.0)
            evidence_probability = max(0.05, category.evidence_probability + frequent_customer.evidence_probability_modifier)

            self.append_case(
                scenario_id=6,
                customer_id=frequent_customer.customer_id,
                agent=agent,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, evidence_probability),
                decision="APPROVED",
                manual_override=boolean_event(self.rng, agent.manual_override_probability),
                decision_time_minutes=self.rng.randint(10, 70),
                day_offset=5 + i,
                minute_offset=12 * i + self.rng.randint(0, 8),
            )

    def generate_high_approval_agent_cases(self, cases: int) -> None:
        """Scenario 7: one support agent approves unusually many refund requests."""
        for i in range(cases):
            category = choose_category(self.rng)
            order_amount = generate_order_amount(self.rng, category, force_high_value=boolean_event(self.rng, 0.25))
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.40, 1.0)

            self.append_case(
                scenario_id=7,
                customer_id=f"customer_{700 + i}",
                agent=HIGH_APPROVAL_AGENT,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, category.evidence_probability * 0.75),
                decision="APPROVED",
                manual_override=boolean_event(self.rng, HIGH_APPROVAL_AGENT.manual_override_probability),
                decision_time_minutes=self.rng.randint(
                    HIGH_APPROVAL_AGENT.min_decision_time,
                    HIGH_APPROVAL_AGENT.max_decision_time,
                ),
                day_offset=10,
                minute_offset=18 * i + self.rng.randint(0, 8),
            )

    def generate_repeated_agent_customer_pattern(self, cases: int) -> None:
        """Scenario 8: same agent repeatedly approves refunds for the same customer."""
        repeated_customer = "customer_888"

        for i in range(cases):
            category = choose_category(self.rng, ("fashion", "electronics", "beauty", "sports"))
            order_amount = generate_order_amount(self.rng, category, force_high_value=boolean_event(self.rng, 0.20))
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.60, 1.0)

            self.append_case(
                scenario_id=8,
                customer_id=repeated_customer,
                agent=REPEATED_AGENT,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=boolean_event(self.rng, category.evidence_probability * 0.60),
                decision="APPROVED",
                manual_override=boolean_event(self.rng, REPEATED_AGENT.manual_override_probability),
                decision_time_minutes=self.rng.randint(
                    REPEATED_AGENT.min_decision_time,
                    REPEATED_AGENT.max_decision_time,
                ),
                day_offset=11 + i,
                minute_offset=10 * i + self.rng.randint(0, 7),
            )

    def generate_suspicious_cluster(self, cases: int) -> None:
        """Scenario 9: same customer and agent, high-value refunds, no evidence, manual overrides."""
        cluster_customer = "customer_999"

        for i in range(cases):
            category = choose_category(self.rng, ("electronics", "luxury", "appliances"))
            order_amount = generate_order_amount(self.rng, category, force_high_value=True)
            refund_amount = generate_refund_amount(self.rng, order_amount, 0.80, 1.0)

            self.append_case(
                scenario_id=9,
                customer_id=cluster_customer,
                agent=CLUSTER_AGENT,
                category=category,
                order_amount=order_amount,
                refund_amount=refund_amount,
                return_reason=self.rng.choice(category.common_reasons),
                evidence_provided=False,
                decision="APPROVED",
                manual_override=True,
                decision_time_minutes=self.rng.randint(
                    CLUSTER_AGENT.min_decision_time,
                    CLUSTER_AGENT.max_decision_time,
                ),
                day_offset=16 + i,
                minute_offset=8 * i + self.rng.randint(0, 5),
            )

    def generate(self, cases_per_scenario: int) -> Tuple[List[Dict[str, object]], List[Dict[str, object]]]:
        """Generate all scenarios."""
        self.generate_normal_refunds(cases_per_scenario)
        self.generate_high_value_no_evidence(cases_per_scenario)
        self.generate_full_expensive_refunds(cases_per_scenario)
        self.generate_very_fast_approvals(cases_per_scenario)
        self.generate_manual_override_high_value(cases_per_scenario)
        self.generate_frequent_customer_returns(cases_per_scenario)
        self.generate_high_approval_agent_cases(cases_per_scenario)
        self.generate_repeated_agent_customer_pattern(cases_per_scenario)
        self.generate_suspicious_cluster(cases_per_scenario)

        return self.rows, self.coverage_rows


def write_csv(path: Path, rows: List[Dict[str, object]], columns: List[str]) -> None:
    """Write rows to CSV."""
    with path.open("w", newline="", encoding="utf-8") as file:
        writer = csv.DictWriter(file, fieldnames=columns)
        writer.writeheader()
        writer.writerows(rows)


def print_summary(coverage_rows: List[Dict[str, object]]) -> None:
    """Print scenario coverage summary."""
    counts: Dict[str, int] = {}

    for row in coverage_rows:
        scenario_name = str(row["scenario_name"])
        counts[scenario_name] = counts.get(scenario_name, 0) + 1

    print("\nScenario coverage:")
    for scenario_name, count in counts.items():
        print(f"- {scenario_name}: {count} cases")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate clean synthetic e-commerce refund dataset."
    )

    parser.add_argument(
        "--output-dir",
        type=str,
        default="data",
        help="Directory where generated CSV files will be saved.",
    )

    parser.add_argument(
        "--cases-per-scenario",
        type=int,
        default=5,
        help="Number of cases generated for each suspicious scenario.",
    )

    parser.add_argument(
        "--seed",
        type=int,
        default=42,
        help="Random seed for reproducible generation.",
    )

    return parser.parse_args()


def main() -> None:
    args = parse_args()

    if args.cases_per_scenario <= 0:
        raise ValueError("cases_per_scenario must be a positive integer.")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    dataset_path = output_dir / "clean_refund_dataset.csv"
    coverage_path = output_dir / "scenario_coverage.csv"

    generator = RefundDatasetGenerator(seed=args.seed)
    rows, coverage_rows = generator.generate(cases_per_scenario=args.cases_per_scenario)

    write_csv(dataset_path, rows, CLEAN_COLUMNS)
    write_csv(coverage_path, coverage_rows, COVERAGE_COLUMNS)

    print(f"Created clean dataset: {dataset_path}")
    print(f"Created scenario coverage file: {coverage_path}")
    print(f"Total rows: {len(rows)}")
    print_summary(coverage_rows)


if __name__ == "__main__":
    main()
