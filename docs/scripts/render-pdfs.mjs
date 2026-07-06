import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { execFileSync } from "node:child_process";

const root = resolve(import.meta.dirname, "../..");
const outDir = resolve(root, "docs/assets/pdf");
const tempDir = resolve(root, "docs/assets/pdf/html");

const docs = [
  {
    source: "docs/demo-flow.md",
    html: "demo-flow.html",
    pdf: "demo-flow.pdf",
    title: "Demo Flow",
  },
  {
    source: "docs/architecture.md",
    html: "architecture.html",
    pdf: "architecture.pdf",
    title: "Architecture",
  },
];

mkdirSync(tempDir, { recursive: true });
mkdirSync(outDir, { recursive: true });

function escapeHtml(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function inlineMarkdown(value) {
  return escapeHtml(value)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_, alt, src) => {
      const fixedSrc = src.startsWith("./") ? `../.${src}` : src;
      return `<img src="${fixedSrc}" alt="${alt}" />`;
    })
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
}

function renderMarkdown(markdown) {
  const lines = markdown.split(/\r?\n/);
  const parts = [];
  let paragraph = [];
  let list = [];
  let code = [];
  let inCode = false;
  let table = [];

  function flushParagraph() {
    if (paragraph.length === 0) return;
    parts.push(`<p>${inlineMarkdown(paragraph.join(" "))}</p>`);
    paragraph = [];
  }

  function flushList() {
    if (list.length === 0) return;
    parts.push(`<ul>${list.map((item) => `<li>${inlineMarkdown(item)}</li>`).join("")}</ul>`);
    list = [];
  }

  function flushCode() {
    if (code.length === 0) return;
    parts.push(`<pre><code>${escapeHtml(code.join("\n"))}</code></pre>`);
    code = [];
  }

  function flushTable() {
    if (table.length === 0) return;
    const rows = table
      .filter((row) => !/^\|\s*-+/.test(row))
      .map((row) => row.split("|").slice(1, -1).map((cell) => inlineMarkdown(cell.trim())));
    const [head, ...body] = rows;
    if (head) {
      parts.push(
        `<table><thead><tr>${head.map((cell) => `<th>${cell}</th>`).join("")}</tr></thead><tbody>${body
          .map((row) => `<tr>${row.map((cell) => `<td>${cell}</td>`).join("")}</tr>`)
          .join("")}</tbody></table>`,
      );
    }
    table = [];
  }

  for (const line of lines) {
    const trimmed = line.trim();

    if (trimmed.startsWith("```")) {
      flushParagraph();
      flushList();
      flushTable();
      if (inCode) {
        inCode = false;
        flushCode();
      } else {
        inCode = true;
      }
      continue;
    }

    if (inCode) {
      code.push(line);
      continue;
    }

    if (trimmed === "---") {
      flushParagraph();
      flushList();
      flushTable();
      parts.push("<hr />");
      continue;
    }

    if (trimmed === "") {
      flushParagraph();
      flushList();
      flushTable();
      continue;
    }

    if (trimmed.startsWith("|")) {
      flushParagraph();
      flushList();
      table.push(trimmed);
      continue;
    }

    const headingMatch = trimmed.match(/^<h([1-3])[^>]*>(.*)<\/h\1>$/);
    if (headingMatch) {
      flushParagraph();
      flushList();
      flushTable();
      parts.push(`<h${headingMatch[1]}>${inlineMarkdown(headingMatch[2])}</h${headingMatch[1]}>`);
      continue;
    }

    if (trimmed.startsWith("* ")) {
      flushParagraph();
      flushTable();
      list.push(trimmed.slice(2));
      continue;
    }

    paragraph.push(trimmed);
  }

  flushParagraph();
  flushList();
  flushCode();
  flushTable();
  return parts.join("\n");
}

function renderHtml(title, body) {
  return `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>${escapeHtml(title)}</title>
  <style>
    @page { margin: 16mm 14mm; }
    body {
      color: #171b26;
      font-family: Arial, sans-serif;
      font-size: 12px;
      line-height: 1.45;
    }
    h1, h2, h3 { color: #202534; page-break-after: avoid; }
    h1 { font-size: 26px; text-align: center; margin: 0 0 18px; }
    h2 { font-size: 20px; text-align: center; margin: 22px 0 12px; }
    h3 { font-size: 15px; text-align: center; margin: 18px 0 10px; }
    p { margin: 8px 0; }
    ul { margin: 8px 0 12px 22px; padding: 0; }
    li { margin: 4px 0; }
    a { color: #0077ff; text-decoration: none; }
    code {
      background: #f1f3f7;
      border-radius: 4px;
      font-family: "Courier New", monospace;
      padding: 1px 4px;
    }
    pre {
      background: #f6f7f9;
      border: 1px solid #d8dde8;
      border-radius: 8px;
      overflow: hidden;
      padding: 12px;
      white-space: pre-wrap;
      page-break-inside: avoid;
    }
    pre code { background: transparent; padding: 0; }
    table {
      border-collapse: collapse;
      margin: 10px 0 16px;
      width: 100%;
      page-break-inside: avoid;
    }
    th, td {
      border: 1px solid #d8dde8;
      padding: 8px;
      text-align: left;
      vertical-align: top;
    }
    th { background: #f1f3f7; }
    img {
      border: 1px solid #d8dde8;
      border-radius: 8px;
      display: block;
      margin: 10px auto 18px;
      max-height: 720px;
      max-width: 100%;
      page-break-inside: avoid;
    }
    hr {
      border: 0;
      border-top: 1px solid #d8dde8;
      margin: 18px 0;
    }
  </style>
</head>
<body>
${body}
</body>
</html>`;
}

for (const doc of docs) {
  const markdown = readFileSync(resolve(root, doc.source), "utf8");
  const htmlPath = resolve(tempDir, doc.html);
  const pdfPath = resolve(outDir, doc.pdf);
  mkdirSync(dirname(htmlPath), { recursive: true });
  writeFileSync(htmlPath, renderHtml(doc.title, renderMarkdown(markdown)));
  execFileSync("google-chrome", [
    "--headless",
    "--disable-gpu",
    "--no-sandbox",
    "--print-to-pdf-no-header",
    `--print-to-pdf=${pdfPath}`,
    `file://${htmlPath}`,
  ]);
  console.log(pdfPath);
}
