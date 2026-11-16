import { Component, type ErrorInfo, type ReactNode } from "react";

interface State {
  failed: boolean;
}

/**
 * Last line of defence: if rendering ever throws (say, an unexpected report
 * shape), show a recovery screen instead of a blank black page.
 */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("UI error:", error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <main className="min-h-dvh bg-black text-white flex items-center justify-center px-4">
        <div className="max-w-md w-full border border-[#333] bg-[#0A0A0A] p-6 sm:p-10 text-center">
          <p className="text-[#FF4444] text-[10px] font-bold uppercase tracking-[0.2em] mb-4">
            Something went wrong
          </p>
          <p className="text-sm text-[#AAA] mb-8">
            The page hit an unexpected error. Going back to the start usually fixes it.
          </p>
          <a
            href="/"
            className="inline-block px-8 py-3 bg-[#FF5B22] text-[10px] font-black uppercase tracking-widest hover:bg-[#e44a16] transition-all"
          >
            Back to start
          </a>
        </div>
      </main>
    );
  }
}
