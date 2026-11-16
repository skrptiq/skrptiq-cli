---
type: document
id: summary-guide
title: Summary Guide
description: "User guide for getting the best results from the book summary pipeline"
tags: [Production, Learning]
connections:
  - target: book-summary-pipeline
    type: references
---

## Getting Started

### Preparing Your Input

The pipeline accepts book content as plain text. How you provide it depends on your source format:

**PDF:** Use your PDF reader's "Select All" and copy, or use a PDF-to-text tool. Most modern PDFs copy cleanly. If the PDF is a scanned image, run OCR first (your operating system's built-in tools or a free service like Adobe's online OCR).

**EPUB/MOBI:** Open in Caliber or your preferred ebook reader, select all text, and copy. Alternatively, convert to PDF first.

**Physical book:** Use a scanning app on your phone (e.g. Adobe Scan, Microsoft Lens) to capture pages, then OCR the result.

**Audiobook:** Use the book's accompanying PDF or ebook version if available. Transcription from audio is not recommended as it introduces too many errors.

### Choosing Summary Depth

- **Brief:** Quick revision aid. Produces an executive summary and key takeaways only. Good for books you've already read and need a refresher on.
- **Standard:** Balanced summary for seminar prep or essay research. Covers all sections with concise treatment. This is the default.
- **Detailed:** Deep analysis for dissertation research or critical essays. Includes extended thematic analysis, methodology discussion, and additional quotes.

### Using Focus Areas

Set `focus_areas` to steer the summary towards specific themes relevant to your coursework. For example:

- For a psychology essay on cognitive biases: `"cognitive biases, heuristics, decision-making errors"`
- For a history module on economic causes: `"economic factors, trade policy, class structure"`

The pipeline will give proportionally more attention to chapters and sections that address your focus areas.

## Tips for Better Results

1. **Provide the full text** when possible. The pipeline produces better thematic analysis when it can see the whole book.
2. **Set the subject area.** This helps the critical evaluation section contextualise the book within its field.
3. **Process difficult books on "detailed"** and easier ones on "brief." Match the depth to how much you need to engage with the material.
4. **Review the critical evaluation** before writing essays. It will flag weaknesses you might want to address in your own argument.
