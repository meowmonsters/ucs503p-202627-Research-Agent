// Shared types mirroring the backend's current data shapes. Only "search"
// is a real tool so far — financials and news get added once the backend
// implements them, and agent/report types get added once the agent loop
// exists.

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
}

export interface NewsItem {
  title: string;
  date: string;
  source: string;
  url: string;
}

export type ToolName = "search";

export interface ToolResult {
  tool: ToolName;
  success: boolean;
  data: SearchResult[] | FinancialData | NewsItem[] | null;
  error?: string;
}
