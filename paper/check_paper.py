import os
import subprocess
import shutil
import sys
import re

def check_latex_environment():
    """Check if pdflatex or latexmk is available in the system PATH."""
    pdflatex_path = shutil.which("pdflatex")
    if not pdflatex_path:
        print("[ERROR] 'pdflatex' compiler not found in system PATH.")
        print("Please install MiKTeX (https://miktex.org/download) or TeX Live.")
        return False
    print(f"[OK] Compiler found: {pdflatex_path}")
    return True

def compile_and_validate(paper_dir, tex_file="main.tex"):
    """Compile main.tex and parse compilation log for errors and warnings."""
    tex_path = os.path.join(paper_dir, tex_file)
    if not os.path.exists(tex_path):
        print(f"[ERROR] Target LaTeX file not found at: {tex_path}")
        return False

    print(f"\n--- Compiling {tex_file} in {paper_dir} ---")
    
    # 1. First pdflatex pass
    cmd1 = ["pdflatex", "-interaction=nonstopmode", "-file-line-error", tex_file]
    res1 = subprocess.run(cmd1, cwd=paper_dir, capture_output=True, text=True)
    
    # 2. BibTeX pass (if bib file referenced)
    base_name = os.path.splitext(tex_file)[0]
    bib_path = os.path.join(paper_dir, "references.bib")
    if os.path.exists(bib_path):
        print("[INFO] Running BibTeX for citations...")
        subprocess.run(["bibtex", base_name], cwd=paper_dir, capture_output=True, text=True)

    # 3. Two follow-up passes to resolve cross-references
    print("[INFO] Resolving cross-references...")
    subprocess.run(cmd1, cwd=paper_dir, capture_output=True, text=True)
    subprocess.run(cmd1, cwd=paper_dir, capture_output=True, text=True)

    pdf_file = os.path.join(paper_dir, f"{base_name}.pdf")
    log_file = os.path.join(paper_dir, f"{base_name}.log")

    print("\n--- Diagnostic Results ---")
    
    if os.path.exists(pdf_file):
        print(f"[SUCCESS] PDF generated successfully: {pdf_file} ({os.path.getsize(pdf_file)} bytes)")
    else:
        print("[FAILURE] PDF generation failed!")

    if not os.path.exists(log_file):
        print("[WARNING] Could not find log file to parse.")
        return False

    # Log parsing
    errors = []
    warnings = []
    missing_cites = []

    with open(log_file, "r", encoding="utf-8", errors="ignore") as f:
        for line in f:
            if line.startswith("!") or "LaTeX Error" in line:
                errors.append(line.strip())
            elif "LaTeX Warning:" in line:
                warnings.append(line.strip())
            elif "Citation" in line and "undefined" in line:
                missing_cites.append(line.strip())

    if errors:
        print(f"\n[ERRORS FOUND: {len(errors)}]")
        for err in errors[:10]: # Print top 10
            print(f"  ❌ {err}")
    else:
        print("[OK] 0 Compilation Errors.")

    if missing_cites:
        print(f"\n[UNDEFINED CITATIONS: {len(missing_cites)}]")
        for cite in missing_cites:
            print(f"  ⚠️  {cite}")

    if warnings:
        print(f"\n[WARNINGS: {len(warnings)}]")
        for warn in warnings[:5]:
            print(f"  ⚠️  {warn}")

    return len(errors) == 0 and os.path.exists(pdf_file)

if __name__ == "__main__":
    script_dir = os.path.dirname(os.path.abspath(__file__))
    if not check_latex_environment():
        sys.exit(1)
    
    success = compile_and_validate(script_dir)
    sys.exit(0 if success else 1)
