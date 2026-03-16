<template>
  <div class="result-overview-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <strong>M5 Result Overview</strong>
            <span class="context-text">{{ contextText }}</span>
          </div>
          <div class="header-actions">
            <el-button
              v-if="canRecalculate"
              type="primary"
              :loading="recalculating"
              :disabled="!contextStore.yearId || loading"
              @click="handleManualRecalculate"
            >
              Manual Recalculate
            </el-button>
            <el-button :loading="loading" @click="loadOverview">Refresh</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="!contextStore.yearId"
        title="Please select an assessment year first"
        type="warning"
        :closable="false"
        class="overview-alert"
      />

      <el-alert
        v-else
        :title="`Current period status: ${currentPeriodStatusText}`"
        type="info"
        :closable="false"
        class="overview-alert"
      />

      <el-row :gutter="12" class="overview-row">
        <el-col :span="6">
          <el-statistic title="Assessment Objects" :value="summary.objectCount" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Calculated Objects" :value="summary.calculatedObjectCount" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Weighted Total" :value="formatFloat(summary.weightedTotal, 2)" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Extra Points Total" :value="signedText(summary.extraTotal)" />
        </el-col>
      </el-row>

      <el-row :gutter="12" class="overview-row">
        <el-col :span="6">
          <el-statistic title="Final Total" :value="formatFloat(summary.finalTotal, 2)" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Vote Tasks" :value="summary.voteTotal" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Completed Votes" :value="summary.voteCompleted" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="Vote Completion" :value="`${formatFloat(summary.voteCompletionRate * 100, 2)}%`" />
        </el-col>
      </el-row>

      <el-progress
        :percentage="Number((summary.voteCompletionRate * 100).toFixed(2))"
        :stroke-width="16"
        status="success"
        class="progress"
      />

      <div class="table-filter">
        <el-input
          v-model="keyword"
          clearable
          placeholder="Filter by object name"
          style="width: 260px"
        />
      </div>

      <el-table v-loading="loading" :data="filteredRows" border row-key="objectId">
        <el-table-column label="Overall Rank" width="110">
          <template #default="{ row }">{{ row.overallRank ?? "-" }}</template>
        </el-table-column>
        <el-table-column label="Group Rank" width="100">
          <template #default="{ row }">{{ row.groupRank ?? "-" }}</template>
        </el-table-column>
        <el-table-column prop="objectName" label="Object" min-width="200" />
        <el-table-column label="Type" width="110">
          <template #default="{ row }">{{ objectTypeText(row.objectType) }}</template>
        </el-table-column>
        <el-table-column label="Category" min-width="180">
          <template #default="{ row }">{{ row.objectCategory ? assessmentCategoryLabel(row.objectCategory) : "-" }}</template>
        </el-table-column>
        <el-table-column label="Weighted Score" width="130">
          <template #default="{ row }">{{ formatFloat(row.weightedScore, 2) }}</template>
        </el-table-column>
        <el-table-column label="Extra" width="110">
          <template #default="{ row }">{{ signedText(row.extraPoints) }}</template>
        </el-table-column>
        <el-table-column label="Final Score" width="130">
          <template #default="{ row }">
            <strong>{{ formatFloat(row.finalScore, 2) }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="Calculated At" min-width="170">
          <template #default="{ row }">{{ formatTimestamp(row.calculatedAt) }}</template>
        </el-table-column>
        <el-table-column label="Vote Progress" min-width="170">
          <template #default="{ row }">
            {{ row.voteCompleted }}/{{ row.voteTotal }}
            <span class="vote-detail">(Pending {{ row.votePending }} / Expired {{ row.voteExpired }})</span>
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="120" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="!row.calculatedScoreId" @click="openModuleDetails(row)">
              Details
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="moduleDialogVisible" :title="moduleDialogTitle" width="860px">
      <el-table v-loading="moduleLoading" :data="moduleRows" border>
        <el-table-column label="#" width="70">
          <template #default="{ $index }">{{ $index + 1 }}</template>
        </el-table-column>
        <el-table-column prop="moduleName" label="Module" min-width="220" />
        <el-table-column prop="moduleCode" label="Code" width="120" />
        <el-table-column prop="moduleKey" label="Key" min-width="150" />
        <el-table-column label="Raw Score" width="120">
          <template #default="{ row }">{{ formatFloat(row.rawScore, 2) }}</template>
        </el-table-column>
        <el-table-column label="Weighted" width="120">
          <template #default="{ row }">{{ formatFloat(row.weightedScore, 2) }}</template>
        </el-table-column>
        <el-table-column label="Detail" width="100" fixed="right">
          <template #default="{ row }">
            <el-popover trigger="click" placement="left" width="460">
              <template #reference>
                <el-button link type="primary">View</el-button>
              </template>
              <pre class="module-detail">{{ formatModuleDetail(row.scoreDetail) }}</pre>
            </el-popover>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { assessmentCategoryLabel } from "@/constants/assessmentCategories";
import { listAssessmentObjects } from "@/api/assessment";
import { listCalculatedModuleScores, listCalculatedScores, listRankings, recalculateScores } from "@/api/calc";
import { listVoteTasks } from "@/api/vote";
import type {
  AssessmentObjectCategory,
  AssessmentObjectType,
  GlobalAssessmentObjectCategory,
} from "@/types/assessment";
import type { CalculatedModuleScoreItem } from "@/types/calc";
import type { ScorePeriodCode } from "@/types/score";
import type { VoteTaskStatus } from "@/types/vote";
import { formatAssessmentYearLabel, formatFloat, formatTimestamp, periodStatusText } from "@/utils/assessment";

interface ObjectScoreRow {
  objectId: number;
  objectName: string;
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
  calculatedScoreId?: number;
  weightedScore: number;
  extraPoints: number;
  finalScore: number;
  overallRank?: number;
  groupRank?: number;
  calculatedAt?: number;
  voteTotal: number;
  voteCompleted: number;
  votePending: number;
  voteExpired: number;
}

interface OverviewSummary {
  objectCount: number;
  calculatedObjectCount: number;
  weightedTotal: number;
  extraTotal: number;
  finalTotal: number;
  voteTotal: number;
  voteCompleted: number;
  voteCompletionRate: number;
}

const appStore = useAppStore();
const contextStore = useContextStore();

const canRecalculate = computed(() => appStore.hasPermission("score:update"));
const loading = ref(false);
const recalculating = ref(false);
const keyword = ref("");
const rows = ref<ObjectScoreRow[]>([]);

const moduleDialogVisible = ref(false);
const moduleLoading = ref(false);
const moduleRows = ref<CalculatedModuleScoreItem[]>([]);
const activeRow = ref<ObjectScoreRow | null>(null);

const summary = ref<OverviewSummary>({
  objectCount: 0,
  calculatedObjectCount: 0,
  weightedTotal: 0,
  extraTotal: 0,
  finalTotal: 0,
  voteTotal: 0,
  voteCompleted: 0,
  voteCompletionRate: 0,
});

const currentPeriodStatusText = computed(() => {
  const status = contextStore.currentPeriod?.status;
  if (!status) {
    return "Unknown";
  }
  return periodStatusText(status);
});

const contextText = computed(() => {
  const yearText = formatAssessmentYearLabel(contextStore.currentYear);
  const periodText = contextStore.periodCode;
  const objectCategoryTextValue = objectCategoryText(contextStore.objectCategory);
  return `${yearText} / ${periodText} / ${objectCategoryTextValue}`;
});

const filteredRows = computed(() => {
  const text = keyword.value.trim().toLowerCase();
  if (!text) {
    return rows.value;
  }
  return rows.value.filter((item) => item.objectName.toLowerCase().includes(text));
});

const moduleDialogTitle = computed(() => {
  if (!activeRow.value) {
    return "Module Score Details";
  }
  return `Module Score Details - ${activeRow.value.objectName}`;
});

function objectCategoryText(value: GlobalAssessmentObjectCategory): string {
  if (value === "all") {
    return "All Categories";
  }
  return assessmentCategoryLabel(value);
}

function signedText(value: number): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${formatFloat(value, 2)}`;
}

function objectTypeText(value?: AssessmentObjectType): string {
  switch (value) {
    case "team":
      return "Team";
    case "individual":
      return "Individual";
    default:
      return "-";
  }
}

function createRow(
  objectId: number,
  objectName: string,
  objectType?: AssessmentObjectType,
  objectCategory?: AssessmentObjectCategory,
): ObjectScoreRow {
  return {
    objectId,
    objectName,
    objectType,
    objectCategory,
    calculatedScoreId: undefined,
    weightedScore: 0,
    extraPoints: 0,
    finalScore: 0,
    overallRank: undefined,
    groupRank: undefined,
    calculatedAt: undefined,
    voteTotal: 0,
    voteCompleted: 0,
    votePending: 0,
    voteExpired: 0,
  };
}

function countVoteStatus(row: ObjectScoreRow, status: VoteTaskStatus): void {
  row.voteTotal += 1;
  switch (status) {
    case "completed":
      row.voteCompleted += 1;
      break;
    case "pending":
      row.votePending += 1;
      break;
    case "expired":
      row.voteExpired += 1;
      break;
    default:
      break;
  }
}

function computeSummary(items: ObjectScoreRow[]): OverviewSummary {
  const objectCount = items.length;
  const calculatedObjectCount = items.filter((item) => Boolean(item.calculatedScoreId)).length;
  const weightedTotal = items.reduce((acc, item) => acc + item.weightedScore, 0);
  const extraTotal = items.reduce((acc, item) => acc + item.extraPoints, 0);
  const finalTotal = items.reduce((acc, item) => acc + item.finalScore, 0);
  const voteTotal = items.reduce((acc, item) => acc + item.voteTotal, 0);
  const voteCompleted = items.reduce((acc, item) => acc + item.voteCompleted, 0);

  return {
    objectCount,
    calculatedObjectCount,
    weightedTotal,
    extraTotal,
    finalTotal,
    voteTotal,
    voteCompleted,
    voteCompletionRate: voteTotal > 0 ? voteCompleted / voteTotal : 0,
  };
}

function emptySummary(): OverviewSummary {
  return {
    objectCount: 0,
    calculatedObjectCount: 0,
    weightedTotal: 0,
    extraTotal: 0,
    finalTotal: 0,
    voteTotal: 0,
    voteCompleted: 0,
    voteCompletionRate: 0,
  };
}

function formatModuleDetail(detail: string): string {
  if (!detail) {
    return "-";
  }
  try {
    const parsed = JSON.parse(detail) as unknown;
    return JSON.stringify(parsed, null, 2);
  } catch (_error) {
    return detail;
  }
}

async function loadOverview(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    summary.value = emptySummary();
    return;
  }

  loading.value = true;
  try {
    const yearId = contextStore.yearId;
    const periodCode = contextStore.periodCode as ScorePeriodCode;
    const objectCategory = contextStore.objectCategory === "all" ? undefined : contextStore.objectCategory;

    const [objects, calculatedScores, voteTasks, groupRankings] = await Promise.all([
      listAssessmentObjects(yearId),
      listCalculatedScores({ yearId, periodCode, objectCategory }),
      listVoteTasks({ yearId, periodCode }),
      listRankings({ yearId, periodCode, scope: "parent_object", objectType: "individual", objectCategory }),
    ]);

    const rowMap = new Map<number, ObjectScoreRow>();
    for (const item of objects) {
      rowMap.set(item.id, createRow(item.id, item.objectName, item.objectType, item.objectCategory));
    }

    for (const item of calculatedScores) {
      const row = rowMap.get(item.objectId) || createRow(item.objectId, item.objectName, item.objectType, item.objectCategory);
      row.objectName = item.objectName;
      row.objectType = item.objectType;
      row.objectCategory = item.objectCategory;
      row.calculatedScoreId = item.id;
      row.weightedScore = item.weightedScore;
      row.extraPoints = item.extraPoints;
      row.finalScore = item.finalScore;
      row.overallRank = item.overallRank;
      row.calculatedAt = item.calculatedAt;
      rowMap.set(item.objectId, row);
    }

    const groupRankMap = new Map<number, number>();
    for (const item of groupRankings) {
      groupRankMap.set(item.objectId, item.rankNo);
    }

    for (const item of voteTasks) {
      const row = rowMap.get(item.objectId) || createRow(item.objectId, `Object#${item.objectId}`);
      countVoteStatus(row, item.status);
      rowMap.set(item.objectId, row);
    }

    let nextRows = Array.from(rowMap.values());
    if (objectCategory) {
      nextRows = nextRows.filter((item) => item.objectCategory === objectCategory);
    }

    nextRows = nextRows.map((item) => ({
      ...item,
      groupRank: item.objectType === "individual" ? groupRankMap.get(item.objectId) : undefined,
    }));

    nextRows.sort((a, b) => {
      const aRank = a.overallRank;
      const bRank = b.overallRank;
      if (typeof aRank === "number" && typeof bRank === "number" && aRank !== bRank) {
        return aRank - bRank;
      }
      if (typeof aRank === "number" && typeof bRank !== "number") {
        return -1;
      }
      if (typeof aRank !== "number" && typeof bRank === "number") {
        return 1;
      }
      if (b.finalScore !== a.finalScore) {
        return b.finalScore - a.finalScore;
      }
      return a.objectName.localeCompare(b.objectName, "zh-CN");
    });

    rows.value = nextRows;
    summary.value = computeSummary(nextRows);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to load overview";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

