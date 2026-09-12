"""Verify ordinary Incus creation needs no separate retained Base instance."""
import json
import pathlib
import sys


def main():
    data = json.loads(pathlib.Path(sys.argv[1]).read_text())
    environment = data["environments"][sys.argv[2]]
    if not environment.get("base"):
        raise RuntimeError("ordinary creation lost Base provenance")
    if data.get("base_assets"):
        raise RuntimeError("fresh CLI fixture unexpectedly retained Base storage")
    print("BASE: ordinary create kept provenance without extra retained storage")


if __name__ == "__main__":
    main()
