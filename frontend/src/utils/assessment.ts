import type { AssessmentObjectItem, AssessmentPeriodStatus, AssessmentYearItem } from "@/types/assessment";

export const PERIOD_OPTIONS = ["Q1", "Q2", "Q3", "Q4", "YEAR_END"] as const;

export function formatAssessmentYearLabel(year?: Pick<AssessmentYearItem, "year" | "yearName">): string {
  if (!year) {
    return "-";
  }
  const yearName = year.yearName?.trim();
  if (!yearName) {
    return `${year.year}年度`;
  }
  if (yearName === String(year.year)) {
    return `${yearName}年度`;
  }
  return yearName;
}

export function periodStatusText(status: AssessmentPeriodStatus): string {
  switch (status) {
    case "not_started":
      return "未开始";
    case "active":
      return "进行中";
    case "ended":
      return "已结束";
    case "locked":
      return "已锁定";
    default:
      return status;
  }
}

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
