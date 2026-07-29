#!/usr/bin/env python3
"""
Build a Wails application with an NSIS installer.

Usage:
    python build.py
    python build.py -clean
    python build.py -debug
"""

import shutil
import subprocess
import sys


def main():
    if shutil.which("wails") is None:
        print("Error: Wails CLI not found in PATH.")
        sys.exit(1)

    cmd = ["wails", "build", "-nsis", *sys.argv[1:]]

    print("Executing:", " ".join(cmd))

    try:
        subprocess.run(cmd, check=True)
        print("\nBuild completed successfully.")
    except subprocess.CalledProcessError as e:
        print(f"\nBuild failed with exit code {e.returncode}.")
        sys.exit(e.returncode)


if __name__ == "__main__":
    main()