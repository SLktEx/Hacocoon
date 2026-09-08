"""Observe automatic Base retention after the ordinary storage CLI create."""
import json
import pathlib
import subprocess
import sys
from cleanup_ci_base_asset import PROJECT, owned_asset, validate_instance


def main():
    data = json.loads(pathlib.Path(sys.argv[1]).read_text())
    environment = data["environments"][sys.argv[2]]
    assets = [a for a in data.get("base_assets", {}).values() if a.get("base") == environment.get("base")]
    if len(assets) != 1:
        raise RuntimeError("ordinary creation did not retain an exact Base")
    name = "haco-base-" + assets[0]["owner"]
    asset = owned_asset(data, name, require_unused=False)
    result = subprocess.run(["incus", "query", "/1.0/instances/" + name + "?project=" + PROJECT], capture_output=True, text=True, timeout=60)
    if result.returncode:
        raise RuntimeError("retained Base observation failed")
    validate_instance(asset, json.loads(result.stdout))
    print("BASE RETENTION: ordinary create recorded the exact ready Base and isolated stopped storage")


if __name__ == "__main__":
    main()
