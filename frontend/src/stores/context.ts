import { computed, ref } from "vue";
import { defineStore } from "pinia";
import {
  getAssessmentSession,
  listAssessmentSessions,
} from "@/api/assessment";
import type {
  AssessmentObjectGroupItem,
  AssessmentSessionDetail,
  AssessmentSessionItem,
} from "@/types/assessment";

const SESSION_KEY = "assessv2_context_session_id";
const PERIOD_KEY = "assessv2_context_period_code";
const GROUP_KEY = "assessv2_context_group_code";

function readStoredSessionId(): number | undefined {
  const raw = localStorage.getItem(SESSION_KEY);
  if (!raw) {
    return undefined;
  }
  const parsed = Number(raw);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }
  return parsed;
}

function readStoredText(key: string): string {
  return String(localStorage.getItem(key) || "").trim();
}

export const useContextStore = defineStore("context", () => {
  const sessions = ref<AssessmentSessionItem[]>([]);
  const periods = ref<AssessmentSessionDetail["periods"]>([]);
  const objectGroups = ref<AssessmentObjectGroupItem[]>([]);

  const sessionId = ref<number | undefined>(readStoredSessionId());
  const periodCode = ref<string>(readStoredText(PERIOD_KEY));
  const objectGroupCode = ref<string>(readStoredText(GROUP_KEY));

  const loadingSessions = ref(false);
  const loadingDetail = ref(false);
  const initialized = ref(false);

  const currentSession = computed(() => sessions.value.find((item) => item.id === sessionId.value));
  const currentPeriod = computed(() => periods.value.find((item) => item.periodCode === periodCode.value));
  const currentObjectGroup = computed(() =>
    objectGroups.value.find((item) => item.groupCode === objectGroupCode.value),
  );

  const objectGroupOptions = computed(() =>
    [...objectGroups.value]
      .sort((a, b) => {
        if (a.sortOrder !== b.sortOrder) {
          return a.sortOrder - b.sortOrder;
        }
        return a.id - b.id;
      })
      .map((item) => ({
        value: item.groupCode,
        label: `${item.objectType === "team" ? "团体" : "个人"} - ${item.groupName}`,
      })),
  );

  function persistSession(value: number | undefined): void {
    sessionId.value = value;
    if (value && value > 0) {
      localStorage.setItem(SESSION_KEY, String(value));
      return;
    }
    localStorage.removeItem(SESSION_KEY);
  }

  function persistPeriod(value: string): void {
    periodCode.value = value;
    if (value) {
      localStorage.setItem(PERIOD_KEY, value);
      return;
    }
    localStorage.removeItem(PERIOD_KEY);
  }

  function persistGroup(value: string): void {
    objectGroupCode.value = value;
    if (value) {
      localStorage.setItem(GROUP_KEY, value);
      return;
    }
    localStorage.removeItem(GROUP_KEY);
  }

  function normalizeSelections(): void {
    if (periods.value.length === 0) {
      persistPeriod("");
    } else if (!periods.value.some((item) => item.periodCode === periodCode.value)) {
      persistPeriod(periods.value[0].periodCode);
    }

    if (objectGroups.value.length === 0) {
      persistGroup("");
    } else if (!objectGroups.value.some((item) => item.groupCode === objectGroupCode.value)) {
      persistGroup(objectGroups.value[0].groupCode);
    }
  }

  async function loadSessionDetail(targetSessionId?: number): Promise<void> {
    if (!targetSessionId) {
      periods.value = [];
      objectGroups.value = [];
      normalizeSelections();
      return;
    }
    loadingDetail.value = true;
    try {
      const detail = await getAssessmentSession(targetSessionId);
      periods.value = detail.periods;
      objectGroups.value = detail.objectGroups;
      normalizeSelections();
    } finally {
      loadingDetail.value = false;
    }
  }

  async function setSession(value: number | undefined): Promise<void> {
    persistSession(value);
    await loadSessionDetail(value);
  }

  function setPeriodCode(value: string): void {
    const normalized = String(value || "").trim().toUpperCase();
    if (normalized && !periods.value.some((item) => item.periodCode === normalized)) {
      persistPeriod(periods.value[0]?.periodCode || "");
      return;
    }
    persistPeriod(normalized);
  }

  function setObjectGroupCode(value: string): void {
    const normalized = String(value || "").trim();
    if (normalized && !objectGroups.value.some((item) => item.groupCode === normalized)) {
      persistGroup(objectGroups.value[0]?.groupCode || "");
      return;
    }
    persistGroup(normalized);
  }

  async function ensureInitialized(force = false): Promise<void> {
    if (initialized.value && !force) {
      return;
    }
    loadingSessions.value = true;
    try {
      sessions.value = await listAssessmentSessions();
      if (sessions.value.length === 0) {
        await setSession(undefined);
      } else {
        const hit = sessionId.value && sessions.value.some((item) => item.id === sessionId.value);
        if (!hit) {
          persistSession(sessions.value[0].id);
        }
        await loadSessionDetail(sessionId.value);
      }
      initialized.value = true;
    } finally {
      loadingSessions.value = false;
    }
  }

  async function refreshSessions(): Promise<void> {
    await ensureInitialized(true);
  }

  async function refreshCurrentDetail(): Promise<void> {
    await loadSessionDetail(sessionId.value);
  }

  return {
    sessions,
    periods,
    objectGroups,
    sessionId,
    periodCode,
    objectGroupCode,
    currentSession,
    currentPeriod,
    currentObjectGroup,
    objectGroupOptions,
    loadingSessions,
    loadingDetail,
    initialized,
    setSession,
    setPeriodCode,
    setObjectGroupCode,
    refreshSessions,
    refreshCurrentDetail,
    ensureInitialized,
  };
});
