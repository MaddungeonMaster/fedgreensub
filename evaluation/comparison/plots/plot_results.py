#!/usr/bin/env python3
"""Generate isolated IEEE-style figures from evaluation/comparison results.

This module reads raw per-run JSON files first. It never invents unavailable
measurements; metrics marked unavailable by the evaluation harness are omitted.
"""

from __future__ import annotations

import argparse
import csv
import json
import math
import statistics
from collections import defaultdict
from pathlib import Path
from typing import Any, Callable, Iterable

import matplotlib.pyplot as plt
import numpy as np

IMPLEMENTATIONS = {
    "gossipsub": "Standard GossipSub",
    "fedgreen": "FedGreenSub",
    "trustaware": "Trust-Aware FedGreenSub",
}
ORDER = list(IMPLEMENTATIONS)
SCENARIOS = {"normal": "Normal", "duplicate": "Duplicate Traffic", "packetloss": "Packet Loss", "scaling": "Scaling", "trust": "Trust"}
COLORS = {"gossipsub": "#4C78A8", "fedgreen": "#F58518", "trustaware": "#54A24B"}


def finite(value: Any) -> float | None:
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    return number if math.isfinite(number) else None


def metric(record: dict[str, Any], name: str) -> float | None:
    metrics = record.get("metrics", {})
    if name in metrics:
        return finite(metrics[name])
    wanted = name.lower()
    for key, value in metrics.items():
        if key.lower() == wanted:
            return finite(value)
    return None


def available(record: dict[str, Any], name: str) -> bool:
    availability = record.get("metrics", {}).get("Available", {})
    if name in availability:
        return bool(availability[name])
    return True


def load_json_records(results_dir: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for path in sorted(results_dir.glob("**/*.json")):
        if path.name == "statistics.json":
            continue
        try:
            with path.open(encoding="utf-8") as handle:
                record = json.load(handle)
            if isinstance(record, dict) and record.get("implementation") in IMPLEMENTATIONS:
                records.append(record)
        except (OSError, json.JSONDecodeError):
            continue
    return records


def load_csv_records(results_dir: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for path in sorted(results_dir.glob("**/*.csv")):
        try:
            with path.open(newline="", encoding="utf-8") as handle:
                for row in csv.DictReader(handle):
                    implementation = row.get("implementation", "")
                    if implementation not in IMPLEMENTATIONS:
                        continue
                    metrics = {key: finite(value) for key, value in row.items() if key not in {"scenario", "implementation", "peers", "repetition"}}
                    records.append({"scenario": row.get("scenario", path.parent.name), "implementation": implementation, "peer_count": int(row.get("peers", 0)), "repetition": int(row.get("repetition", 0)), "metrics": metrics})
        except (OSError, ValueError):
            continue
    return records


def load_records(results_dir: Path) -> list[dict[str, Any]]:
    records = load_json_records(results_dir)
    return records if records else load_csv_records(results_dir)


def peer_count_for(records: list[dict[str, Any]], scenario: str) -> int | None:
    counts: defaultdict[int, set[str]] = defaultdict(set)
    for record in records:
        if record.get("scenario") == scenario:
            counts[int(record.get("peer_count", 0))].add(record.get("implementation", ""))
    complete = [count for count, implementations in counts.items() if set(ORDER).issubset(implementations)]
    return max(complete) if complete else None


def selected(records: list[dict[str, Any]], scenario: str) -> list[dict[str, Any]]:
    count = peer_count_for(records, scenario)
    return [record for record in records if record.get("scenario") == scenario and (count is None or int(record.get("peer_count", 0)) == count)]


def grouped(records: Iterable[dict[str, Any]], field: str) -> dict[str, list[float]]:
    values: defaultdict[str, list[float]] = defaultdict(list)
    for record in records:
        value = metric(record, field)
        if value is not None:
            values[record["implementation"]].append(value)
    return values


def mean_std(values: list[float]) -> tuple[float, float]:
    if not values:
        return math.nan, math.nan
    return statistics.mean(values), statistics.stdev(values) if len(values) > 1 else 0.0


def save_figure(fig: plt.Figure, output_dir: Path, stem: str) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_dir / f"{stem}.png", dpi=300, bbox_inches="tight")
    fig.savefig(output_dir / f"{stem}.pdf", bbox_inches="tight")
    plt.close(fig)


def style_axis(ax: plt.Axes) -> None:
    ax.grid(axis="y", color="#D9D9D9", linewidth=0.6, alpha=0.7)
    ax.set_axisbelow(True)
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)


