import type {
  AssessmentObjectItem,
  AssessmentPeriodCode,
  AssessmentPeriodStatus,
  AssessmentYearItem,
} from "@/types/assessment";

const PERIOD_DISPLAY_LABELS: Record<string, string> = {
  Q1: "一季度",
  Q2: "二季度",
  Q3: "三季度",
  Q4: "四季度",
  YEAR_END: "年终",
};

export function periodDisplayLabel(code: AssessmentPeriodCode, name?: string): string {
  const periodName = name?.trim();
  if (periodName) {
    return periodName;
  }
  const normalizedCode = String(code || "").trim().toUpperCase();
  return PERIOD_DISPLAY_LABELS[normalizedCode] ?? code;
}

export function formatAssessmentYearLabel(year?: Pick<AssessmentYearItem, "year">): string {
  if (!year) {
    return "-";
  }
  return `${year.year}年度`;
}

export function periodStatusText(status: AssessmentPeriodStatus): string {
  switch (status) {
    case "preparing":
      return "筹备中";
    case "active":
      return "进行中";
    case "completed":
      return "已完成";
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
