import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { AgentThinking } from "../components/AgentThinking";
import { ReportCard } from "../components/ReportCard";
import { runAgent } from "../lib/api";
import type { AgentStep, ResearchReport } from "../types";

export default function Report() {
  // useParams already URL-decodes the segment.
  const { company = "" } = useParams<{ company: string }>();
  const navigate = useNavigate();

  const [steps, setSteps] = useState<AgentStep[]>([]);
  const [report, setReport] = useState<ResearchReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isRunning, setIsRunning] = useState(true);
  const [elapsed, setElapsed] = useState(0);
  // Bumped by "Try again" so a retry re-runs the effect below without a
  // full page reload.
  const [attempt, setAttempt] = useState(0);

  // Remembers which request was already started (company + attempt).
  // React's StrictMode runs effects twice in development, and each agent
  // run costs real LLM calls, so the second run must not start another.
  const startedFor = useRef<string | null>(null);

  useEffect(() => {
    document.title = company ? `${company} — ResearchAgent` : "ResearchAgent";
  }, [company]);

  useEffect(() => {
    const key = `${company}#${attempt}`;
    if (!company || startedFor.current === key) return;
    startedFor.current = key;

    setSteps([]);
    setReport(null);
    setError(null);
    setIsRunning(true);
    setElapsed(0);

    runAgent(company)
      .then((messages) => {
        const nextSteps: AgentStep[] = [];
        let nextReport: ResearchReport | null = null;
        let nextError: string | null = null;

        for (const m of messages) {
          if (m.type === "step" && m.step) nextSteps.push(m.step);
          else if (m.type === "report" && m.report) nextReport = m.report;
          else if (m.type === "error") nextError = m.error || "The agent reported an error.";
        }

        setSteps(nextSteps);
        setReport(nextReport);
        // An error only matters if there's no report to show.
        setError(nextReport ? null : (nextError ?? "The agent finished without producing a report."));
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : "Something went wrong.");
      })
      .finally(() => setIsRunning(false));
  }, [company, attempt]);

  // Tick a visible timer while waiting — the backend only responds once the
  // whole run is finished, so there is nothing else to show until then.
  useEffect(() => {
    if (!isRunning) return;
    const id = setInterval(() => setElapsed((s) => s + 1), 1000);
    return () => clearInterval(id);
  }, [isRunning]);

  return (
    <main className="min-h-dvh bg-black text-white font-sans selection:bg-[#FF5B22] flex flex-col">
      <nav className="flex items-center justify-between gap-4 px-4 sm:px-8 py-4 sm:py-5 border-b border-[#111] bg-black/80 backdrop-blur-md sticky top-0 z-50">
        <button onClick={() => navigate("/")} className="flex items-center gap-1 group">
          <span className="font-bold text-xs tracking-tighter text-[#FF5B22] group-hover:text-white transition-colors">
            RESEARCH
          </span>
          <span className="font-light text-xs tracking-tighter text-[#888]">AGENT</span>
        </button>
        <div className="text-[10px] font-bold uppercase tracking-widest text-[#444] truncate max-w-[55%] sm:max-w-[50%]">
          {company}
        </div>
      </nav>

      <div className="flex-1">
        {isRunning && (
          <div className="my-6 sm:my-12">
            <AgentThinking steps={steps} isRunning />
            <p className="text-center text-[11px] text-[#555] mt-2 px-4 tabular-nums break-words">
              Researching {company}… {elapsed}s — this can take up to a minute.
            </p>
            {elapsed >= 20 && (
              <p className="text-center text-[11px] text-[#444] mt-2 px-4">
                Still working. If the server was idle, the first request can take longer while it wakes up.
              </p>
            )}
          </div>
        )}

        {!isRunning && error && (
          <div className="max-w-2xl mx-4 sm:mx-auto my-10 sm:my-16 px-5 sm:px-8 py-8 sm:py-10 border border-[#333] bg-[#0A0A0A]">
            <p className="text-[#FF4444] text-[10px] font-bold uppercase tracking-[0.2em] mb-4">
              Couldn't complete the research
            </p>
            <p className="text-base sm:text-lg font-bold tracking-tight text-white mb-8 break-words">{error}</p>
            <div className="flex flex-col sm:flex-row gap-3">
              <button
                onClick={() => setAttempt((n) => n + 1)}
                className="px-8 py-3 text-center bg-[#FF5B22] text-white text-[10px] font-black uppercase tracking-widest hover:bg-[#e44a16] transition-all"
              >
                Try again
              </button>
              <button
                onClick={() => navigate("/")}
                className="px-8 py-3 border border-[#222] text-[10px] font-bold uppercase tracking-widest hover:border-[#FF5B22] hover:text-[#FF5B22] transition-all"
              >
                Back
              </button>
            </div>
          </div>
        )}

        {!isRunning && report && (
          <>
            <ReportCard report={report} />
            {steps.length > 0 && (
              <details className="max-w-3xl mx-auto px-4 pb-12 sm:pb-16">
                <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-[0.25em] text-[#555] hover:text-[#FF5B22] transition-colors">
                  Show research log ({steps.length} steps)
                </summary>
                <AgentThinking steps={steps} isRunning={false} />
              </details>
            )}
          </>
        )}
      </div>
    </main>
  );
}
