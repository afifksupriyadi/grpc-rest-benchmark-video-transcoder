#!/usr/bin/env python3
"""Query Prometheus for one Scenario A payload point and write the raw
long-format CSV plus the four per-payload wide-format result tables
(latency, throughput, CPU, memory), CSV and Markdown.

Uses only the standard library so it runs with a plain `python3`, no pip
install required.
"""
import argparse
import csv
import json
import math
import time
import urllib.parse
import urllib.request
from pathlib import Path

# (prometheus metric suffix, human unit, conversion factor from raw to unit)
METRICS = [
    ("latency_seconds", "ms", 1000.0),
    ("throughput_bytes_per_second", "MB/s", 1 / 1_048_576),
    ("cpu_usage_ratio", "ratio/core", 1.0),
    ("memory_usage_bytes", "MB", 1 / 1_048_576),
]
METRIC_SHORT_NAME = {
    "latency_seconds": "latency",
    "throughput_bytes_per_second": "throughput",
    "cpu_usage_ratio": "cpu",
    "memory_usage_bytes": "memory",
}
SERVICES = ["gateway", "worker"]
PERCENTILES = [50, 95, 99]
# Canonical segment order used in every wide table, regardless of which
# segments actually have data for a given metric (missing cells render "—").
CANONICAL_SEGMENTS = [
    ("client_to_gateway", "Client ke Gateway"),
    ("gateway_to_worker", "Gateway ke Worker"),
    ("worker_to_gateway", "Worker ke Gateway"),
    ("gateway_to_client", "Gateway ke Client"),
]
PROTOCOLS = ["rest", "grpc"]
DASH = "—"


def _retry_write(path, write_fn, attempts=8, delay=3):
    """Runs write_fn() (the actual file write), retrying on PermissionError.
    On Windows-mounted drives, a CSV freshly opened in Excel (or mid-sync in
    OneDrive) holds an exclusive lock that blocks writes/renames/deletes from
    other processes — this survives that instead of crashing a 45-60 minute
    collection run over one locked file."""
    for attempt in range(1, attempts + 1):
        try:
            write_fn()
            return
        except PermissionError:
            if attempt == attempts:
                raise PermissionError(
                    f"Could not write {path} after {attempts} attempts ({attempts * delay}s). "
                    "It's likely locked by another program (e.g. open in Excel, or mid-sync in "
                    "OneDrive/Google Drive). Close it and re-run."
                )
            time.sleep(delay)


def query_prometheus(base_url, promql):
    url = f"{base_url}/api/v1/query?" + urllib.parse.urlencode({"query": promql})
    with urllib.request.urlopen(url, timeout=30) as resp:
        payload = json.load(resp)
    if payload["status"] != "success":
        raise RuntimeError(f"query failed: {promql!r} -> {payload}")
    return payload["data"]["result"]


def collect_raw(base_url, scenario, payload_size, window):
    """Returns dict[(metric, segment, protocol)][percentile] = converted value."""
    raw = {}
    for metric, _unit, factor in METRICS:
        for service in SERVICES:
            bucket = f"{service}_segment_{metric}_bucket"
            for pct in PERCENTILES:
                promql = (
                    f'histogram_quantile(0.{pct}, rate({bucket}'
                    f'{{scenario="{scenario}",payload_size="{payload_size}"}}[{window}]))'
                )
                for series in query_prometheus(base_url, promql):
                    labels = series["metric"]
                    segment = labels.get("segment")
                    protocol = labels.get("protocol")
                    if segment is None or protocol is None:
                        continue
                    value = float(series["value"][1])
                    if value != value:  # NaN, no data in this window
                        continue
                    key = (metric, segment, protocol)
                    raw.setdefault(key, {})[pct] = round(value * factor, 2)
    return raw


def write_raw_csv(path, payload_size, raw):
    path.parent.mkdir(parents=True, exist_ok=True)
    is_new = not path.exists()

    def _do_write():
        with path.open("a", newline="") as f:
            writer = csv.writer(f)
            if is_new:
                writer.writerow(
                    ["payload_size", "metric", "segment", "protocol", "percentile", "value", "unit"]
                )
            unit_by_metric = {m: u for m, u, _ in METRICS}
            for (metric, segment, protocol), by_pct in sorted(raw.items()):
                for pct, value in sorted(by_pct.items()):
                    writer.writerow(
                        [payload_size, metric, segment, protocol, pct, value, unit_by_metric[metric]]
                    )

    _retry_write(path, _do_write)