def grouped_bar(records: list[dict[str, Any]], field: str, ylabel: str, stem: str, output_dir: Path, reductions: bool = False) -> None:
    values = grouped(records, field)
    labels = [key for key in ORDER if key in values]
    if not labels:
        return
    means = [mean_std(values[key])[0] for key in labels]
    errors = [mean_std(values[key])[1] for key in labels]
    fig, ax = plt.subplots(figsize=(6.2, 4.0))
    x = np.arange(len(labels))
    bars = ax.bar(x, means, yerr=errors, capsize=4, color=[COLORS[key] for key in labels], edgecolor="black", linewidth=0.5)
    ax.set_xticks(x, [IMPLEMENTATIONS[key] for key in labels], rotation=12, ha="right")
    ax.set_ylabel(ylabel)
    style_axis(ax)
    for bar, value in zip(bars, means):
        ax.annotate(f"{value * 100:.2f}%" if "Ratio" in ylabel else f"{value:.4g}", (bar.get_x() + bar.get_width() / 2, bar.get_height()), xytext=(0, 4), textcoords="offset points", ha="center", fontsize=8)
    if reductions and "gossipsub" in values:
        baseline = mean_std(values["gossipsub"])[0]
        for key in labels:
            if key != "gossipsub" and baseline:
                reduction = (baseline - mean_std(values[key])[0]) / baseline * 100
                ax.annotate(f"{reduction:.2f}% vs baseline", (x[labels.index(key)], means[labels.index(key)]), xytext=(0, -24), textcoords="offset points", ha="center", fontsize=7, color="#444444")
    fig.tight_layout()
    save_figure(fig, output_dir, stem)


def line_plot(records: list[dict[str, Any]], x_field: str, y_field: str, xlabel: str, ylabel: str, stem: str, output_dir: Path, multiplier: float = 1.0) -> None:
    series: defaultdict[str, defaultdict[float, list[float]]] = defaultdict(lambda: defaultdict(list))
    for record in records:
        x = metric(record, x_field) if x_field == "PacketLoss" else finite(record.get("peer_count")) if x_field == "PeerCount" else finite(record.get(x_field))
        y = metric(record, y_field)
        if x is not None and y is not None:
            series[record["implementation"]][x].append(y * multiplier)
    if not series:
        return
    fig, ax = plt.subplots(figsize=(6.2, 4.0))
    for key in ORDER:
        if key not in series:
            continue
        points = sorted(series[key].items())
        xs = [point[0] * (100 if x_field == "PacketLoss" else 1) for point in points]
        ys = [mean_std(point[1])[0] for point in points]
        errors = [mean_std(point[1])[1] for point in points]
        ax.plot(xs, ys, marker="o", linewidth=1.8, label=IMPLEMENTATIONS[key], color=COLORS[key])
        if len(points) > 1:
            ax.fill_between(xs, np.array(ys) - errors, np.array(ys) + errors, color=COLORS[key], alpha=0.12)
    ax.set_xlabel(xlabel)
    ax.set_ylabel(ylabel)
    ax.legend(frameon=False)
    style_axis(ax)
    fig.tight_layout()
    save_figure(fig, output_dir, stem)


