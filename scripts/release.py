#!/usr/bin/env python3
"""
AI-powered GitHub Release Script.

Usage:
    python scripts/release.py
    python scripts/release.py v1.2.3
    python scripts/release.py --draft
    python scripts/release.py --prerelease
    python scripts/release.py --dry-run

Environment Variables:
    FAST_LLM_API_KEY
    FAST_LLM_BASE_URL

The script uses:
    model = openai/gpt-oss-120b

Workflow:
1. Determine release tag from CLI or VERSION file.
2. Find previous git tag.
3. Collect commits since previous tag.
4. Generate release notes using AI.
5. Preview release notes.
6. Create git tag.
7. Push tag.
8. Create GitHub release.
"""

import argparse
import os
import subprocess
import sys
from pathlib import Path

# Ensure UTF-8 encoding for standard output/error streams on Windows
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
if hasattr(sys.stderr, "reconfigure"):
    sys.stderr.reconfigure(encoding="utf-8")

PROJECT_ROOT = Path(__file__).resolve().parent.parent
os.chdir(PROJECT_ROOT)

from openai import OpenAI


# ---------------------------------------------------------------------
# Load .env file if present
# ---------------------------------------------------------------------
def load_env_file(filepath=".env"):
    if not os.path.exists(filepath):
        return
    with open(filepath, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, val = line.split("=", 1)
            key = key.strip()
            val = val.strip().strip("'\"")
            if key and key not in os.environ:
                os.environ[key] = val


load_env_file()


MODEL = "openai/gpt-oss-120b"


# ---------------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------------


def run_cmd(cmd, check=True, capture=False):
    """Run a shell command."""

    result = subprocess.run(
        cmd,
        check=check,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        text=True,
    )
    return result


# ---------------------------------------------------------------------
# Git Helpers
# ---------------------------------------------------------------------


def get_latest_tag():
    """Return latest git tag or None."""

    try:
        return run_cmd(
            ["git", "describe", "--tags", "--abbrev=0"],
            capture=True,
        ).stdout.strip()
    except subprocess.CalledProcessError:
        return None


def get_commit_history(previous_tag=None):
    """
    Returns detailed commit history since previous tag.

    Includes commit body so AI has more context.
    """

    cmd = [
        "git",
        "log",
        "--no-merges",
        "--pretty=format:Commit: %h%nAuthor: %an%nSubject: %s%nBody:%n%b%n---",
    ]

    if previous_tag:
        cmd.append(f"{previous_tag}..HEAD")

    return run_cmd(cmd, capture=True).stdout.strip()


# ---------------------------------------------------------------------
# AI Release Notes
# ---------------------------------------------------------------------


def generate_release_notes(tag_name, commit_history):
    """
    Generate release notes using GPT-OSS-120B.
    """

    api_key = os.getenv("FAST_LLM_API_KEY") or os.getenv("HEAVY_LLM_API_KEY")
    base_url = (
        os.getenv("FAST_LLM_BASE_URL")
        or os.getenv("HEAVY_LLM_BASE_URL")
        or "https://api.openai.com/v1"
    )

    if base_url:
        base_url = base_url.rstrip("/")
        if base_url.endswith("/openai"):
            base_url = base_url[:-7] + "/openai/v1"

    if not api_key:
        raise RuntimeError("Neither FAST_LLM_API_KEY nor HEAVY_LLM_API_KEY is set.")

    model = (
        os.getenv("FAST_LLM_MODEL")
        or os.getenv("HEAVY_LLM_MODEL")
        or MODEL
    )

    client = OpenAI(
        api_key=api_key,
        base_url=base_url,
    )

    prompt = f"""
You are an experienced open-source maintainer.

Generate professional GitHub Release Notes.

Version:
{tag_name}

Commit history:

{commit_history}

Requirements:

- Output ONLY markdown.
- Don't mention commit hashes.
- Don't mention authors.
- Group related commits.
- Rewrite technical commit messages into user-friendly release notes.
- Combine multiple commits belonging to one feature.
- Ignore trivial commits unless important.

Structure:

# What's New

## ✨ Features

## 🚀 Improvements

## 🐛 Bug Fixes

## 🧹 Maintenance

## 📦 Full Changelog

At the end include:

"Thanks for using Studyloop!"

Keep it concise.
"""

    response = client.chat.completions.create(
        model=model,
        temperature=0.3,
        messages=[
            {
                "role": "system",
                "content": (
                    "You write high quality GitHub release notes in Markdown."
                ),
            },
            {
                "role": "user",
                "content": prompt,
            },
        ],
    )

    return response.choices[0].message.content.strip()


# ---------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(
        description="Create AI-powered GitHub releases."
    )

    parser.add_argument(
        "tag",
        nargs="?",
        help="Release tag (defaults to VERSION file)",
    )

    parser.add_argument(
        "--draft",
        action="store_true",
        help="Create draft release",
    )

    parser.add_argument(
        "--prerelease",
        action="store_true",
        help="Create prerelease",
    )

    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Generate release notes only",
    )

    args = parser.parse_args()

    # ----------------------------------------------------------
    # Determine tag
    # ----------------------------------------------------------

    tag_name = args.tag

    if not tag_name:
        if not os.path.exists("VERSION"):
            print(
                "VERSION file not found and no tag supplied.",
                file=sys.stderr,
            )
            sys.exit(1)

        with open("VERSION", encoding="utf-8") as f:
            tag_name = f.read().strip()

    if not tag_name.startswith("v"):
        tag_name = "v" + tag_name

    print(f"Preparing release {tag_name}")

    # ----------------------------------------------------------
    # Previous tag
    # ----------------------------------------------------------

    previous_tag = get_latest_tag()

    if previous_tag:
        print(f"Previous tag: {previous_tag}")
    else:
        print("No previous tag found.")

    # ----------------------------------------------------------
    # Commit history
    # ----------------------------------------------------------

    commit_history = get_commit_history(previous_tag)

    if not commit_history:
        commit_history = "No commits."

    # ----------------------------------------------------------
    # AI Release Notes
    # ----------------------------------------------------------

    print("Generating release notes using AI...")

    try:
        release_notes = generate_release_notes(
            tag_name,
            commit_history,
        )
    except Exception as e:
        print(f"AI generation failed:\n{e}")

        release_notes = (
            "# What's Changed\n\n"
            "_Automatic AI generation failed._\n\n"
            "See commit history for details."
        )

    print()
    print("=" * 70)
    print(release_notes)
    print("=" * 70)
    print()

    if args.dry_run:
        print("Dry run complete.")
        return

    # ----------------------------------------------------------
    # Check gh
    # ----------------------------------------------------------

    try:
        run_cmd(["gh", "--version"], capture=True)
    except Exception:
        print("GitHub CLI (gh) not installed.")
        sys.exit(1)

    # ----------------------------------------------------------
    # Create Local Tag (if missing)
    # ----------------------------------------------------------

    head_commit = run_cmd(["git", "rev-parse", "HEAD"], capture=True).stdout.strip()
    local_tags = run_cmd(["git", "tag", "-l", tag_name], capture=True).stdout.strip()
    if not local_tags:
        print(f"Creating local tag {tag_name}...")
        run_cmd(["git", "tag", "-a", tag_name, "-m", f"Release {tag_name}"])
    else:
        print(f"Tag {tag_name} already exists locally.")

    print(f"Pushing tag {tag_name} to origin...")
    run_cmd(["git", "push", "origin", tag_name], check=False)

    # ----------------------------------------------------------
    # Collect Release Assets
    # ----------------------------------------------------------

    bin_dir = os.path.join("build", "bin")
    asset_candidates = [
        "Studyloop.exe",
        "Studyloop-amd64-installer.exe",
        "rag-assets.zip",
    ]
    assets = []
    if os.path.exists(bin_dir):
        for candidate in asset_candidates:
            asset_path = os.path.join(bin_dir, candidate)
            if os.path.isfile(asset_path):
                assets.append(asset_path)
                print(f"Found release asset: {asset_path}")

    # ----------------------------------------------------------
    # Create GitHub Release (gh creates & pushes tag automatically)
    # ----------------------------------------------------------

    rel_check = run_cmd(["gh", "release", "view", tag_name], capture=True, check=False)
    if rel_check.returncode == 0:
        print(f"GitHub Release {tag_name} already exists. Skipping release creation.")
    else:
        print("Creating GitHub Release via gh...")
        gh_cmd = [
            "gh",
            "release",
            "create",
            tag_name,
            "--title",
            f"Release {tag_name}",
            "--notes",
            release_notes,
        ]
        if args.draft:
            gh_cmd.append("--draft")
        if args.prerelease:
            gh_cmd.append("--prerelease")
        if assets:
            gh_cmd.extend(assets)

        run_cmd(gh_cmd)

    print()
    print(f"Release {tag_name} processed successfully.")


if __name__ == "__main__":
    main()
