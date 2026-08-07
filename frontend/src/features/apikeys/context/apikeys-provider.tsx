import { useRef, useState, type ReactNode } from 'react';
import type { ApiKey } from '../data/schema';
import { ApiKeysContext, type ApiKeyDialogType } from './apikeys-context';

export function ApiKeysProvider({ children }: { children: ReactNode }) {
  const [selectedApiKey, setSelectedApiKey] = useState<ApiKey | null>(null);
  const [selectedApiKeys, setSelectedApiKeys] = useState<ApiKey[]>([]);
  const [isDialogOpen, setIsDialogOpen] = useState<Record<ApiKeyDialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
    status: false,
    view: false,
    profiles: false,
    profileTemplates: false,
    archive: false,
    bulkDisable: false,
    bulkArchive: false,
    bulkEnable: false,
    rotate: false,
  });
  const resetRowSelectionRef = useRef<() => void>(() => {});

  const openDialog = (type: ApiKeyDialogType, apiKey?: ApiKey | ApiKey[]) => {
    if (apiKey) {
      if (Array.isArray(apiKey)) {
        setSelectedApiKeys(apiKey);
      } else {
        setSelectedApiKey(apiKey);
      }
    }
    setIsDialogOpen((prev) => ({ ...prev, [type]: true }));
  };

  const closeDialog = (type?: ApiKeyDialogType) => {
    if (type) {
      setIsDialogOpen((prev) => ({ ...prev, [type]: false }));
      if (
        type === 'delete' ||
        type === 'edit' ||
        type === 'view' ||
        type === 'archive' ||
        type === 'status' ||
        type === 'profiles' ||
        type === 'rotate'
      ) {
        setSelectedApiKey(null);
      }
      if (type === 'bulkDisable' || type === 'bulkArchive' || type === 'bulkEnable') {
        setSelectedApiKeys([]);
      }
    } else {
      setIsDialogOpen({
        create: false,
        edit: false,
        delete: false,
        status: false,
        view: false,
        profiles: false,
        profileTemplates: false,
        archive: false,
        bulkDisable: false,
        bulkArchive: false,
        bulkEnable: false,
        rotate: false,
      });
      setSelectedApiKey(null);
      setSelectedApiKeys([]);
    }
  };

  return (
    <ApiKeysContext.Provider
      value={{
        selectedApiKey,
        setSelectedApiKey,
        selectedApiKeys,
        setSelectedApiKeys,
        isDialogOpen,
        openDialog,
        closeDialog,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn;
        },
      }}
    >
      {children}
    </ApiKeysContext.Provider>
  );
}

export default ApiKeysProvider;
