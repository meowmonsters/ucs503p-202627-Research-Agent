import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { ResearchReport } from "../types";

interface ReportCardProps {
  report: ResearchReport;
}

const SENTIMENT_STYLES: Record<string, string> = {
  Bullish: "text-[#00FF88] border-[#00FF88]/30",
  Neutral: "text-[#FFCC00] border-[#FFCC00]/30",
  Bearish: "text-[#FF4444] border-[#FF4444]/30",
};

// Section content arrives as "- point" lines; strip the markers and blanks.
function toBullets(content: string): string[] {
  return content
    .split("\n")
    .map((line) => line.replace(/^\s*(?:[-*•]|\d+\.)\s*/, "").trim())
    .filter((line) => line.length > 0);
}

export function ReportCard({ report }: ReportCardProps) {
  const navigate = useNavigate();
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");

  const generated = new Date(report.generatedAt);
  const dateLabel = Number.isNaN(generated.getTime()) ? "" : generated.toLocaleDateString();

  const handleCopy = async () => {
    const body = report.sections
      .map((s) => `${s.title.toUpperCase()}\n\n${s.content}`)
      .join("\n\n---\n\n");
    const text =
      `RESEARCH REPORT: ${report.company.toUpperCase()}\n` +
      `Generated: ${dateLabel}\nSentiment: ${report.overallSentiment}\n\n---\n\n${body}\n\n` +
      `Informational only — not financial advice.`;

    try {
      await navigator.clipboard.writeText(text);
      setCopyState("copied");
    } catch {
      // Clipboard access can be denied (e.g. non-secure origin).
      setCopyState("failed");
    }
    setTimeout(() => setCopyState("idle"), 2000);
  };

  const copyLabel = { idle: "Copy to clipboard", copied: "Copied", failed: "Copy failed" }[copyState];

  return (
    <div className="w-full max-w-7xl mx-auto px-4 py-8 sm:py-12 animate-fade-in font-sans">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-8 sm:mb-12 border-b border-[#111] pb-8 sm:pb-10">
        <div className="space-y-3 min-w-0">
          <div className="flex items-center gap-3">
            <span className="h-px w-8 bg-[#FF5B22]" />
            <span className="text-[10px] font-bold uppercase tracking-[0.25em] text-[#FF5B22]">
              Research Brief
            </span>
          </div>
          <h1 className="text-3xl sm:text-4xl md:text-5xl font-extrabold tracking-tighter uppercase break-words">
            {report.company}
          </h1>
        </div>

        <div className="flex flex-wrap items-center gap-x-8 gap-y-4">
          {dateLabel && (
            <div>
              <p className="text-[9px] font-bold uppercase tracking-widest text-[#444] mb-1.5">Date</p>
              <p className="text-xs font-medium text-white/40">{dateLabel}</p>
            </div>
          )}
          <div className="min-w-[120px]">
            <p className="text-[9px] font-bold uppercase tracking-widest text-[#444] mb-1.5">Outlook</p>
            <div
              className={`text-[10px] font-black uppercase tracking-[0.2em] border px-4 py-1.5 text-center ${
                SENTIMENT_STYLES[report.overallSentiment] ?? SENTIMENT_STYLES.Neutral
              }`}
            >
              {report.overallSentiment}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-px bg-[#111] border border-[#111]">
        {report.sections.map((section, index) => (
          <div key={index} className="bg-black p-6 sm:p-8 md:p-10 hover:bg-[#050505] transition-colors group">
            <p className="text-[9px] font-black uppercase tracking-[0.25em] text-[#FF5B22]/60 mb-6 sm:mb-8 flex items-center gap-2">
              <span className="text-[#333] font-light">0{index + 1}</span>
              {section.title}
            </p>
            <ul className="space-y-4">
              {toBullets(section.content).map((item, i) => (
                <li key={i} className="flex items-start gap-3">
                  <span className="w-1.5 h-1.5 rounded-full bg-[#FF5B22]/40 mt-1.5 flex-shrink-0 group-hover:bg-[#FF5B22] transition-colors" />
                  <span className="text-sm text-white/60 sm:text-white/50 leading-relaxed break-words min-w-0 group-hover:text-white/80 transition-colors">
                    {item}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      <div className="flex flex-col md:flex-row items-center justify-center gap-4 mt-10 sm:mt-16">
        <button
          onClick={handleCopy}
          className="w-full md:w-auto px-10 py-4 border border-[#222] text-[10px] font-bold uppercase tracking-[0.25em] hover:border-[#FF5B22] hover:text-[#FF5B22] transition-all"
        >
          {copyLabel}
        </button>
        <button
          onClick={() => navigate("/")}
          className="w-full md:w-auto px-10 py-4 bg-[#FF5B22] text-white text-[10px] font-bold uppercase tracking-[0.25em] hover:bg-[#e44a16] transition-all"
        >
          New analysis
        </button>
      </div>

      <p className="mt-10 text-center text-[10px] text-[#444] leading-relaxed max-w-2xl mx-auto">
        Informational research aid only — not financial advice. Figures come from public
        sources that may be delayed, incomplete, or unavailable; sections marked "not
        available" could not be supported by retrieved data.
      </p>
    </div>
  );
}
