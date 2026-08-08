import type { BackendModule, ReadCallback } from 'i18next';

type LocaleModule = {
  default: Record<string, unknown>;
};

type LocaleLoader = () => Promise<LocaleModule>;

const localeLoaders: Record<string, Record<string, LocaleLoader>> = {
  en: import.meta.glob<LocaleModule>('../locales/en/*.json'),
  zh: import.meta.glob<LocaleModule>('../locales/zh-CN/*.json'),
};

const localePromises = new Map<string, Promise<Record<string, unknown>>>();

function normalizeLanguage(language: string) {
  return language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

function loadLocale(language: string) {
  const normalizedLanguage = normalizeLanguage(language);
  const cachedPromise = localePromises.get(normalizedLanguage);
  if (cachedPromise) {
    return cachedPromise;
  }

  const promise = Promise.all(Object.values(localeLoaders[normalizedLanguage]).map((loadModule) => loadModule())).then((modules) =>
    Object.assign({}, ...modules.map((module) => module.default))
  );
  localePromises.set(normalizedLanguage, promise);
  return promise;
}

export const localeBackend: BackendModule = {
  type: 'backend',
  init() {},
  read(language: string, _namespace: string, callback: ReadCallback) {
    void loadLocale(language).then(
      (translations) => callback(null, translations),
      (error: unknown) => callback(error instanceof Error ? error : new Error(String(error)), null)
    );
  },
};
