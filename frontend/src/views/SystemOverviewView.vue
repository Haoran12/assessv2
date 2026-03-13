<template>
  <div class="system-overview-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div>
            <strong>系统概览</strong>
            <div class="subtitle">下方分数表会实时使用 TopBar 选择的年度、周期和分类</div>
          </div>
          <el-button :loading="syncingLatest" @click="syncToLatestContext">定位最新周期</el-button>
        </div>
      </template>

      <el-alert :title="contextText" type="info" :closable="false" />
    </el-card>

    <ResultOverviewView />

    <AssessmentView />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useContextStore } from "@/stores/context";
import { assessmentCategoryLabel } from "@/constants/assessmentCategories";
import { listAssessmentPeriods, listAssessmentYears } from "@/api/assessment";
import type { AssessmentPeriodCode, AssessmentPeriodItem, AssessmentYearItem } from "@/types/assessment";
import AssessmentView from "@/views/AssessmentView.vue";
import ResultOverviewView from "@/views/ResultOverviewView.vue";
import { formatAssessmentYearLabel } from "@/utils/assessment";

interface LatestContext {
  year: AssessmentYearItem;
  period: AssessmentPeriodItem;
}

const contextStore = useContextStore();
const syncingLatest = ref(false);

const periodOrder: Record<AssessmentPeriodCode, number> = {
  Q1: 1,
  Q2: 2,
  Q3: 3,
  Q4: 4,
  YEAR_END: 5,
};

const contextText = computed(() => {
  const yearText = formatAssessmentYearLabel(contextStore.currentYear);
  const periodText = contextStore.currentPeriod?.periodCode || contextStore.periodCode;
  const objectCategoryText =
    contextStore.objectCategory === "all" ? "全部分类" : assessmentCategoryLabel(contextStore.objectCategory);
  return `当前全局上下文：${yearText} / ${periodText} / ${objectCategoryText}`;
});

function statusPriority(status: AssessmentPeriodItem["status"]): number {
  switch (status) {
    case "active":
      return 0;
    case "ended":
      return 1;
    case "not_started":
      return 2;
    case "locked":
      return 3;
    default:
      return 9;
  }
}

function sortYearsDesc(items: AssessmentYearItem[]): AssessmentYearItem[] {
  return [...items].sort((a, b) => {
    if (b.year !== a.year) {
      return b.year - a.year;
    }
    return b.id - a.id;
  });
}

function pickLatestPeriod(items: AssessmentPeriodItem[]): AssessmentPeriodItem | undefined {
  if (items.length === 0) {
    return undefined;
  }
  return [...items].sort((a, b) => {
    const statusDiff = statusPriority(a.status) - statusPriority(b.status);
    if (statusDiff !== 0) {
      return statusDiff;
    }
    return periodOrder[b.periodCode] - periodOrder[a.periodCode];
  })[0];
}

async function locateLatestContext(): Promise<LatestContext | undefined> {
  const years = sortYearsDesc(await listAssessmentYears());
  for (const year of years) {
    const periods = await listAssessmentPeriods(year.id);
    const picked = pickLatestPeriod(periods);
    if (picked) {
      return { year, period: picked };
    }
  }
  return undefined;
}

async function syncToLatestContext(): Promise<void> {
  syncingLatest.value = true;
  try {
    await contextStore.ensureInitialized();
    const latest = await locateLatestContext();
    if (!latest) {
      ElMessage.warning("未找到可用的年度周期数据");
      return;
    }

    await contextStore.setYear(latest.year.id);
    contextStore.setPeriodCode(latest.period.periodCode);
    ElMessage.success("已切换到最新考核周期");
  } catch (error) {
    const message = error instanceof Error ? error.message : "最新周期定位失败";
    ElMessage.error(message);
  } finally {
    syncingLatest.value = false;
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
  } catch (error) {
    const message = error instanceof Error ? error.message : "系统概览初始化失败";
    ElMessage.error(message);
  }
});
</script>

<style scoped>
.system-overview-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.subtitle {
  margin-top: 4px;
  color: #606266;
  font-size: 13px;
}

@media (max-width: 900px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>


