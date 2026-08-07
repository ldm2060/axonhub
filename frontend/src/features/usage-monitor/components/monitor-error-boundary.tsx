import { Component, type ErrorInfo, type ReactNode } from 'react';
import { MonitorErrorFallback } from './monitor-error-fallback';

interface Props {
  children: ReactNode;
  channelName?: string;
}

interface State {
  hasError: boolean;
}

export class MonitorErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {
    // Error already captured in state; no logger available in class component
  }

  render() {
    if (this.state.hasError) {
      return <MonitorErrorFallback channelName={this.props.channelName} onRetry={() => this.setState({ hasError: false })} />;
    }
    return this.props.children;
  }
}
