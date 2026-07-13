export type RequestDetailTab = 'overview' | 'request' | 'response' | 'executions';

export const DEFAULT_REQUEST_DETAIL_TAB: RequestDetailTab = 'overview';

export function nextExpandedExecution(currentId: string | null, clickedId: string): string | null {
  return currentId === clickedId ? null : clickedId;
}
