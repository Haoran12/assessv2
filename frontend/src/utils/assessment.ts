import type { AssessmentObjectItem } from "@/types/assessment";

export const PERIOD_OPTIONS = ["Q1", "Q2", "Q3", "Q4", "YEAR_END"] as const;

export function formatTimestamp(timestamp?: number): string {
  if (!timestamp) {
    return "-";
  }
  const date = new Date(timestamp * 1000);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString();
}

export function toObjectNameMap(items: AssessmentObjectItem[]): Record<number, string> {
  return items.reduce<Record<number, string>>((acc, item) => {
    acc[item.id] = item.objectName;
    return acc;
  }, {});
}

export function formatFloat(value: number, digits = 2): string {
  if (!Number.isFinite(value)) {
    return "-";
  }
  return value.toFixed(digits);
}
