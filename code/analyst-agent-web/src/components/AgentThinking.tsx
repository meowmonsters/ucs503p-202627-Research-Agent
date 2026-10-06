import { useEffect, useRef } from "react";
import type { AgentStep } from "../types";
import { ToolCallBadge } from "./ToolCallBadge";

interface AgentThinkingProps {
  steps: AgentStep[];
  isRunning: boolean;
}

export function AgentThinking({ steps, isRunning }: AgentThinkingProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      if (scrollRef.current) {
        scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
      }
    });
    return () => cancelAnimationFrame(frame);
  }, [steps.length]);

  return (
    <div className="w-full max-w-3xl mx-auto px-4 py-12 font-sans">
      <div className="bg-[#050505] border border-[#111] p-8">
        <div className="flex items-center gap-3 mb-8 border-b border-[#111] pb-6">
          <span
            className={`w-2 h-2 rounded-full ${isRunning ? "bg-[#FF5B22] animate-pulse" : "bg-white/20"}`}
          />
          <h2 className="text-[11px] font-black uppercase tracking-[0.3em] text-[#FF5B22]">
            {isRunning ? "Researching" : "Finished"}
          </h2>
        </div>

        <div ref={scrollRef} className="space-y-6 max-h-[60vh] overflow-y-auto pr-2">
          {steps.map((step) => (
            <div key={step.id} className="flex items-start gap-6">
              <div className="text-[9px] font-bold text-[#333] pt-1 min-w-[56px] tabular-nums">
                {new Date(step.timestamp).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                  second: "2-digit",
                  hour12: false,
                })}
              </div>
              <div className="flex-1 border-l border-[#111] pl-6">
                <p className="text-sm text-[#AAA] leading-relaxed">{step.content}</p>
                {step.toolCall && (
                  <div className="mt-3">
                    <ToolCallBadge tool={step.toolCall.tool} />
                  </div>
                )}
              </div>
            </div>
          ))}

          {isRunning && (
            <div className="flex items-center gap-6 opacity-30">
              <div className="text-[9px] font-bold text-[#222] min-w-[56px]">--:--:--</div>
              <div className="flex-1 border-l border-[#111] pl-6">
                <span className="text-[9px] font-black text-[#333] uppercase tracking-[0.3em]">
                  Waiting...
                </span>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
