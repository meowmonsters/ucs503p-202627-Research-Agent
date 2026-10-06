interface ToolCallBadgeProps {
  tool: string;
}

export function ToolCallBadge({ tool }: ToolCallBadgeProps) {
  const getToolName = (t: string) => {
    switch (t.toLowerCase()) {
      case "search":
        return "WEB SEARCH";
      case "financials":
        return "FINANCIAL DATA";
      case "news":
        return "LATEST NEWS";
      default:
        return t.toUpperCase();
    }
  };

  return (
    <div className="inline-flex items-center gap-2 px-3 py-1 border border-[#FF5B22]/30 bg-[#FF5B22]/5">
      <span className="text-[9px] font-black uppercase tracking-[0.2em] text-[#FF5B22]">
        {getToolName(tool)}
      </span>
    </div>
  );
}
