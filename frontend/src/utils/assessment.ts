export function periodDisplayLabel(code: string, name?: string): string {
  const periodName = String(name || "").trim();
  if (periodName) {
    return periodName;
  }
  const normalizedCode = String(code || "").trim().toUpperCase();
  const mapping: Record<string, string> = {
    Q1: "第一季度",
    Q2: "第二季度",
    Q3: "第三季度",
    Q4: "第四季度",
    YEAR_END: "年终",
  };
  return mapping[normalizedCode] || normalizedCode || "-";
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

export function periodStatusText(status: string): string {
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
