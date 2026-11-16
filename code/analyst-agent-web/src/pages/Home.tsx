import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { warmUpBackend } from "../lib/api";

const EXAMPLES = ["TCS", "INFY.NS", "AAPL", "RELIANCE.NS"];

export default function Home() {
  const [company, setCompany] = useState("");
  const navigate = useNavigate();

  useEffect(() => {
    document.title = "ResearchAgent — AI investment briefs";
    // A sleeping host takes a while to start; begin waking it now.
    warmUpBackend();
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = company.trim();
    if (trimmed) navigate(`/report/${encodeURIComponent(trimmed)}`);
  };

  return (
<<<<<<< HEAD
    <main className="min-h-screen bg-black flex flex-col text-white font-sans bg-[url('/src/imgs/')] bg-cover bg-center">
      <nav className="flex items-center justify-between px-8 py-6 border-b border-[#111]">
=======
    <main className="min-h-screen bg-black flex flex-col text-white font-sans">
      <nav className="flex items-center justify-between px-4 sm:px-8 py-5 sm:py-6 border-b border-[#111]">
>>>>>>> 675dcb8 (feat: add report page and connect to backend)
        <div className="flex items-center gap-1">
          <span className="font-bold text-lg tracking-tighter text-[#FF5B22]">RESEARCH</span>
          <span className="font-light text-lg tracking-tighter text-[#888]">AGENT</span>
        </div>
      </nav>

<<<<<<< HEAD
      <section className="flex-1 flex flex-col items-center justify-center max-w-5xl mx-auto px-6 text-center py-24 bg-gray-900/30 backdrop-blur-sm">
=======
      <section className="flex-1 w-full flex flex-col items-center justify-center max-w-5xl mx-auto px-4 sm:px-6 text-center py-12 sm:py-24">
>>>>>>> 675dcb8 (feat: add report page and connect to backend)
        <div className="inline-block px-3 py-1 mb-8 border border-[#222] rounded-full">
          <span className="text-[10px] font-bold uppercase tracking-[0.25em] text-[#FF5B22]">
            Autonomous Pipeline
          </span>
        </div>

        <h1 className="text-3xl min-[400px]:text-4xl sm:text-5xl md:text-7xl font-bold leading-[1] tracking-tighter mb-6 sm:mb-8">
          Enterprise Grade
          <br />
          <span className="text-[#FF5B22]">Data Synthesis</span>
        </h1>

        <p className="text-sm md:text-base text-[#666] max-w-xl leading-relaxed mb-8 sm:mb-12">
          Analyze any company or stock ticker instantly with live web
          intelligence and financial data.
        </p>

        <form onSubmit={handleSubmit} className="w-full max-w-xl group relative z-10">
          <div className="flex flex-col sm:flex-row bg-[#0A0A0A] border border-[#222] group-focus-within:border-[#FF5B22]/50 transition-all duration-300">
            <input
              type="text"
              placeholder="COMPANY OR TICKER…"
              value={company}
              onChange={(e) => setCompany(e.target.value)}
              maxLength={100}
              aria-label="Company name or ticker"
              className="flex-1 min-w-0 bg-transparent px-5 sm:px-8 py-4 sm:py-5 text-base sm:text-xs font-medium tracking-wider sm:tracking-widest uppercase outline-none"
              autoComplete="off"
              autoCapitalize="off"
              spellCheck={false}
            />
            <button
              type="submit"
              disabled={!company.trim()}
              className="bg-[#FF5B22] px-10 py-4 sm:py-5 text-[10px] font-black uppercase tracking-[0.2em] hover:bg-[#e44a16] disabled:opacity-30 transition-all"
            >
              Analyze
            </button>
          </div>

          <div className="flex flex-wrap items-center justify-center gap-3 mt-6">
            <span className="text-[9px] font-bold uppercase tracking-widest text-[#444]">
              Top Queries
            </span>
            {EXAMPLES.map((ex) => (
              <button
                key={ex}
                type="button"
                onClick={() => navigate(`/report/${encodeURIComponent(ex)}`)}
                className="text-[9px] font-bold uppercase tracking-widest px-3 py-2 sm:py-1 border border-[#111] hover:border-[#FF5B22] hover:text-[#FF5B22] transition-all"
              >
                {ex}
              </button>
            ))}
          </div>
        </form>
      </section>

      <footer className="px-4 sm:px-8 py-6 border-t border-[#111] text-center">
        <span className="text-[9px] font-bold uppercase tracking-widest text-[#333]">
          RESEARCHAGENT © 2026
        </span>
      </footer>
    </main>
  );
}
