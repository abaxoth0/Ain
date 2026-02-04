import csv
import math

filename = "bench.csv"

data = []
with open('/home/abaxoth/proj/Ain/scripts/'+filename, 'r') as f:
    reader = csv.DictReader(f)
    for row in reader:
        data.append(row)

# Group data by struct and type
groups = {}
for row in data:
    key = (row['struct'], row['type'])
    if key not in groups:
        groups[key] = []
    groups[key].append(row)

def analyze_values(values, is_float=True):
    if is_float:
        values = [float(v) for v in values if v != 'N\\A' and v != '']
    else:
        values = [int(v) for v in values if v != 'N\\A' and v != '']

    if not values:
        return 0, 0, 0, 0

    mean_val = sum(values) / len(values)
    variance = sum((x - mean_val) ** 2 for x in values) / len(values)
    std_val = math.sqrt(variance)
    min_val = min(values)
    max_val = max(values)

    return mean_val, std_val, min_val, max_val

print("=== BENCHMARK STATISTICAL RESULTS (100 iterations) ===")
print()

print("=== SINGLE OPERATIONS ===")
for struct in ['Disruptor', 'Channel', 'SyncQueue']:
    key = (struct, 'Single Operation')
    if key in groups:
        rows = groups[key]

        ns_vals = [float(row['ns/op']) for row in rows]
        ns_mean, ns_std, ns_min, ns_max = analyze_values(ns_vals)

        ops_per_sec = 1_000_000_000 / ns_mean

        b_vals = [row['B/op'] for row in rows]
        b_mean, b_std, b_min, b_max = analyze_values(b_vals, False)

        alloc_vals = [row['allocs/op'] for row in rows]
        alloc_mean, alloc_std, alloc_min, alloc_max = analyze_values(alloc_vals, False)

        print(f"\n{struct}:")
        print(f"  ns/op: {ns_mean:.2f} ± {ns_std:.2f} (min: {ns_min:.2f}, max: {ns_max:.2f})")
        print(f"  ops/s: {ops_per_sec:,.0f}")
        print(f"  B/op: {b_mean:.0f} ± {b_std:.0f}")
        print(f"  allocs/op: {alloc_mean:.0f}")

print("\n" + "="*60)

print("=== THROUGHPUT (1,000 items) ===")
for struct in ['Disruptor', 'Channel', 'SyncQueue']:
    key = (struct, 'Throughput')
    if key in groups:
        rows = groups[key]

        ops_vals = [float(row['op/s']) for row in rows]
        ops_mean, ops_std, ops_min, ops_max = analyze_values(ops_vals)

        ns_vals = [float(row['ns/op']) for row in rows]
        ns_mean, ns_std, ns_min, ns_max = analyze_values(ns_vals)

        b_vals = [row['B/op'] for row in rows]
        b_mean, b_std, b_min, b_max = analyze_values(b_vals, False)

        alloc_vals = [row['allocs/op'] for row in rows]
        alloc_mean, alloc_std, alloc_min, alloc_max = analyze_values(alloc_vals, False)

        print(f"\n{struct}:")
        print(f"  items/sec: {ops_mean:,.0f} ± {ops_std:,.0f} (min: {ops_min:,.0f}, max: {ops_max:,.0f})")
        print(f"  ns/op: {ns_mean:,.0f} ± {ns_std:,.0f}")
        print(f"  B/op: {b_mean:.0f} ± {b_std:.0f}")
        print(f"  allocs/op: {alloc_mean:.0f}")

print("\n" + "="*60)

print("=== SAMPLE SIZES ===")
for key, rows in groups.items():
    print(f"{key[0]} {key[1]}: {len(rows)} iterations")

