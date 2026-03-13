import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { listAssessmentPeriods, listAssessmentYears } from "@/api/assessment";
import { listAssessmentCategories } from "@/api/org";
import {
  allAssessmentCategories,
  applyAssessmentCategoryDefinitions,
  globalAssessmentCategoryOptions,
  isAssessmentObjectCategory,
} from "@/constants/assessmentCategories";
import type {
  GlobalAssessmentObjectCategory,
  AssessmentPeriodCode,
  AssessmentPeriodItem,
  AssessmentYearItem,
} from "@/types/assessment";
import type { AssessmentCategoryItem } from "@/types/org";

const YEAR_KEY = "assessv2_context_year_id";
const PERIOD_KEY = "assessv2_context_period_code";
const OBJECT_CATEGORY_KEY = "assessv2_context_object_category";
const LEGACY_OBJECT_TYPE_KEY = "assessv2_context_object_type";

const DEFAULT_PERIOD: AssessmentPeriodCode = "Q1";
const DEFAULT_OBJECT_CATEGORY: GlobalAssessmentObjectCategory = "all";
const PERIOD_SET = new Set<AssessmentPeriodCode>(["Q1", "Q2", "Q3", "Q4", "YEAR_END"]);

function objectCategorySet(): Set<GlobalAssessmentObjectCategory> {
  const result = new Set<GlobalAssessmentObjectCategory>(["all"]);
  for (const item of allAssessmentCategories()) {
    result.add(item);
  }
  return result;
}

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

function mapLegacyObjectTypeToCategory(value: string | null): GlobalAssessmentObjectCategory {
  if (value === "all") {
    return value;
  }
  return DEFAULT_OBJECT_CATEGORY;
}

function readStoredObjectCategory(): GlobalAssessmentObjectCategory {
  const value = localStorage.getItem(OBJECT_CATEGORY_KEY) as GlobalAssessmentObjectCategory | null;
  if (value && objectCategorySet().has(value)) {
    return value;
  }

  const legacyType = localStorage.getItem(LEGACY_OBJECT_TYPE_KEY);
  return mapLegacyObjectTypeToCategory(legacyType);
}

export const useContextStore = defineStore("context", () => {
  const years = ref<AssessmentYearItem[]>([]);
  const periods = ref<AssessmentPeriodItem[]>([]);
  const assessmentCategories = ref<AssessmentCategoryItem[]>([]);

  const yearId = ref<number | undefined>(readStoredYearId());
  const periodCode = ref<AssessmentPeriodCode>(readStoredPeriodCode());
  const objectCategory = ref<GlobalAssessmentObjectCategory>(readStoredObjectCategory());

  const loadingYears = ref(false);
  const loadingPeriods = ref(false);
  const loadingCategories = ref(false);
  const initialized = ref(false);

  const currentYear = computed(() => years.value.find((item) => item.id === yearId.value));
  const currentPeriod = computed(() => periods.value.find((item) => item.periodCode === periodCode.value));
  const categoryOptions = computed(() => globalAssessmentCategoryOptions());

  function setPeriodCode(value: AssessmentPeriodCode): void {
    const nextValue = PERIOD_SET.has(value) ? value : DEFAULT_PERIOD;
    periodCode.value = nextValue;
    localStorage.setItem(PERIOD_KEY, nextValue);
  }

  function setObjectCategory(value: GlobalAssessmentObjectCategory | string): void {
    const nextValue = objectCategorySet().has(value as GlobalAssessmentObjectCategory)
      ? (value as GlobalAssessmentObjectCategory)
      : DEFAULT_OBJECT_CATEGORY;
    objectCategory.value = nextValue;
    localStorage.setItem(OBJECT_CATEGORY_KEY, nextValue);
    localStorage.removeItem(LEGACY_OBJECT_TYPE_KEY);
  }

  function persistYear(value: number | undefined): void {
    yearId.value = value;
    if (value) {
      localStorage.setItem(YEAR_KEY, String(value));
      return;
    }
    localStorage.removeItem(YEAR_KEY);
  }

  function normalizePeriodSelection(): void {
    if (periods.value.length === 0) {
      setPeriodCode(DEFAULT_PERIOD);
      return;
    }

    if (!periods.value.some((item) => item.periodCode === periodCode.value)) {
      setPeriodCode(periods.value[0].periodCode);
    }
  }

  async function loadPeriodsForYear(targetYearId?: number): Promise<void> {
    if (!targetYearId) {
      periods.value = [];
      return;
    }

    loadingPeriods.value = true;
    try {
      periods.value = await listAssessmentPeriods(targetYearId);
      normalizePeriodSelection();
    } finally {
      loadingPeriods.value = false;
    }
  }

  async function setYear(value: number | undefined): Promise<void> {
    persistYear(value);
    await loadPeriodsForYear(value);
  }

  async function refreshAssessmentCategories(): Promise<void> {
    loadingCategories.value = true;
    try {
      const items = await listAssessmentCategories({ status: "active" });
      assessmentCategories.value = items;
      applyAssessmentCategoryDefinitions(
        items.map((item) => ({
          categoryCode: item.categoryCode,
          categoryName: item.categoryName,
          objectType: item.objectType,
          sortOrder: item.sortOrder,
        })),
      );
    } finally {
      loadingCategories.value = false;
    }
  }

  async function refreshPeriods(): Promise<void> {
    await loadPeriodsForYear(yearId.value);
  }

  async function ensureInitialized(force = false): Promise<void> {
    if (initialized.value && !force) {
      return;
    }

    loadingYears.value = true;
    try {
      try {
        await refreshAssessmentCategories();
      } catch (_error) {
        // Fall back to built-in category dictionary when category metadata API is unavailable.
      }
      years.value = await listAssessmentYears();
      if (years.value.length === 0) {
        await setYear(undefined);
      } else {
        if (!yearId.value || !years.value.some((item) => item.id === yearId.value)) {
          persistYear(years.value[0].id);
        }
        await loadPeriodsForYear(yearId.value);
      }

      if (!PERIOD_SET.has(periodCode.value)) {
        setPeriodCode(DEFAULT_PERIOD);
      }
      if (!objectCategorySet().has(objectCategory.value)) {
        setObjectCategory(DEFAULT_OBJECT_CATEGORY);
      }
      if (objectCategory.value !== "all" && !isAssessmentObjectCategory(objectCategory.value)) {
        setObjectCategory(DEFAULT_OBJECT_CATEGORY);
      }
      normalizePeriodSelection();

      initialized.value = true;
    } finally {
      loadingYears.value = false;
    }
  }

  return {
    years,
    periods,
    assessmentCategories,
    yearId,
    periodCode,
    objectCategory,
    currentYear,
    currentPeriod,
    categoryOptions,
    loadingYears,
    loadingPeriods,
    loadingCategories,
    initialized,
    setYear,
    setPeriodCode,
    setObjectCategory,
    refreshPeriods,
    refreshAssessmentCategories,
    ensureInitialized,
  };
});
