from pathlib import Path

ROOT_DIR = Path(".")
LINE_LIMIT = 800

VALID_EXTENSIONS = {
    ".py", ".go", ".js", ".ts", ".tsx", ".jsx",
    ".java", ".cpp", ".c", ".cs", ".rs",
    ".html", ".css", ".scss", ".sql", ".json",
    ".yaml", ".yml", ".sh"
}

# folders to ignore anywhere in path
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

# files to ignore
IGNORE_FILES = {
    "package-lock.json",
    "yarn.lock",
    "pnpm-lock.yaml",
    "go.sum",
    "poetry.lock",
}

results = []

for file in ROOT_DIR.rglob("*"):
    if not file.is_file():
        continue

    # skip ignored dirs
    if any(part.lower() in IGNORE_DIRS for part in file.parts):
        continue

    # skip ignored files
    if file.name.lower() in IGNORE_FILES:
        continue

    # skip unsupported extensions
    if file.suffix.lower() not in VALID_EXTENSIONS:
        continue

    try:
        # fast pre-filter using bytes
        if file.stat().st_size < 12000:
            continue

        with open(file, "r", encoding="utf-8", errors="ignore") as f:
            lines = sum(1 for _ in f)

        if lines > LINE_LIMIT:
            results.append((lines, str(file)))

    except:
        pass

results.sort(reverse=True)

print(f"\nFiles over {LINE_LIMIT} lines (project files only):\n")

for lines, path in results:
    print(f"{lines:5} lines | {path}")

print(f"\nTotal: {len(results)} file(s)")