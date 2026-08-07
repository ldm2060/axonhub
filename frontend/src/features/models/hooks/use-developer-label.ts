import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

export function useDeveloperLabel() {
  const { t, i18n } = useTranslation();

  return useCallback(
    (developer: string) => {
      const key = `models.developers.${developer}`;
      return i18n.exists(key) ? t(key) : developer;
    },
    [t, i18n]
  );
}
