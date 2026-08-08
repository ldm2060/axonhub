import i18n from 'i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';
import { localeBackend } from './locale-backend';

i18n
  .use(LanguageDetector)
  .use(localeBackend)
  .use(initReactI18next)
  .init({
    fallbackLng: 'en',
    debug: false,
    supportedLngs: ['en', 'zh'],

    interpolation: {
      escapeValue: false, // React 已经默认转义了
      format: (value, format, lng, options) => {
        if (format === 'currency') {
          return new Intl.NumberFormat(options?.locale || lng, {
            style: 'currency',
            currency: options?.currency || 'USD',
            currencyDisplay: 'narrowSymbol',
            minimumFractionDigits: options?.minimumFractionDigits,
            maximumFractionDigits: options?.maximumFractionDigits,
          }).format(value);
        }
        return value;
      },
    },

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
