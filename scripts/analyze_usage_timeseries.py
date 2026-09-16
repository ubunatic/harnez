#!/usr/bin/env python3
"""
analyze_usage_timeseries.py
Analyzes multi-agent usage history, token burn velocity, model transitions,
caching efficiency, and quota dynamics across fleet hosts over the 100-day window (June-Sept 2026).
"""

import json
import os
import glob
from datetime import datetime
from collections import defaultdict

USAGE_DIR = os.path.expanduser("~/.claude/harnez/usage-history")
STATS_CACHE = os.path.expanduser("~/.claude/stats-cache.json")

def load_jsonl(path):
    if not os.path.exists(path):
        return []
    records = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    records.append(json.loads(line))
                except json.JSONDecodeError:
                    pass
    return records

def parse_iso_date(ts_str):
    # Extracts YYYY-MM-DD
    return ts_str[:10]

def analyze():
    print("================================================================================")
    print(" FLEET USAGE & TOKEN TIMESERIES ANALYSIS (June 2026 - September 2026)")
    print("================================================================================\n")

    # Load host datasets
    x600_data = load_jsonl(os.path.join(USAGE_DIR, "x600.jsonl"))
    t14_data = load_jsonl(os.path.join(USAGE_DIR, "t14.jsonl"))
    um760_data = load_jsonl(os.path.join(USAGE_DIR, "um760.jsonl"))
    quota_data = load_jsonl(os.path.join(USAGE_DIR, "quota-history.jsonl"))

    print(f"Loaded Datasets:")
    print(f"  - x600.jsonl:  {len(x600_data)} snapshots")
    print(f"  - t14.jsonl:   {len(t14_data)} snapshots")
    print(f"  - um760.jsonl: {len(um760_data)} snapshots")
    print(f"  - quota-history.jsonl: {len(quota_data)} snapshots\n")

    # 1. Timeline & Token Volume Evolution (from x600 chronosequence)
    print("--------------------------------------------------------------------------------")
    print(" 1. CHRONOLOGICAL TOKEN GROWTH & VELOCITY (x600 Anchor Series)")
    print("--------------------------------------------------------------------------------")
    print("Date       | Cumulative (M) | Daily Burn (M) | Cache Read (M) | Cache % | Active / Top Models")
    print("-----------+----------------+----------------+----------------+---------+--------------------------------------")
    
    prev_tot = 0
    burn_rates = []
    for snap in x600_data:
        dt = parse_iso_date(snap["timestamp"])
        for a in snap.get("agents", []):
            if a["agent_id"] == "claude":
                tok = a.get("tokens", {})
                tot = tok.get("total_tokens", 0)
                diff = max(0, tot - prev_tot)
                prev_tot = tot
                cr = tok.get("cache_read_tokens", 0)
                cpct = (cr / tot * 100) if tot > 0 else 0
                mt = a.get("model_tokens", {})
                
                # identify top models
                sorted_models = sorted(mt.items(), key=lambda x: x[1], reverse=True)
                top_m_str = ", ".join([f"{m.replace('claude-', '')}: {cnt/1e6:.1f}M" for m, cnt in sorted_models if cnt > 0][:3])
                
                burn_rates.append((dt, tot, diff, cr, cpct, top_m_str))
                print(f"{dt:10s} | {tot/1e6:14.2f} | {diff/1e6:14.2f} | {cr/1e6:14.2f} | {cpct:6.1f}% | {top_m_str}")

    # 2. Peak Activity Spikes
    print("\n--------------------------------------------------------------------------------")
    print(" 2. PEAK TOKEN BURN SPIKES & EPOCHS")
    print("--------------------------------------------------------------------------------")
    # Sort by diff
    spikes = sorted(burn_rates, key=lambda x: x[2], reverse=True)[:6]
    for rank, (dt, tot, diff, cr, cpct, top_m) in enumerate(spikes, 1):
        print(f"  Top {rank}: {dt} -> +{diff/1e6:,.2f} M tokens burn (Total: {tot/1e6:,.2f} M, Cache: {cpct:.1f}%)")

    # 3. Model Distribution & Evolution
    print("\n--------------------------------------------------------------------------------")
    print(" 3. MODEL GENERATION TRANSITIONS & TOKEN TOTALS")
    print("--------------------------------------------------------------------------------")
    
    # Extract latest model breakdown across hosts
    def get_latest_claude_breakdown(dataset):
        if not dataset:
            return {}
        last = dataset[-1]
        for a in last.get("agents", []):
            if a["agent_id"] == "claude":
                return a.get("model_tokens", {})
        return {}

    t14_models = get_latest_claude_breakdown(t14_data)
    x600_models = get_latest_claude_breakdown(x600_data)
    um760_models = get_latest_claude_breakdown(um760_data)

    all_models = sorted(list(set(list(t14_models.keys()) + list(x600_models.keys()) + list(um760_models.keys()))))
    print(f"{'Model Name':<28} | {'x600 Tokens':<15} | {'t14 Tokens':<15} | {'um760 Tokens':<15}")
    print("-" * 78)
    for m in all_models:
        if m == "<synthetic>": continue
        print(f"{m:<28} | {x600_models.get(m, 0):>14,} | {t14_models.get(m, 0):>14,} | {um760_models.get(m, 0):>14,}")

    # 4. Agent Multi-Fleet Footprint
    print("\n--------------------------------------------------------------------------------")
    print(" 4. FLEET-WIDE MULTI-AGENT INVENTORY & ACTIVE TIERS")
    print("--------------------------------------------------------------------------------")
    for name, data in [("x600 (Dev Box / Anchor)", x600_data), ("t14 (Mobile / Sprint)", t14_data), ("um760 (Mini Server)", um760_data)]:
        if not data: continue
        last = data[-1]
        print(f"\nHost: {name} (Snapshot: {last['timestamp'][:19]})")
        for a in last.get("agents", []):
            aid = a.get("agent_id")
            aname = a.get("name")
            act = a.get("active_model", "Auto / Default")
            plan = a.get("plan_tier", "Standard")
            tok = a.get("tokens")
            tok_str = f"{tok['total_tokens']:,} tokens (Cache: {tok['cache_read_tokens']:,})" if tok else "N/A (Quota tracking)"
            print(f"  - [{aid}] {aname:<20} | Plan: {plan:<8} | Active Model: {str(act):<24} | Tokens: {tok_str}")

    # 5. Quota Dynamics Summary
    print("\n--------------------------------------------------------------------------------")
    print(" 5. QUOTA WINDOW DYNAMICS & SUSTAINABILITY (Current Snapshot)")
    print("--------------------------------------------------------------------------------")
    for q in quota_data:
        ag = q.get("agent")
        win = q.get("window")
        used = q.get("used_percent")
        rem = q.get("remaining_percent")
        reset = q.get("reset_at")
        print(f"  Agent: {ag:<8} | Window: {win:<26} | Used: {used:3d}% | Remaining: {rem:3d}% | Reset: {reset}")

if __name__ == "__main__":
    analyze()