async function handleManualRecalculate(): Promise<void> {
  if (!contextStore.yearId) {
    ElMessage.warning("Please select an assessment year first");
    return;
  }

  recalculating.value = true;
  try {
    const payload = {
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      objectCategory: contextStore.objectCategory === "all" ? undefined : contextStore.objectCategory,
    };
    const result = await recalculateScores(payload);
    ElMessage.success(
      `Recalculated ${result.calculatedObjects}/${result.totalObjects} objects in ${result.durationMs}ms`,
    );
    await loadOverview();
  } catch (error) {
    const message = error instanceof Error ? error.message : "Manual recalculate failed";
    ElMessage.error(message);
  } finally {
    recalculating.value = false;
  }
}

async function openModuleDetails(row: ObjectScoreRow): Promise<void> {
  if (!row.calculatedScoreId) {
    ElMessage.warning("No calculated score detail available");
    return;
  }

  activeRow.value = row;
  moduleDialogVisible.value = true;
  moduleLoading.value = true;
  try {
    moduleRows.value = await listCalculatedModuleScores(row.calculatedScoreId);
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to load module details";
    ElMessage.error(message);
    moduleRows.value = [];
  } finally {
    moduleLoading.value = false;
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
    await loadOverview();
  } catch (error) {
    const message = error instanceof Error ? error.message : "Page initialization failed";
    ElMessage.error(message);
  }
});

watch(
  () => [contextStore.yearId, contextStore.periodCode, contextStore.objectCategory],
  async () => {
    await loadOverview();
  },
);
</script>

<style scoped>
.result-overview-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.header-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.context-text {
  font-size: 13px;
  color: #606266;
}

.overview-alert {
  margin-bottom: 12px;
}

.overview-row {
  margin-bottom: 10px;
}

.progress {
  margin-bottom: 14px;
}

.table-filter {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.vote-detail {
  margin-left: 6px;
  color: #909399;
  font-size: 12px;
}

.module-detail {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  line-height: 1.45;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .table-filter {
    justify-content: flex-start;
  }
}
</style>
