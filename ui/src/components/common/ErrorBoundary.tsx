import { AlertTriangle, RotateCcw } from "lucide-react";
import { Component, type ErrorInfo, type ReactNode } from "react";

import { Button } from "../ui/Button";

type Props = {
  children: ReactNode;
  resetKey: string;
};

type State = {
  error?: Error;
};

export class ErrorBoundary extends Component<Props, State> {
  state: State = {};

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidUpdate(previous: Props) {
    if (previous.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: undefined });
    }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Route render failed", error, info.componentStack);
  }

  render() {
    if (!this.state.error) {
      return this.props.children;
    }
    return (
      <div className="route-error" role="alert">
        <AlertTriangle size={22} />
        <div>
          <h2>View failed to render</h2>
          <p>{this.state.error.message || "The admin view hit an unexpected rendering error."}</p>
        </div>
        <Button icon={<RotateCcw size={16} />} onClick={() => this.setState({ error: undefined })}>
          Retry
        </Button>
      </div>
    );
  }
}
