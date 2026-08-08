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

# Production Sync & Research Analytics Credentials (Injected into binary via -ldflags during build)
# You can change these constants directly here or set environment variables before building.
PRODUCTION_SYNC_URL = os.environ.get(
    "CLOUD_SYNC_URL",
    "https://dkqahgkkighcpycexovi.supabase.co/rest/v1/rpc/handle_cloud_sync",
)
PRODUCTION_ANON_KEY = os.environ.get(
    "SUPABASE_ANON_KEY",
    "sb_publishable_Gno-X5ppMB6YZza52F4Nog__7kxobfX",
)
PRODUCTION_RESEARCH_URL = os.environ.get(
    "RESEARCH_ANALYTICS_URL",
    "https://rptpauakhdsqinpcnebw.supabase.co/rest/v1/anonymous_analytics_events",
)
PRODUCTION_RESEARCH_ANON_KEY = os.environ.get(
    "RESEARCH_ANALYTICS_ANON_KEY",
    "sb_publishable_aL0Wgco3ZzH_OS64pP4g-w_tWRN_bNf",
)


def main():
    os.chdir(PROJECT_ROOT)

    if shutil.which("wails") is None:
        print("Error: Wails CLI not found in PATH.")
        sys.exit(1)

    ldflags = (
        f"-X ai-tutor/internal/study.DefaultProductionSyncURL={PRODUCTION_SYNC_URL} "
        f"-X ai-tutor/internal/study.DefaultProductionAnonKey={PRODUCTION_ANON_KEY} "
        f"-X ai-tutor/internal/study.DefaultResearchAnalyticsURL={PRODUCTION_RESEARCH_URL} "
        f"-X ai-tutor/internal/study.DefaultResearchAnalyticsAnonKey={PRODUCTION_RESEARCH_ANON_KEY}"
    )

    cmd = ["wails", "build", "-nsis", "-ldflags", ldflags, *sys.argv[1:]]

    print("Executing:", " ".join(cmd))

    try:
        subprocess.run(cmd, check=True)
        print("\nBuild completed successfully.")
    except subprocess.CalledProcessError as e:
        print(f"\nBuild failed with exit code {e.returncode}.")
        sys.exit(e.returncode)


if __name__ == "__main__":
    main()
