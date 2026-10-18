# User Guide PDF Module

SOURCE docs/user-guide.md is the editable usage manual.
BUILD scripts/build-user-guide.py:
  load UTF-8 source; split explicit page separators
  render headings, paragraphs, code and two-column tables with shared styles
  add consistent header, footer, page count and PDF metadata
  write output/pdf/local-bots-user-guide.pdf
  render each page to tmp/pdfs for visual inspection
  verify text extraction and page count before delivery
CONTENT:
  follow current code when old documentation conflicts
  cover setup, Studio, prompts, approvals, CLI, packages and operations
  distinguish generated output, downloaded files and granted workspace writes
  never inspect or embed private service tokens or personal task history
DEPENDENCIES reportlab, pypdf, pymupdf (render fallback if Poppler unavailable).
