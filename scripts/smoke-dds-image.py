#!/usr/bin/env python3
import json
import pathlib
import subprocess
import sys
import tempfile


def main():
    if len(sys.argv) not in (2, 3) or (len(sys.argv) == 3 and sys.argv[2] != "--zero-swap"):
        raise SystemExit("Usage: smoke-dds-image.py IMAGE [--zero-swap]")
    repositoryRoot = pathlib.Path(__file__).resolve().parent.parent
    fixturesPath = repositoryRoot / "apps/api/internal/analysis/testdata/dds-golden.json"
    fixtures = json.loads(fixturesPath.read_text())["fixtures"]
    vulnerabilityCycle = [0, 2, 3, 1, 2, 3, 1, 0, 3, 1, 0, 2, 1, 0, 2, 3]
    for fixture in fixtures:
        boardNumber = fixture["boardNumber"]
        values = [(boardNumber - 1) % 4, vulnerabilityCycle[(boardNumber - 1) % 16]]
        for seat in ["north", "east", "south", "west"]:
            for suit in "SHDC":
                values.append(sum(
                    1 << ("23456789TJQKA".index(card["rank"]) + 2)
                    for card in fixture["deal"][seat] if card["suit"] == suit
                ))
        with tempfile.TemporaryDirectory(prefix="dds-memory-probe-") as probeDirectory:
            probeArguments = []
            if len(sys.argv) == 3:
                probePath = pathlib.Path(probeDirectory) / "free"
                probePath.write_text(
                    "#!/bin/sh\n/usr/bin/free \"$@\" | awk '/^Swap:/ {$2=0; $3=0; $4=0} {print}'\n"
                )
                probePath.chmod(0o755)
                probeArguments = [
                    "--mount", f"type=bind,src={probePath},dst=/dds-probe/free,readonly",
                    "--env", "PATH=/dds-probe:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                ]
            result = subprocess.run(
                ["docker", "run", "--rm", "-i", "--memory=256m", "--cpus=1", *probeArguments,
                 "--entrypoint", "/workspace/bin/bridgeyok-dds", sys.argv[1]],
                input=" ".join(map(str, values)), text=True, capture_output=True, timeout=30,
            )
        if result.returncode:
            raise SystemExit(
                f"DDS image failed fixture {fixture['name']} (exit {result.returncode})\n"
                f"stdout: {result.stdout.strip()}\nstderr: {result.stderr.strip()}"
            )
        output = json.loads(result.stdout)
        expected = fixture["expected"]
        expectedTable = [
            [expected["doubleDummyTable"][seat][strain] for seat in "NESW"]
            for strain in ["S", "H", "D", "C", "NT"]
        ]
        expectedContracts = [
            {"level": contract["level"], "denomination": ["NT", "S", "H", "D", "C"].index(contract["strain"]),
             "seats": {("N",): 0, ("E",): 1, ("S",): 2, ("W",): 3, ("N", "S"): 4, ("E", "W"): 5}[tuple(contract["declarers"])],
             "overTricks": contract["overTricks"], "underTricks": contract["underTricks"]}
            for contract in expected["par"]["contracts"]
        ]
        if (output["solverVersion"] != expected["solverVersion"]
                or output["table"] != expectedTable
                or output["scoreNS"] != expected["par"]["scoreNS"]
                or output["contracts"] != expectedContracts):
            raise SystemExit(f"DDS image golden mismatch: {fixture['name']}")
        print(f"DDS image {fixture['name']}: PASS")


if __name__ == "__main__":
    main()
