from pathlib import Path
from datetime import datetime

# ============================================================
# CONFIG
# ============================================================

PROJECT_ROOT = Path(__file__).resolve().parent
DOCS_DIR = PROJECT_ROOT / "doc"

OUTPUT_FILE = PROJECT_ROOT / "doc_god.md"

# File extensions to include
INCLUDE_EXTENSIONS = {
    ".md",
    ".txt",
}

# Files to ignore
IGNORE_FILES = {
    "doc_god.md",
}

# ============================================================
# HEADER
# ============================================================

header = f"""# DOCUMENT GOD FILE
Generated: {datetime.now().isoformat()}

Purpose:
- Aggregate all documentation files from /doc
- Preserve file boundaries and locations
- Allow architecture review in AI chats
- Provide a single copy-paste context file

IMPORTANT:
- Original files inside /doc remain source of truth
- This file is generated automatically
- Do not manually edit this file
"""

sections = [header]

# ============================================================
# DISCOVER FILES
# ============================================================

doc_files = []

for path in DOCS_DIR.glob("*"):

    if not path.is_file():
        continue

    if path.name in IGNORE_FILES:
        continue

    if path.suffix.lower() not in INCLUDE_EXTENSIONS:
        continue

    doc_files.append(path)

# Sort for stable ordering
doc_files.sort()

# ============================================================
# BUILD AGGREGATE
# ============================================================

print("=" * 60)
print("BUILDING doc_god.md")
print("=" * 60)

for file_path in doc_files:

    try:
        relative = file_path.relative_to(PROJECT_ROOT)

        content = file_path.read_text(
            encoding="utf-8",
            errors="ignore"
        ).strip()

        boundary = f"""

{"=" * 100}
FILE: {relative}
ABSOLUTE: {file_path}
{"=" * 100}

{content}

"""

        sections.append(boundary)

        print(f"[OK] Added: {relative}")

    except Exception as e:
        print(f"[ERROR] {file_path}: {e}")

# ============================================================
# WRITE OUTPUT
# ============================================================

OUTPUT_FILE.write_text(
    "\n".join(sections),
    encoding="utf-8"
)

print()
print("=" * 60)
print(f"[DONE] Created: {OUTPUT_FILE}")
print("=" * 60)