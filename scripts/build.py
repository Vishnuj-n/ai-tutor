#!/usr/bin/env python3
"""
Build a Wails application with an NSIS installer.

Usage:
    python scripts/build.py
    python scripts/build.py -clean
    python scripts/build.py -debug
"""

import os
import shutil
import subprocess
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parent.parent


def main():
    os.chdir(PROJECT_ROOT)

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
