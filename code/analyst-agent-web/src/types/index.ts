// Shared types mirroring the backend's current data shapes.

export interface SearchResult {
  title: string;
  url: string;
  snippet: string;
}

export interface FinancialData {
  ticker: string;
  companyName: string;
  price: number | null;
  change: number | null;
  changePercent: number | null;
  currency: string;
  marketCap: string | null;
  peRatio: number | null;
  dividendYield: number | null;
  revenue: string | null;
}

export interface NewsItem {
  title: string;
  date: string;
  source: string;
  url: string;
}

export type ToolName = "search" | "financials" | "news";

export interface ToolResult {
  tool: ToolName;
  success: boolean;
  data: SearchResult[] | FinancialData | NewsItem[] | null;
  error?: string;
}

// --- Agent types ---

export interface ToolCall {
  tool: ToolName;
  args: Record<string, string>;
  reasoning: string;
}

export type AgentStepType =
  | "plan"
  | "tool_call"
  | "tool_result"
  | "reflection"
  | "synthesis"
  | "error";

export interface AgentStep {
  id: number;
  type: AgentStepType;
  content: string;
  toolCall?: ToolCall;
  toolResult?: ToolResult;
  timestamp: number;
}

// --- Report types ---

export interface ReportSection {
  title: string;
  icon: string;
  content: string;
}

export interface ResearchReport {
  company: string;
  generatedAt: string;
  sections: ReportSection[];
  overallSentiment: "Bullish" | "Neutral" | "Bearish";
}

// --- Streaming / message types ---

export type AgentMessageType = "step" | "report" | "error" | "done";

export interface AgentMessage {
  type: AgentMessageType;
  step?: AgentStep;
  report?: ResearchReport;
  error?: string;
}
