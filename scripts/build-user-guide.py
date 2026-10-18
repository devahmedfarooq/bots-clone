"""Build the repository's usage guide from its editable Markdown source."""
from pathlib import Path
import sys
from xml.sax.saxutils import escape

ROOT = Path(__file__).resolve().parents[1]
LOCAL_DEPS = ROOT / '.cache/pdf-guide-deps'
if LOCAL_DEPS.exists():
    sys.path.insert(0, str(LOCAL_DEPS))

from reportlab.lib import colors
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.pagesizes import A4
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, PageBreak, Table, TableStyle, Preformatted

OUT = ROOT / 'output/pdf/local-bots-user-guide.pdf'
OUT.parent.mkdir(parents=True, exist_ok=True)
FONT_DIR = Path('C:/Windows/Fonts')
for name, file in [('Guide','segoeui.ttf'), ('GuideBold','segoeuib.ttf')]:
    pdfmetrics.registerFont(TTFont(name, str(FONT_DIR / file)))
INK = colors.HexColor('#172b42')
ACCENT = colors.HexColor('#087e8b')
MUTED = colors.HexColor('#5c6b7a')
styles = {
    'title': ParagraphStyle('title', fontName='GuideBold', fontSize=24, leading=29, textColor=INK, spaceAfter=10),
    'h1': ParagraphStyle('heading', fontName='GuideBold', fontSize=20, leading=25, textColor=INK, spaceAfter=13),
    'h2': ParagraphStyle('subheading', fontName='GuideBold', fontSize=11, leading=15, textColor=ACCENT, spaceBefore=10, spaceAfter=5, keepWithNext=True),
    'body': ParagraphStyle('body', fontName='Guide', fontSize=9.3, leading=13.3, textColor=INK, spaceAfter=7),
    'cell': ParagraphStyle('cell', fontName='Guide', fontSize=8.7, leading=12, textColor=INK),
    'code': ParagraphStyle('code', fontName='Courier', fontSize=8.5, leading=11.7, textColor=INK, backColor=colors.HexColor('#eef3f7'), borderPadding=8, spaceBefore=3, spaceAfter=10),
    'meta': ParagraphStyle('meta', fontName='GuideBold', fontSize=9, leading=13, textColor=ACCENT, spaceAfter=12),
}

def wrapped_code(line, limit=90):
    if len(line) <= limit:
        return [line]
    quote = None
    candidates = []
    for i, char in enumerate(line[:limit]):
        if char in ("'", '"'):
            quote = None if quote == char else char if quote is None else quote
        elif char == ' ' and quote is None and i > 4:
            candidates.append(i)
    if not candidates:
        return [line]
    cut = candidates[-1]
    return [line[:cut] + ' `'] + wrapped_code('  ' + line[cut:].lstrip(), limit)

def parse_page(text):
    lines = text.strip().splitlines()
    result = []
    i = 0
    while i < len(lines):
        line = lines[i].strip()
        if not line:
            i += 1
            continue
        if line.startswith('```'):
            block = []
            i += 1
            while i < len(lines) and not lines[i].startswith('```'):
                block.extend(wrapped_code(lines[i]))
                i += 1
            result.append(Preformatted('\n'.join(block), styles['code']))
        elif line.startswith('|'):
            rows = []
            while i < len(lines) and lines[i].startswith('|'):
                rows.append([Paragraph(escape(c.strip()), styles['cell']) for c in lines[i].strip().strip('|').split('|')])
                i += 1
            table = Table(rows, colWidths=[143, 348], hAlign='LEFT')
            table.setStyle(TableStyle([
                ('BACKGROUND', (0,0),(-1,0), colors.HexColor('#dcecf0')),
                ('ROWBACKGROUNDS',(0,1),(-1,-1),[colors.HexColor('#f2f5f8'),colors.white]),
                ('VALIGN',(0,0),(-1,-1),'TOP'),
                ('LEFTPADDING',(0,0),(-1,-1),8),('RIGHTPADDING',(0,0),(-1,-1),8),
                ('TOPPADDING',(0,0),(-1,-1),6),('BOTTOMPADDING',(0,0),(-1,-1),6),
                ('LINEBELOW',(0,0),(-1,0),0.6,ACCENT),
            ]))
            result.extend([table, Spacer(1,8)])
            continue
        elif line.startswith('### '):
            result.append(Paragraph(escape(line[4:]), styles['h2']))
        elif line.startswith('## '):
            result.append(Paragraph(escape(line[3:]), styles['h1']))
        elif line.startswith('# '):
            result.append(Paragraph(escape(line[2:]), styles['title']))
        else:
            style = 'meta' if line.startswith('Windows +') else 'body'
            result.append(Paragraph(escape(line), styles[style]))
        i += 1
    return result

source = (ROOT / 'docs/user-guide.md').read_text(encoding='utf-8-sig')
pages = source.split('---PAGE---')
story = []
for i, page in enumerate(pages):
    if i:
        story.append(PageBreak())
    story.extend(parse_page(page))

def furniture(canvas, doc):
    canvas.saveState()
    width, height = A4
    canvas.setStrokeColor(ACCENT)
    canvas.setLineWidth(2)
    canvas.line(52, height-40, width-52, height-40)
    canvas.setFont('GuideBold',8)
    canvas.setFillColor(MUTED)
    canvas.drawString(52, height-31, 'LOCAL BOTS / PRACTICAL USER GUIDE')
    canvas.setFont('Guide',8)
    canvas.drawString(52,31,'Windows + RTX 5050 | Source checked 18 Oct 2026')
    canvas.drawRightString(width-52,31,f'{doc.page} / {len(pages)}')
    canvas.restoreState()

doc = SimpleDocTemplate(str(OUT), pagesize=A4, rightMargin=52, leftMargin=52,
                        topMargin=58, bottomMargin=52,
                        title='Local Bots - Practical User Guide', author='Local Bots Project')
doc.build(story,onFirstPage=furniture,onLaterPages=furniture)
from pypdf import PdfReader
reader = PdfReader(str(OUT))
if len(reader.pages) != len(pages):
    raise RuntimeError(f'Unexpected overflow: {len(reader.pages)} pages, expected {len(pages)}')
assert all(page.extract_text().strip() for page in reader.pages)
print(f'Created {OUT} ({len(reader.pages)} pages)')

import pymupdf
previews = ROOT / 'tmp/pdfs/previews'
previews.mkdir(parents=True,exist_ok=True)
with pymupdf.open(OUT) as pdf:
    for i, page in enumerate(pdf):
        page.get_pixmap(matrix=pymupdf.Matrix(1.25,1.25)).save(previews / f'page-{i+1}.png')