def build_wide_rows(raw, metric):
    """Returns list of rows: [segment_label, rest50, rest95, rest99, grpc50, grpc95, grpc99, ratio50, ratio95, ratio99].
    Numeric cells are float or None (None = no data); ratio is None whenever
    either side is missing or REST is exactly 0 (division guard)."""
    rows = []
    for segment_key, segment_label in CANONICAL_SEGMENTS:
        rest = raw.get((metric, segment_key, "rest"), {})
        grpc = raw.get((metric, segment_key, "grpc"), {})
        row = [segment_label]
        for pct in PERCENTILES:
            row.append(rest.get(pct))
        for pct in PERCENTILES:
            row.append(grpc.get(pct))
        for pct in PERCENTILES:
            r, g = rest.get(pct), grpc.get(pct)
            row.append(round(g / r, 2) if (r and g) else None)
        rows.append(row)
    return rows


WIDE_HEADER = [
    "Segmen",
    "REST P50", "REST P95", "REST P99",
    "gRPC P50", "gRPC P95", "gRPC P99",
    "Rasio P50", "Rasio P95", "Rasio P99",
]

# Ratio color banding: distance from 1x on a log2 scale so that 2x and 0.5x
# get the same visual intensity. "good" = gRPC lower than REST, "warn" = gRPC
# higher. Direction is purely descriptive (see caption text), not a judgment
# of better/worse, since that flips depending on the metric (throughput vs
# latency/CPU/RAM).
def ratio_band_class(ratio):
    if ratio is None or ratio <= 0:
        return "na"
    log = math.log2(ratio)
    if abs(log) < 0.15:
        return "neutral"
    direction = "warn" if log > 0 else "good"
    mag = abs(log)
    tier = "light" if mag < 1 else "medium" if mag < math.log2(5) else "strong"
    return f"{direction}-{tier}"


def fmt(value, suffix=""):
    return DASH if value is None else f"{value:.2f}{suffix}"


def write_wide_csv(path, rows):
    path.parent.mkdir(parents=True, exist_ok=True)

    def _do_write():
        with path.open("w", newline="") as f:
            writer = csv.writer(f)
            writer.writerow(WIDE_HEADER)
            for row in rows:
                writer.writerow([row[0]] + [fmt(v) for v in row[1:]])

    _retry_write(path, _do_write)