def energy_reduction(records: list[dict[str, Any]], output_dir: Path) -> None:
    scenarios = ["normal", "packetloss", "duplicate", "scaling"]
    labels, means, errors = [], [], []
    for scenario in scenarios:
        current = selected(records, scenario)
        values = grouped(current, "EnergyPerDeliveredMessage")
        if not all(key in values for key in ("gossipsub", "trustaware")):
            continue
        baseline, _ = mean_std(values["gossipsub"])
        trust, trust_error = mean_std(values["trustaware"])
        if baseline == 0:
            continue
        labels.append(SCENARIOS[scenario]); means.append((baseline - trust) / baseline * 100); errors.append(trust_error / baseline * 100)
    if not means:
        return
    fig, ax = plt.subplots(figsize=(6.2, 4.0))
    bars = ax.bar(labels, means, yerr=errors, capsize=4, color=COLORS["trustaware"], edgecolor="black", linewidth=0.5)
    ax.set_ylabel("Estimated Energy Reduction (%)")
    ax.set_title("Energy values represent modeled estimates.", fontsize=9, loc="left")
    for bar, value in zip(bars, means):
        ax.annotate(f"{value:.2f}%", (bar.get_x() + bar.get_width() / 2, bar.get_height()), xytext=(0, 4), textcoords="offset points", ha="center", fontsize=8)
    style_axis(ax); fig.tight_layout(); save_figure(fig, output_dir, "06_energy_reduction")


def overhead_plot(records: list[dict[str, Any]], output_dir: Path) -> bool:
    current = selected(records, "trust") or selected(records, "normal")
    metrics = [("CPU", "CPU utilization", False), ("TrustUpdateTime", "Trust update time (us)", True), ("PeerRankingTime", "Peer ranking time (us)", True), ("TrustAggregationTime", "Aggregation time (us)", True)]
    available_metrics = []
    for field, label, trust_only in metrics:
        values = grouped(current, field)
        if trust_only:
            if "trustaware" in values and any(value > 0 for value in values["trustaware"]):
                available_metrics.append((field, label, values))
        elif all(key in values for key in ("fedgreen", "trustaware")) and any(value > 0 for value in values["fedgreen"] + values["trustaware"]):
            available_metrics.append((field, label, values))
    if not available_metrics:
        return False
    fig, ax = plt.subplots(figsize=(7.0, 4.2)); x = np.arange(len(available_metrics)); width = .36
    for offset, key in [(-width / 2, "fedgreen"), (width / 2, "trustaware")]:
        values = [mean_std(item[2].get(key, []))[0] if item[2].get(key) else math.nan for item in available_metrics]
        ax.bar(x + offset, values, width, label=IMPLEMENTATIONS[key], color=COLORS[key])
    ax.set_xticks(x, [item[1] for item in available_metrics], rotation=15, ha="right"); ax.set_ylabel("Measured value"); ax.legend(frameon=False); style_axis(ax); fig.tight_layout(); save_figure(fig, output_dir, "07_trust_overhead"); return True


def tradeoff_plot(records: list[dict[str, Any]], output_dir: Path) -> None:
    values = grouped(records, "EnergyPerDeliveredMessage")
    delivery = grouped(records, "DeliveryRatio")
    fig, ax = plt.subplots(figsize=(6.2, 4.0))
    for key in ORDER:
        if key not in values or key not in delivery:
            continue
        x, _ = mean_std(values[key]); y, _ = mean_std(delivery[key]); ax.scatter([x], [y], s=60, color=COLORS[key], label=IMPLEMENTATIONS[key]); ax.annotate(IMPLEMENTATIONS[key], (x, y), xytext=(5, 5), textcoords="offset points", fontsize=8)
    ax.set_xlabel("Estimated Energy per Delivered Message"); ax.set_ylabel("Delivery Ratio"); style_axis(ax); fig.tight_layout(); save_figure(fig, output_dir, "08_energy_delivery_tradeoff")


