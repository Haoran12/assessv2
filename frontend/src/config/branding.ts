const fallbackBrandName = "集团企业";
const fallbackAppTitle = "集团企业考核系统";

function normalize(value: string | undefined, fallback: string): string {
  const text = value?.trim();
  return text && text.length > 0 ? text : fallback;
}

export const appBrandName = normalize(import.meta.env.VITE_APP_BRAND_NAME, fallbackBrandName);
export const appTitle = normalize(import.meta.env.VITE_APP_TITLE, fallbackAppTitle);