def write_wide_markdown(path, title, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    lines = [f"### {title}", ""]
    lines.append("| " + " | ".join(WIDE_HEADER) + " |")
    lines.append("|" + "|".join(["---"] * len(WIDE_HEADER)) + "|")
    for row in rows:
        cells = [row[0]]
        cells += [fmt(v) for v in row[1:7]]
        cells += [fmt(v, "×") for v in row[7:10]]
        lines.append("| " + " | ".join(cells) + " |")
    _retry_write(path, lambda: path.write_text("\n".join(lines) + "\n"))


HTML_STYLE = """
:root {
  --ink: #1b2430; --ink-soft: #4b5563; --paper: #f7f8fa; --paper-raised: #ffffff;
  --line: #d9dee4; --accent: #2f5d62; --accent-soft: #e4edec;
  --good-light: #dcefe4; --good-medium: #a9d9bd; --good-strong: #3f8f6c;
  --warn-light: #f7e3da; --warn-medium: #eab596; --warn-strong: #b5502f;
  --neutral: #eef0f2; --na: #f2f2f2;
}
:root[data-theme="dark"] {
  --ink: #e7ebef; --ink-soft: #a8b2bd; --paper: #14181d; --paper-raised: #1c2229;
  --line: #333b44; --accent: #7fb8ba; --accent-soft: #1f2d2d;
  --good-light: #17301f; --good-medium: #1f4a30; --good-strong: #3f8f6c;
  --warn-light: #3a2318; --warn-medium: #5c3320; --warn-strong: #d97a4c;
  --neutral: #262b31; --na: #20242a;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --ink: #e7ebef; --ink-soft: #a8b2bd; --paper: #14181d; --paper-raised: #1c2229;
    --line: #333b44; --accent: #7fb8ba; --accent-soft: #1f2d2d;
    --good-light: #17301f; --good-medium: #1f4a30; --good-strong: #3f8f6c;
    --warn-light: #3a2318; --warn-medium: #5c3320; --warn-strong: #d97a4c;
    --neutral: #262b31; --na: #20242a;
  }
}
* { box-sizing: border-box; }
body { margin: 0; background: var(--paper); color: var(--ink); font-family: -apple-system, "Segoe UI", system-ui, sans-serif; padding: 2.5rem 1.25rem 4rem; }
.page { max-width: 900px; margin: 0 auto; display: flex; flex-direction: column; gap: 2rem; }
h1 { font-family: Iowan Old Style, Charter, Georgia, serif; font-size: 1.9rem; font-weight: 600; letter-spacing: -0.01em; text-wrap: balance; margin: 0; }
.sub { color: var(--ink-soft); font-size: 0.95rem; margin-top: 0.35rem; }
.legend { background: var(--paper-raised); border: 1px solid var(--line); border-radius: 10px; padding: 1rem 1.25rem; display: flex; flex-direction: column; gap: 0.6rem; }
.legend-title { font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.08em; color: var(--ink-soft); font-weight: 600; }
.swatches { display: flex; flex-wrap: wrap; gap: 0.5rem 1.25rem; align-items: center; }
.swatch { display: flex; align-items: center; gap: 0.45rem; font-size: 0.85rem; }
.swatch i { width: 0.9rem; height: 0.9rem; border-radius: 3px; display: inline-block; border: 1px solid rgba(0,0,0,0.08); }
.legend-note { font-size: 0.82rem; color: var(--ink-soft); line-height: 1.5; }
section.table-block { background: var(--paper-raised); border: 1px solid var(--line); border-radius: 10px; padding: 1.25rem 1.25rem 1rem; display: flex; flex-direction: column; gap: 0.75rem; }
section.table-block h2 { font-family: Iowan Old Style, Charter, Georgia, serif; font-size: 1.15rem; font-weight: 600; margin: 0; }
.table-scroll { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; min-width: 720px; font-family: ui-monospace, "SF Mono", "Cascadia Code", Consolas, monospace; font-variant-numeric: tabular-nums; font-size: 0.86rem; }
thead th { background: var(--accent-soft); color: var(--accent); font-family: -apple-system, "Segoe UI", system-ui, sans-serif; font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.06em; font-weight: 600; padding: 0.55rem 0.7rem; text-align: right; border-bottom: 1px solid var(--line); white-space: nowrap; }
thead th:first-child { text-align: left; }
tbody td { padding: 0.5rem 0.7rem; text-align: right; border-bottom: 1px solid var(--line); }
tbody td:first-child { text-align: left; font-family: -apple-system, "Segoe UI", system-ui, sans-serif; font-size: 0.86rem; color: var(--ink); }
tbody tr:last-child td { border-bottom: none; }
.ratio-cell { font-weight: 600; border-radius: 5px; }
.neutral { background: var(--neutral); }
.good-light { background: var(--good-light); } .good-medium { background: var(--good-medium); }
.good-strong { background: var(--good-strong); color: #fff; }
.warn-light { background: var(--warn-light); } .warn-medium { background: var(--warn-medium); }
.warn-strong { background: var(--warn-strong); color: #fff; }
.na { background: var(--na); color: var(--ink-soft); }
.caption { font-size: 0.8rem; color: var(--ink-soft); line-height: 1.5; }
"""

LEGEND_HTML = """
<div class="legend">
  <div class="legend-title">Keterangan Warna Rasio (gRPC ÷ REST)</div>
  <div class="swatches">
    <div class="swatch"><i style="background:var(--good-strong)"></i>gRPC jauh lebih rendah (&lt; 0.2×)</div>
    <div class="swatch"><i style="background:var(--good-medium)"></i>gRPC lebih rendah (0.2×-0.5×)</div>
    <div class="swatch"><i style="background:var(--good-light)"></i>gRPC sedikit lebih rendah (0.5×-0.9×)</div>
    <div class="swatch"><i style="background:var(--neutral); border:1px solid var(--line)"></i>Setara (0.9×-1.1×)</div>
    <div class="swatch"><i style="background:var(--warn-light)"></i>gRPC sedikit lebih tinggi (1.1×-2×)</div>
    <div class="swatch"><i style="background:var(--warn-medium)"></i>gRPC lebih tinggi (2×-5×)</div>
    <div class="swatch"><i style="background:var(--warn-strong)"></i>gRPC jauh lebih tinggi (&gt; 5×)</div>
    <div class="swatch"><i style="background:var(--na)"></i>Tidak ada data</div>
  </div>
  <div class="legend-note">
    Rasio dihitung sebagai nilai gRPC dibagi nilai REST pada persentil yang sama (P50 dengan P50, P95 dengan P95, P99 dengan P99).
    Warna murni menunjukkan arah dan besar selisih (gRPC lebih rendah atau lebih tinggi dari REST) &mdash; bukan penilaian baik/buruk,
    karena arti "lebih rendah lebih baik" berbeda antar metrik (mis. latency vs throughput).
  </div>
</div>
"""

CPU_MEMORY_NOTE = (
    'Baris "Worker ke Gateway" tidak punya data CPU/RAM &mdash; arsitektur sistem sengaja tidak '
    "mengukur CPU/RAM pada segmen ini (lihat <code>video_service.go</code>), bukan kesalahan "
    "pengambilan data. Baris tetap ditampilkan supaya bentuk tabel konsisten dengan tabel lain."
)
RATIO_CAPTION = (
    "Rasio = nilai gRPC P50/P95/P99 dibagi nilai REST pada persentil yang sama. Nilai &lt; 1× "
    "berarti gRPC lebih rendah dari REST pada persentil itu; nilai &gt; 1× berarti gRPC lebih tinggi."
)


def render_table_section(title, unit, rows, extra_note=None):
    head_cells = "".join(f"<th>{h}</th>" for h in WIDE_HEADER)
    body_rows = []
    for row in rows:
        cells = [f"<td>{row[0]}</td>"]
        for v in row[1:7]:
            cls = " class=\"na\"" if v is None else ""
            cells.append(f"<td{cls}>{fmt(v)}</td>")
        for v in row[7:10]:
            cls = ratio_band_class(v)
            cells.append(f'<td class="ratio-cell {cls}">{fmt(v, "×")}</td>')
        body_rows.append(f"<tr>{''.join(cells)}</tr>")
    caption = RATIO_CAPTION + (f" {extra_note}" if extra_note else "")
    return f"""
<section class="table-block">
  <h2>{title} ({unit})</h2>
  <div class="table-scroll">
    <table>
      <thead><tr>{head_cells}</tr></thead>
      <tbody>{''.join(body_rows)}</tbody>
    </table>
  </div>
  <div class="caption">{caption}</div>
</section>"""


def write_payload_html(path, payload_size, sections):
    """sections: list of (title, unit, rows, extra_note_or_None)."""
    path.parent.mkdir(parents=True, exist_ok=True)
    body = "".join(render_table_section(t, u, r, n) for t, u, r, n in sections)
    html = f"""<title>Skenario A — Payload {payload_size.upper()}</title>
<style>{HTML_STYLE}</style>
<div class="page">
  <div>
    <h1>Skenario A — Payload {payload_size.upper()}</h1>
    <div class="sub">Hasil REST vs gRPC &mdash; Latency, Throughput, CPU, dan Memori</div>
  </div>
  {LEGEND_HTML}
  {body}
</div>
"""
    _retry_write(path, lambda: path.write_text(html))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--payload-size", required=True, help="e.g. 10mb, 50mb, 100mb")
    parser.add_argument("--scenario", default="scenario_a")
    parser.add_argument("--prometheus-url", default="http://localhost:9090")
    parser.add_argument("--window", default="90m")
    parser.add_argument(
        "--root",
        default=str(Path(__file__).resolve().parent.parent),
        help="repo root, results/ is created under here",
    )
    args = parser.parse_args()

    root = Path(args.root)
    raw = collect_raw(args.prometheus_url, args.scenario, args.payload_size, args.window)

    if not raw:
        raise SystemExit(
            f"No data returned for payload_size={args.payload_size!r}. "
            "Did the requests actually run and get scraped yet?"
        )

    write_raw_csv(root / "results" / "scenario_a" / "raw.csv", args.payload_size, raw)

    payload_title = args.payload_size.upper()
    html_sections = []
    for metric, unit, _factor in METRICS:
        short = METRIC_SHORT_NAME[metric]
        rows = build_wide_rows(raw, metric)
        base = root / "results" / "scenario_a" / "tables" / f"{args.payload_size}_{short}"
        write_wide_csv(base.with_suffix(".csv"), rows)
        write_wide_markdown(
            base.with_suffix(".md"),
            f"Tabel {short.capitalize()} — Payload {payload_title} ({unit})",
            rows,
        )
        print(f"Wrote {base.with_suffix('.csv')} and .md")
        note = CPU_MEMORY_NOTE if short in ("cpu", "memory") else None
        html_sections.append((short.capitalize(), unit, rows, note))

    html_path = root / "results" / "scenario_a" / "tables" / f"{args.payload_size}.html"
    write_payload_html(html_path, args.payload_size, html_sections)
    print(f"Wrote {html_path}")

    print(f"Done. Raw data appended to results/scenario_a/raw.csv for payload_size={args.payload_size}")


if __name__ == "__main__":
    main()
