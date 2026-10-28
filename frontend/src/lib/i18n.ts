import i18n from 'i18next';
import type { InterpolationOptions } from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';
import { localeBackend } from './locale-backend';

// i18next 26 dropped `format` from the InterpolationOptions type even though
// the runtime still reads options.interpolation.format; keep the currency
// formatter typed through this extension.
type InterpolationWithFormat = InterpolationOptions & {
  format?: (value: never, format?: string, lng?: string, options?: Record<string, unknown>) => string;
};

const interpolation: InterpolationWithFormat = {
  escapeValue: false, // React 已经默认转义了
  format: (value, format, lng, options) => {
    if (format === 'currency') {
      return new Intl.NumberFormat((options?.locale as string) || lng, {
        style: 'currency',
        currency: (options?.currency as string) || 'USD',
        currencyDisplay: 'narrowSymbol',
        minimumFractionDigits: options?.minimumFractionDigits as number | undefined,
        maximumFractionDigits: options?.maximumFractionDigits as number | undefined,
      }).format(value as number);
    }
    return value as string;
  },
};

i18n
  .use(LanguageDetector)
  .use(localeBackend)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    debug: false,
    supportedLngs: ['en', 'zh'],

    interpolation,

    detection: {
      order: ['localStorage', 'navigator', 'htmlTag'],
      caches: ['localStorage'],
      convertDetectedLanguage: (lng: string) => {
        const normalized = lng.toLowerCase();
        if (normalized === 'zh-cn' || normalized.startsWith('zh-')) {
          return 'zh';
        }
        return lng;
      },
    },
  });

export default i18n;