def write_summary(records: list[dict[str, Any]], results_dir: Path) -> None:
    rows: list[dict[str, Any]] = []
    fields = {"DuplicateRatio": "Duplicate Ratio", "DeliveryRatio": "Delivery Ratio", "EnergyPerDeliveredMessage": "Estimated Energy per Delivered Message", "MemoryMB": "Go Heap Memory (MB)", "LatencyAverage": "Average Latency (ms)", "AverageTrust": "Average TrustScore", "MinimumTrust": "Minimum TrustScore", "TrustVariance": "Trust Variance", "TrustedPeerRatio": "Trusted Peer Ratio", "TrustUpdateTime": "Trust Update Time (us)", "PeerRankingTime": "Peer Ranking Time (us)", "TrustAggregationTime": "Trust Aggregation Time (us)"}
    for scenario in sorted({record.get("scenario") for record in records}):
        current = selected(records, scenario)
        for field in fields:
            values = grouped(current, field)
            baseline = mean_std(values["gossipsub"])[0] if "gossipsub" in values else math.nan
            for key in ORDER:
                if key not in values:
                    continue
                mean, stddev = mean_std(values[key]); difference = (mean - baseline) / baseline * 100 if baseline and math.isfinite(baseline) else None
                rows.append({"scenario": scenario, "metric": fields[field], "implementation": IMPLEMENTATIONS[key], "mean": mean, "stddev": stddev, "baseline_value": baseline, "percentage_difference": difference})
    with (results_dir / "plot_summary.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=["scenario", "metric", "implementation", "mean", "stddev", "baseline_value", "percentage_difference"]); writer.writeheader(); writer.writerows(rows)
    with (results_dir / "plot_summary.txt").open("w", encoding="utf-8") as handle:
        handle.write("Plot summary generated from raw evaluation results. Energy values represent modeled estimates.\n\n")
        for row in rows:
            difference = f"{row['percentage_difference']:.3f}%" if row["percentage_difference"] is not None else "n/a"
            handle.write(f"{row['scenario']}: {row['metric']} - {row['implementation']}: mean={row['mean']:.6g}, stddev={row['stddev']:.6g}, difference_vs_baseline={difference}\n")


def generate(results_dir: Path, output_dir: Path) -> list[str]:
    records = load_records(results_dir)
    if not records:
        raise RuntimeError(f"No result JSON/CSV records found under {results_dir}")
    plt.rcParams.update({"font.size": 9, "axes.labelsize": 10, "axes.titlesize": 10, "legend.fontsize": 8, "pdf.fonttype": 42, "ps.fonttype": 42})
    generated: list[str] = []
    normal = selected(records, "normal")
    if normal:
        grouped_bar(normal, "DuplicateRatio", "Duplicate Ratio (%)", "01_normal_duplicate_ratio", output_dir, reductions=True); generated.append("01_normal_duplicate_ratio")
    duplicate = selected(records, "duplicate")
    if duplicate:
        grouped_bar(duplicate, "DuplicateRatio", "Duplicate Ratio (%)", "02_duplicate_traffic", output_dir, reductions=True); generated.append("02_duplicate_traffic")
    packetloss = [record for record in selected(records, "packetloss") if metric(record, "PacketLoss") is not None]
    if packetloss:
        line_plot(packetloss, "PacketLoss", "DeliveryRatio", "Packet Loss (%)", "Delivery Ratio", "03_delivery_vs_packet_loss", output_dir); generated.append("03_delivery_vs_packet_loss")
    scaling = selected(records, "scaling")
    if scaling:
        line_plot(scaling, "PeerCount", "DuplicateRatio", "Number of Peers", "Duplicate Ratio (%)", "04_duplicate_vs_peers", output_dir, multiplier=100); generated.append("04_duplicate_vs_peers")
        line_plot(scaling, "PeerCount", "EnergyPerDeliveredMessage", "Number of Peers", "Estimated Energy per Delivered Message", "05_energy_vs_peers", output_dir); generated.append("05_energy_vs_peers")
    before = set(output_dir.glob("*.pdf")); energy_reduction(records, output_dir); generated.extend([path.stem for path in output_dir.glob("*.pdf") if path not in before and path.stem not in generated])
    if overhead_plot(records, output_dir): generated.append("07_trust_overhead")
    if normal:
        tradeoff_plot(normal, output_dir); generated.append("08_energy_delivery_tradeoff")
    write_summary(records, results_dir)
    return sorted(set(generated))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--results", type=Path, default=Path(__file__).resolve().parents[1] / "results")
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parent / "output")
    args = parser.parse_args()
    generated = generate(args.results, args.output)
    print("Generated figures:")
    for name in generated:
        print(f"- {name}.png")
        print(f"- {name}.pdf")
    print(f"Summary: {args.results / 'plot_summary.csv'}")
    print(f"Summary: {args.results / 'plot_summary.txt'}")


if __name__ == "__main__":
    main()
