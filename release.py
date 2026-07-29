#!/usr/bin/env python3
"""
AI-powered GitHub Release Script.

Usage:
    python release.py
    python release.py v1.2.3
    python release.py --draft
    python release.py --prerelease
    python release.py --dry-run

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

from openai import OpenAI


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

    api_key = os.getenv("FAST_LLM_API_KEY")
    base_url = os.getenv("FAST_LLM_BASE_URL")

    if not api_key:
        raise RuntimeError("FAST_LLM_API_KEY is not set.")

    if not base_url:
        raise RuntimeError("FAST_LLM_BASE_URL is not set.")

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
        model=MODEL,
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
    # Validate worktree cleanliness
    # ----------------------------------------------------------

    status_out = run_cmd(["git", "status", "--porcelain"], capture=True).stdout.strip()
    if status_out:
        print("Error: Worktree is not clean. Commit or stash changes before releasing.")
        sys.exit(1)

    # ----------------------------------------------------------
    # Create / Push Tag (with retry/idempotency checks)
    # ----------------------------------------------------------

    head_commit = run_cmd(["git", "rev-parse", "HEAD"], capture=True).stdout.strip()

    # Check local tag
    local_tags = run_cmd(["git", "tag", "-l", tag_name], capture=True).stdout.strip()
    if local_tags:
        tag_commit = run_cmd(["git", "rev-parse", f"{tag_name}^{{commit}}"], capture=True).stdout.strip()
        if tag_commit != head_commit:
            print(f"Error: Tag {tag_name} exists locally but points to {tag_commit}, not HEAD ({head_commit}).")
            sys.exit(1)
        print(f"Tag {tag_name} already exists locally on HEAD. Skipping creation.")
    else:
        print("Creating tag...")
        run_cmd(["git", "tag", "-a", tag_name, "-m", f"Release {tag_name}"])

    # Check remote tag
    ls_remote = run_cmd(["git", "ls-remote", "origin", f"refs/tags/{tag_name}"], capture=True).stdout.strip()
    if ls_remote:
        remote_commit = ls_remote.split()[0]
        tag_commit = run_cmd(["git", "rev-parse", f"{tag_name}^{{commit}}"], capture=True).stdout.strip()
        if remote_commit != tag_commit:
            print(f"Error: Remote tag {tag_name} points to {remote_commit}, not {tag_commit}.")
            sys.exit(1)
        print(f"Tag {tag_name} already pushed to remote. Skipping push.")
    else:
        print("Pushing tag...")
        run_cmd(["git", "push", "origin", tag_name])

    # ----------------------------------------------------------
    # Create Release (with retry/idempotency check)
    # ----------------------------------------------------------

    rel_check = run_cmd(["gh", "release", "view", tag_name], capture=True, check=False)
    if rel_check.returncode == 0:
        print(f"GitHub Release {tag_name} already exists. Skipping release creation.")
    else:
        print("Creating GitHub Release...")
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
        run_cmd(gh_cmd)

    print()
    print(f"Release {tag_name} processed successfully.")


if __name__ == "__main__":
    main()