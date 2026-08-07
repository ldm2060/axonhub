import { createContext, useContext } from 'react';
import type { ApiKey } from '../data/schema';

export type ApiKeyDialogType =
  | 'create'
  | 'edit'
  | 'delete'
  | 'status'
  | 'view'
  | 'profiles'
  | 'profileTemplates'
  | 'archive'
  | 'bulkDisable'
  | 'bulkArchive'
  | 'bulkEnable'
  | 'rotate';

interface ApiKeysContextType {
  selectedApiKey: ApiKey | null;
  setSelectedApiKey: (apiKey: ApiKey | null) => void;
  selectedApiKeys: ApiKey[];
  setSelectedApiKeys: (apiKeys: ApiKey[]) => void;
  isDialogOpen: Record<ApiKeyDialogType, boolean>;
  openDialog: (type: ApiKeyDialogType, apiKey?: ApiKey | ApiKey[]) => void;
  closeDialog: (type?: ApiKeyDialogType) => void;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
}

export const ApiKeysContext = createContext<ApiKeysContextType | undefined>(undefined);

export function useApiKeysContext() {
  const context = useContext(ApiKeysContext);
  if (context === undefined) {
    throw new Error('useApiKeysContext must be used within a ApiKeysProvider');
  }
  return context;
}
