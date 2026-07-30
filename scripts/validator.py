#!/usr/bin/env python3
"""Validate a repository PORTFOLIO.json manifest."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


REQUIRED_FIELDS = {
    "title",
    "summary",
    "category",
    "date",
    "tier",
    "priority",
    "techStack",
}
OPTIONAL_FIELDS = {"highlights", "media", "links"}
DATE_PATTERN = re.compile(r"^\d{4}-(0[1-9]|1[0-2])$")


def validate_string(
    value: Any,
    field: str,
    errors: list[str],
    minimum: int,
    maximum: int,
) -> None:
    if not isinstance(value, str):
        errors.append(f"{field} must be a string")
        return

    if value != value.strip():
        errors.append(f"{field} must not start or end with whitespace")

    if not minimum <= len(value) <= maximum:
        errors.append(f"{field} must contain {minimum} to {maximum} characters")


def validate_string_array(
    value: Any,
    field: str,
    errors: list[str],
    minimum: int,
    maximum: int,
    item_minimum: int,
    item_maximum: int,
) -> None:
    if not isinstance(value, list):
        errors.append(f"{field} must be an array")
        return

    if not minimum <= len(value) <= maximum:
        errors.append(f"{field} must contain {minimum} to {maximum} items")

    normalized_items: set[str] = set()
    for index, item in enumerate(value):
        item_field = f"{field}[{index}]"
        validate_string(item, item_field, errors, item_minimum, item_maximum)
        if isinstance(item, str):
            normalized = item.strip().casefold()
            if normalized in normalized_items:
                errors.append(f"{item_field} duplicates another {field} item")
            normalized_items.add(normalized)


def validate_https_url(value: Any, field: str, errors: list[str]) -> None:
    if not isinstance(value, str) or not value:
        errors.append(f"{field} must be a non-empty HTTPS URL")
        return

    parsed = urlparse(value)
    if parsed.scheme != "https" or not parsed.netloc or parsed.username or parsed.password:
        errors.append(f"{field} must be an absolute HTTPS URL without credentials")


def validate_url_group(
    value: Any,
    field: str,
    allowed_fields: set[str],
    errors: list[str],
) -> None:
    if not isinstance(value, dict):
        errors.append(f"{field} must be an object")
        return

    if not value:
        errors.append(f"{field} must be omitted when it has no values")
        return

    unknown_fields = sorted(set(value) - allowed_fields)
    for unknown in unknown_fields:
        errors.append(f"{field}.{unknown} is not an allowed field")

    for key in sorted(set(value) & allowed_fields):
        validate_https_url(value[key], f"{field}.{key}", errors)


def validate_manifest(manifest: Any) -> list[str]:
    errors: list[str] = []

    if not isinstance(manifest, dict):
        return ["manifest root must be a JSON object"]

    manifest_fields = set(manifest)
    for field in sorted(REQUIRED_FIELDS - manifest_fields):
        errors.append(f"missing required field: {field}")

    for field in sorted(manifest_fields - REQUIRED_FIELDS - OPTIONAL_FIELDS):
        errors.append(f"{field} is not an allowed field")

    if "title" in manifest:
        validate_string(manifest["title"], "title", errors, 2, 80)
    if "summary" in manifest:
        validate_string(manifest["summary"], "summary", errors, 40, 500)
    if "category" in manifest:
        validate_string(manifest["category"], "category", errors, 2, 80)

    if "date" in manifest:
        date = manifest["date"]
        if not isinstance(date, str) or not DATE_PATTERN.fullmatch(date):
            errors.append("date must use YYYY-MM format with a valid month")

    if "tier" in manifest:
        tier = manifest["tier"]
        if isinstance(tier, bool) or not isinstance(tier, int) or tier not in {1, 2, 3}:
            errors.append("tier must be an integer from 1 to 3")

    if "priority" in manifest:
        priority = manifest["priority"]
        if (
            isinstance(priority, bool)
            or not isinstance(priority, int)
            or not 0 <= priority <= 100
        ):
            errors.append("priority must be an integer from 0 to 100")

    if "techStack" in manifest:
        validate_string_array(
            manifest["techStack"], "techStack", errors, 1, 12, 1, 50
        )

    if "highlights" in manifest:
        validate_string_array(
            manifest["highlights"], "highlights", errors, 1, 6, 10, 200
        )

    if "media" in manifest:
        validate_url_group(
            manifest["media"],
            "media",
            {"thumbnail", "videoDemo"},
            errors,
        )

    if "links" in manifest:
        validate_url_group(
            manifest["links"],
            "links",
            {"live", "documentation"},
            errors,
        )

    return errors


def load_manifest(source: str) -> Any:
    if source == "-":
        return json.load(sys.stdin)

    with Path(source).open(encoding="utf-8") as manifest_file:
        return json.load(manifest_file)


def main() -> int:
    if len(sys.argv) > 2:
        print("Usage: python scripts/validate-portfolio.py [PORTFOLIO.json|-]", file=sys.stderr)
        return 2

    source = sys.argv[1] if len(sys.argv) == 2 else "PORTFOLIO.json"

    try:
        manifest = load_manifest(source)
    except FileNotFoundError:
        print(f"ERROR: file not found: {source}", file=sys.stderr)
        return 2
    except (OSError, UnicodeError) as error:
        print(f"ERROR: could not read {source}: {error}", file=sys.stderr)
        return 2
    except json.JSONDecodeError as error:
        print(
            f"INVALID: malformed JSON at line {error.lineno}, "
            f"column {error.colno}: {error.msg}",
            file=sys.stderr,
        )
        return 1

    errors = validate_manifest(manifest)
    if errors:
        print(f"INVALID: {source}", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"VALID: {source}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
