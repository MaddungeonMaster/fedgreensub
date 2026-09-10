#!/usr/bin/env python3
"""Plot the controlled multi-peer modeled-resource-cost experiment."""

from __future__ import annotations

import argparse
import csv
from collections import defaultdict
from pathlib import Path

import matplotlib.pyplot as plt

IMPLEMENTATIONS = {
    "gossipsub": ("Standard GossipSub", "#4C78A8"),
    "fedgreen": ("FedGreenSub", "#F58518"),
    "trustaware": ("Trust-Aware FedGreenSub", "#54A24B"),
}


def load(path: Path) -> list[dict[str, str]]:
    with path.open(newline="", encoding="utf-8") as handle:
        return list(csv.DictReader(handle))


def values(rows: list[dict[str, str]], implementation: str, field: str) -> tuple[list[int], list[float], list[float]]:
    selected = [row for row in rows if row["implementation"] == implementation]
    selected.sort(key=lambda row: int(row["peer_count"]))
    return ([int(row["peer_count"]) for row in selected], [float(row[field]) for row in selected], [float(row[field.replace("_mean", "_stddev")]) for row in selected])


def plot(rows: list[dict[str, str]], field: str, ylabel: str, stem: str, output: Path, percent: bool = False) -> None:
    fig, ax = plt.subplots(figsize=(6.5, 4.2))
    for implementation, (label, color) in IMPLEMENTATIONS.items():
        xs, ys, errors = values(rows, implementation, field)
        if not xs:
            continue
        if percent:
            ys = [float(rows_for_peer(rows, x, implementation, "reduction_from_baseline_percent")) for x in xs]
            errors = [0.0 for _ in xs]
        ax.errorbar(xs, ys, yerr=errors, marker="o", linewidth=1.8, capsize=3, label=label, color=color)
    ax.set_xlabel("Number of peers")
    ax.set_ylabel(ylabel)
    ax.grid(axis="y", color="#D9D9D9", linewidth=0.6)
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    ax.legend(frameon=False)
    fig.tight_layout()
    output.mkdir(parents=True, exist_ok=True)
    fig.savefig(output / f"{stem}.png", dpi=300, bbox_inches="tight")
    fig.savefig(output / f"{stem}.pdf", bbox_inches="tight")
    plt.close(fig)


def rows_for_peer(rows: list[dict[str, str]], peer_count: int, implementation: str, field: str) -> str:
    for row in rows:
        if int(row["peer_count"]) == peer_count and row["implementation"] == implementation:
            return row[field]
    return "0"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--summary", type=Path, default=Path(__file__).resolve().parents[1] / "results" / "energy_scaling" / "energy_scaling_summary.csv")
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parent / "output" / "energy_scaling")
    args = parser.parse_args()
    rows = load(args.summary)
    plot(rows, "modeled_resource_cost_total_mean", "Modeled resource-cost units per workload", "energy_cost_vs_peers", args.output)
    plot(rows, "modeled_resource_cost_per_delivered_mean", "Modeled cost per delivered message", "energy_cost_per_delivered_vs_peers", args.output)
    plot(rows, "reduction_from_baseline_percent", "Reduction vs Standard GossipSub (%)", "energy_reduction_vs_peers", args.output, percent=True)
    print(f"Generated energy scaling plots under {args.output}")


if __name__ == "__main__":
    main()
