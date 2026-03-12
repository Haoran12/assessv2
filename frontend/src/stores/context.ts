import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { listAssessmentYears } from "@/api/assessment";
import type { AssessmentPeriodCode, AssessmentYearItem } from "@/types/assessment";

const YEAR_KEY = "assessv2_context_year_id";
const PERIOD_KEY = "assessv2_context_period_code";
const DEFAULT_PERIOD: AssessmentPeriodCode = "Q1";
const PERIOD_SET = new Set<AssessmentPeriodCode>(["Q1", "Q2", "Q3", "Q4", "YEAR_END"]);

function readStoredYearId(): number | undefined {
  const value = localStorage.getItem(YEAR_KEY);
  if (!value) {
    return undefined;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

function readStoredPeriodCode(): AssessmentPeriodCode {
  const value = localStorage.getItem(PERIOD_KEY) as AssessmentPeriodCode | null;
  if (value && PERIOD_SET.has(value)) {
    return value;
  }
  return DEFAULT_PERIOD;
}

export const useContextStore = defineStore("context", () => {
  const years = ref<AssessmentYearItem[]>([]);
  const yearId = ref<number | undefined>(readStoredYearId());
  const periodCode = ref<AssessmentPeriodCode>(readStoredPeriodCode());
  const loadingYears = ref(false);
  const initialized = ref(false);

  const currentYear = computed(() => years.value.find((item) => item.id === yearId.value));

  function setYear(value: number | undefined): void {
    yearId.value = value;
    if (value) {
      localStorage.setItem(YEAR_KEY, String(value));
      return;
    }
    localStorage.removeItem(YEAR_KEY);
  }

  function setPeriodCode(value: AssessmentPeriodCode): void {
    periodCode.value = value;
    localStorage.setItem(PERIOD_KEY, value);
  }

  async function ensureInitialized(force = false): Promise<void> {
    if (initialized.value && !force) {
      return;
    }
    loadingYears.value = true;
    try {
      years.value = await listAssessmentYears();
      if (years.value.length === 0) {
        setYear(undefined);
      } else if (!yearId.value || !years.value.some((item) => item.id === yearId.value)) {
        setYear(years.value[0].id);
      }
      if (!PERIOD_SET.has(periodCode.value)) {
        setPeriodCode(DEFAULT_PERIOD);
      }
      initialized.value = true;
    } finally {
      loadingYears.value = false;
    }
  }

  return {
    years,
    yearId,
    periodCode,
    currentYear,
    loadingYears,
    initialized,
    setYear,
    setPeriodCode,
    ensureInitialized,
  };
});
