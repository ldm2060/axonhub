'use client';

import { createContext, useContext, type RefObject } from 'react';
import type { FileUIPart } from 'ai';

export type AttachmentsContext = {
  files: (FileUIPart & { id: string })[];
  add: (files: File[] | FileList) => void;
  remove: (id: string) => void;
  clear: () => void;
  openFileDialog: () => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
};

export const ProviderAttachmentsContext = createContext<AttachmentsContext | null>(null);
export const LocalAttachmentsContext = createContext<AttachmentsContext | null>(null);

export const useOptionalProviderAttachments = () => useContext(ProviderAttachmentsContext);

export const usePromptInputAttachments = () => {
  // Dual-mode: prefer provider if present, otherwise use local
  const provider = useOptionalProviderAttachments();
  const local = useContext(LocalAttachmentsContext);
  const context = provider ?? local;
  if (!context) {
    throw new Error('usePromptInputAttachments must be used within a PromptInput or PromptInputProvider');
  }
  return context;
};
