import subprocess
import argparse
from datetime import datetime, timezone

def is_any_number(s):
    try:
        float(s)
        return True
    except ValueError:
        return False

def create_csv_report(filename: str):
    print("Creating CSV report...")

    report = "struct,type,cores,iterations,ns/op,MB/s,op/s,B/op,allocs/op\n"
    with open(filename+".result", "r") as result:
        while True:
            row = result.readline()
            if ":" in row: # skip system info
                continue
            if row == "" or row == "PASS\n":
                break
            entry = {}
            if "SyncQueue" in row:
                entry["struct"] = "SyncQueue"
            elif "Disruptor" in row:
                entry["struct"] = "Disruptor"
            elif "Channel" in row:
                entry["struct"] = "Channel"
            else:
                raise ValueError("Unknow struct")
            row = row.split()
            entry["type"] = "Throughput" if len(row)==12 else "Single Operation"
            try:
                coreIdx = row[0].index("-")
                entry["cores"] = int(row[0][coreIdx+1:len(row[0])])
            except ValueError:
                pass
            row = list(filter(lambda c: is_any_number(c),row))
            match len(row):
                case 6:
                    entry["iters"] = int(row[0])
                    entry["ns/op"] = float(row[1])
                    entry["MB/s"] = float(row[2])
                    entry["op/s"] = int(row[3])
                    entry["B/op"] = int(row[4])
                    entry["allocs/op"] = int(row[5])
                    for key, value in entry.items():
                        report += f"{value}"
                        if key != "allocs/op":
                            report += ","
                case 4:
                    entry["iters"] = int(row[0])
                    entry["ns/op"] = float(row[1])
                    entry["B/op"] = int(row[2])
                    entry["allocs/op"] = int(row[3])
                    for key, value in entry.items():
                        if key == "B/op":
                            report += "N\\A,N\\A,"
                        report += f"{value}"
                        if key != "allocs/op":
                            report += ","

            report += "\n"
    with open(filename+".csv", "w") as output:
        output.write(report)

    print(f"CSV report created ({filename}.csv)")

def bench(count: int, path: str):
    # RFC3339
    now = datetime.now(timezone.utc).isoformat(timespec='seconds').replace('+00:00', 'Z')
    output_file = f"bench_{now}"

    print("Benchmark in progres...")
    subprocess.run(f'go test -timeout 0 -bench=. -count={count} -benchmem {path} >> {output_file}.result', shell=True, check=True)
    print("Benchmark finished")

    create_csv_report(output_file)

    subprocess.run(["rm", output_file+".result"])

def main():
    parser = argparse.ArgumentParser(prog="Benchmark runner for Ain")
    parser.add_argument("-c", "--count", default=1)
    parser.add_argument("--path", default=".")
    args = parser.parse_args()

    bench(args.count, args.path)

if __name__ == "__main__":
    main()
