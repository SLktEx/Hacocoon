#!/usr/bin/env python3
"""Installed Linux stages on the existing dedicated Windows/WSL acceptance runner.

This is an internal integration gate, separate from the exact daily user-path
gate. It never stops WSL, compacts a VHD or independently mounts/deletes storage.
"""
import json
import os
import subprocess
import uuid

def registration():
    import winreg
    found = []
    with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Microsoft\Windows\CurrentVersion\Lxss") as root:
        for i in range(winreg.QueryInfoKey(root)[0]):
            name = winreg.EnumKey(root, i)
            with winreg.OpenKey(root, name) as key:
                distro, kind = winreg.QueryValueEx(key, "DistributionName")
                version, version_kind = winreg.QueryValueEx(key, "Version")
                if kind == winreg.REG_SZ and distro == "Hacocoon":
                    assert version_kind == winreg.REG_DWORD and version == 2
                    found.append("{" + str(uuid.UUID(name)) + "}")
    assert len(found) == 1, "exact dedicated WSL registration required"
    return found[0]

def main():
    assert os.name == "nt", "requires the installed Windows/WSL gate"
    reg = registration()
    def run(*args, expected=0):
        p = subprocess.run(["wsl.exe", "--distribution-id", reg, "--user", "root",
                            "--cd", "/", "--exec", "/usr/bin/env", "-i",
                            "PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", *args],
                           capture_output=True, text=True, encoding="utf-8", timeout=450)
        assert p.returncode == expected, f"installed reclamation command exit {p.returncode}, expected {expected}"
        return p.stdout
    identity = json.loads(run("/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration"))
    assert identity["registration_id"] == reg
    # Ordinary setup establishes Incus-owned Host/pool use after cold WSL entry.
    run("haco", "setup")
    # Existing installer sentinel was created through the ordinary Host session.
    marker = 'test "$(cat "$HOME/.hacocoon-installer-acceptance")" = kept-through-restart-and-rerun'
    run("incus", "exec", "haco-host", "--project", "hacocoon", "--", "sh", "-ec", marker)
    foreign = str(uuid.uuid4())
    assert foreign != identity["installation_id"]
    refused = json.loads(run("haco", "_reclaim-linux", reg, foreign, expected=1))
    assert refused["failure"] == "identity_changed"
    assert all(refused[k]["status"] == "skipped" and not refused[k]["attempted"] for k in ("incus_btrfs_loop", "wsl_ext4"))
    report = json.loads(run("haco", "_reclaim-linux", reg, identity["installation_id"]))
    assert report["protocol_version"] == 1 and not report.get("failure") and not report.get("cleanup_failed")
    for k in ("incus_btrfs_loop", "wsl_ext4"):
        assert report[k]["status"] == "complete" and report[k]["attempted"]
        assert type(report[k]["kernel_trimmed_bytes"]) is int and report[k]["kernel_trimmed_bytes"] >= 0
        assert report[k]["filesystem_before"]["capacity_bytes"] == report[k]["filesystem_after"]["capacity_bytes"] > 0
        for field in ("filesystem_before", "filesystem_after"):
            assert 0 <= report[k][field]["used_bytes"] <= report[k][field]["capacity_bytes"]
    pool = report["incus_btrfs_loop"]
    assert pool["before"]["logical_bytes"] == pool["after"]["logical_bytes"] > 0
    assert "before" not in report["wsl_ext4"] and "after" not in report["wsl_ext4"]
    run("incus", "exec", "haco-host", "--project", "hacocoon", "--", "sh", "-ec", marker)
    print(json.dumps({"linux_stages": "PASS", "foreign_identity": "refused",
                      "host_sentinel": "retained", "observations": report}))
    print("Windows VHDX compaction and whole persistent-data acceptance: not exercised by this gate")

if __name__ == "__main__":
    main()
