import sys
from pathlib import Path

ROOT_DIR = Path(__file__).resolve().parent.parent
LINE_LIMIT = 300

VALID_EXTENSIONS = {
    ".py", ".go", ".js", ".ts", ".tsx", ".jsx",
    ".java", ".cpp", ".c", ".cs", ".rs",
    ".html", ".css", ".scss", ".sql",
    ".yaml", ".yml", ".sh", ".vue"
}

IGNORE_DIRS = {
    ".git",
    ".github",
    ".vscode",
    ".idea",
    "__pycache__",
    "node_modules",
    "dist",
    "build",
    "coverage",
    ".next",
    ".turbo",
    ".cache",
    ".pytest_cache",
    ".venv",
    "venv",
    "vendor",
    "bin",
    "obj",
    "target",
    ".gomodcache",
    "gocache",
    ".gocache",
    "pkg",
}

IGNORE_FILES = {
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "go.sum",
    "poetry.lock",
}

results = []

frontend_total = 0
backend_total = 0
other_total = 0

for file in ROOT_DIR.rglob("*"):
    if not file.is_file():
        continue

    # Skip ignored directories
    if any(part.lower() in IGNORE_DIRS for part in file.parts):
        continue

    # Skip ignored files
    if file.name.lower() in IGNORE_FILES:
        continue

    # Skip unsupported extensions
    if file.suffix.lower() not in VALID_EXTENSIONS:
        continue

    # Skip test files
    name = file.name.lower()
    if (
        name.endswith("_test.go")
        or ".spec." in name
    ):
        continue

    try:
        with open(file, "r", encoding="utf-8", errors="ignore") as f:
            lines = sum(1 for _ in f)

        rel_path = str(file.relative_to(ROOT_DIR))

        if lines > LINE_LIMIT:
            results.append((lines, rel_path))

        # Categorize LOC
        rel = rel_path.replace("\\", "/")

        if rel.startswith("frontend/"):
            frontend_total += lines
        elif rel.startswith(("internal/", "cmd/")) or rel == "main.go":
            backend_total += lines
        else:
            other_total += lines

    except OSError as e:
        print(f"Error processing {file}: {e}", file=sys.stderr)
    except Exception as e:
        print(f"Unexpected error processing {file}: {e}", file=sys.stderr)

results.sort(reverse=True)

print(f"\nFiles over {LINE_LIMIT} lines (excluding tests):\n")

for lines, path in results:
    print(f"{lines:5} lines | {path}")

print("\n" + "=" * 50)
print("LOC SUMMARY (excluding tests)")
print("=" * 50)
print(f"Frontend : {frontend_total:>7,} LOC")
print(f"Backend  : {backend_total:>7,} LOC")
print(f"Other    : {other_total:>7,} LOC")
print("-" * 50)
print(f"Total    : {frontend_total + backend_total + other_total:>7,} LOC")
print("=" * 50)

print(f"\nTotal files: {len(results)}")