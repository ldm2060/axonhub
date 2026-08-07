import { Component, type ErrorInfo, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';

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

function MonitorErrorFallback({ channelName, onRetry }: { channelName?: string; onRetry: () => void }) {
  const { t } = useTranslation('usage-monitor');
  return (
    <div className='rounded-lg border border-red-200 bg-red-50 p-4'>
      <p className='text-sm font-medium text-red-800'>
        {channelName ? `${channelName}: ` : ''}
        {t('usageMonitor.status.error')}
      </p>
      <Button variant='outline' size='sm' className='mt-2' onClick={onRetry}>
        {t('usageMonitor.refreshNow')}
      </Button>
    </div>
  );
}
