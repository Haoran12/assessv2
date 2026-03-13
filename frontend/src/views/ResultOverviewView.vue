<template>
  <div class="result-overview-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <strong>考核结果概览</strong>
            <span class="context-text">{{ contextText }}</span>
          </div>
          <el-button :loading="loading" @click="loadOverview">刷新</el-button>
        </div>
      </template>

      <el-alert
        v-if="!contextStore.yearId"
        title="请先在顶部选择考核年度"
        type="warning"
        :closable="false"
        class="overview-alert"
      />

      <el-alert
        v-else
        :title="`当前周期状态：${currentPeriodStatusText}`"
        type="info"
        :closable="false"
        class="overview-alert"
      />

      <el-row :gutter="12" class="overview-row">
        <el-col :span="6">
          <el-statistic title="考核对象" :value="summary.objectCount" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="已产生记录对象" :value="summary.participatedObjectCount" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="直接得分总计" :value="formatFloat(summary.directTotal, 2)" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="加减分净值" :value="signedText(summary.extraNetTotal)" />
        </el-col>
      </el-row>

      <el-row :gutter="12" class="overview-row">
        <el-col :span="6">
          <el-statistic title="综合分总计" :value="formatFloat(summary.finalTotal, 2)" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="投票任务总数" :value="summary.voteTotal" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="已完成投票" :value="summary.voteCompleted" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="投票完成率" :value="`${formatFloat(summary.voteCompletionRate * 100, 2)}%`" />
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
          placeholder="按考核对象名称筛选"
          style="width: 260px"
        />
      </div>

      <el-table v-loading="loading" :data="filteredRows" border>
        <el-table-column label="排名" width="80">
          <template #default="{ $index }">{{ $index + 1 }}</template>
        </el-table-column>
        <el-table-column prop="objectName" label="考核对象" min-width="200" />
        <el-table-column label="对象类型" width="100">
          <template #default="{ row }">{{ objectTypeText(row.objectType) }}</template>
        </el-table-column>
        <el-table-column label="对象分类" min-width="160">
          <template #default="{ row }">{{ assessmentCategoryLabel(row.objectCategory || "-") }}</template>
        </el-table-column>
        <el-table-column label="直接得分" width="120">
          <template #default="{ row }">{{ formatFloat(row.directScore, 2) }}</template>
        </el-table-column>
        <el-table-column label="加分" width="100">
          <template #default="{ row }">{{ formatFloat(row.extraAdd, 2) }}</template>
        </el-table-column>
        <el-table-column label="减分" width="100">
          <template #default="{ row }">{{ formatFloat(row.extraDeduct, 2) }}</template>
        </el-table-column>
        <el-table-column label="净值" width="100">
          <template #default="{ row }">{{ signedText(row.extraNet) }}</template>
        </el-table-column>
        <el-table-column label="综合分" width="120">
          <template #default="{ row }">
            <strong>{{ formatFloat(row.totalScore, 2) }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="投票进度" min-width="160">
          <template #default="{ row }">
            {{ row.voteCompleted }}/{{ row.voteTotal }}
            <span class="vote-detail">(待处理 {{ row.votePending }} / 过期 {{ row.voteExpired }})</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useContextStore } from "@/stores/context";
import { assessmentCategoryLabel } from "@/constants/assessmentCategories";
import { listAssessmentObjects } from "@/api/assessment";
import { listDirectScores, listExtraPoints } from "@/api/score";
import { listVoteTasks } from "@/api/vote";
import type { AssessmentObjectCategory, AssessmentObjectType, GlobalAssessmentObjectCategory } from "@/types/assessment";
import type { ScorePeriodCode } from "@/types/score";
import type { VoteTaskStatus } from "@/types/vote";
import {
  formatAssessmentYearLabel,
  formatFloat,
  periodStatusText,
} from "@/utils/assessment";

interface ObjectScoreRow {
  objectId: number;
  objectName: string;
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
  directScore: number;
  extraAdd: number;
  extraDeduct: number;
  extraNet: number;
  totalScore: number;
  voteTotal: number;
  voteCompleted: number;
  votePending: number;
  voteExpired: number;
}

interface OverviewSummary {
  objectCount: number;
  participatedObjectCount: number;
  directTotal: number;
  extraNetTotal: number;
  finalTotal: number;
  voteTotal: number;
  voteCompleted: number;
  voteCompletionRate: number;
}

const contextStore = useContextStore();

const loading = ref(false);
const keyword = ref("");
const rows = ref<ObjectScoreRow[]>([]);
const summary = ref<OverviewSummary>({
  objectCount: 0,
  participatedObjectCount: 0,
  directTotal: 0,
  extraNetTotal: 0,
  finalTotal: 0,
  voteTotal: 0,
  voteCompleted: 0,
  voteCompletionRate: 0,
});

const currentPeriodStatusText = computed(() => {
  const status = contextStore.currentPeriod?.status;
  if (!status) {
    return "未知";
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
    directScore: 0,
    extraAdd: 0,
    extraDeduct: 0,
    extraNet: 0,
    totalScore: 0,
    voteTotal: 0,
    voteCompleted: 0,
    votePending: 0,
    voteExpired: 0,
  };
}

function objectCategoryText(value: GlobalAssessmentObjectCategory): string {
  if (value === "all") {
    return "全部分类";
  }
  return assessmentCategoryLabel(value);
}

function signedText(value: number): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${formatFloat(value, 2)}`;
}

function objectTypeText(value?: AssessmentObjectType | "all"): string {
  switch (value) {
    case "team":
      return "团体";
    case "individual":
      return "个人";
    case "all":
      return "全部对象";
    default:
      return "-";
  }
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
  const directTotal = items.reduce((acc, item) => acc + item.directScore, 0);
  const extraNetTotal = items.reduce((acc, item) => acc + item.extraNet, 0);
  const finalTotal = items.reduce((acc, item) => acc + item.totalScore, 0);
  const voteTotal = items.reduce((acc, item) => acc + item.voteTotal, 0);
  const voteCompleted = items.reduce((acc, item) => acc + item.voteCompleted, 0);
  const participatedObjectCount = items.filter(
    (item) =>
      item.directScore !== 0 ||
      item.extraNet !== 0 ||
      item.voteTotal > 0,
  ).length;

  return {
    objectCount,
    participatedObjectCount,
    directTotal,
    extraNetTotal,
    finalTotal,
    voteTotal,
    voteCompleted,
    voteCompletionRate: voteTotal > 0 ? voteCompleted / voteTotal : 0,
  };
}

async function loadOverview(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    summary.value = {
      objectCount: 0,
      participatedObjectCount: 0,
      directTotal: 0,
      extraNetTotal: 0,
      finalTotal: 0,
      voteTotal: 0,
      voteCompleted: 0,
      voteCompletionRate: 0,
    };
    return;
  }

  loading.value = true;
  try {
    const yearId = contextStore.yearId;
    const periodCode = contextStore.periodCode as ScorePeriodCode;

    const [objects, directScores, extraPoints, voteTasks] = await Promise.all([
      listAssessmentObjects(yearId),
      listDirectScores({ yearId, periodCode }),
      listExtraPoints({ yearId, periodCode }),
      listVoteTasks({ yearId, periodCode }),
    ]);

    const rowMap = new Map<number, ObjectScoreRow>();
    for (const item of objects) {
      rowMap.set(item.id, createRow(item.id, item.objectName, item.objectType, item.objectCategory));
    }

    for (const item of directScores) {
      const row = rowMap.get(item.objectId) || createRow(item.objectId, `对象#${item.objectId}`);
      row.directScore += item.score;
      rowMap.set(item.objectId, row);
    }

    for (const item of extraPoints) {
      const row = rowMap.get(item.objectId) || createRow(item.objectId, `对象#${item.objectId}`);
      if (item.pointType === "deduct") {
        row.extraDeduct += item.points;
      } else {
        row.extraAdd += item.points;
      }
      rowMap.set(item.objectId, row);
    }

    for (const item of voteTasks) {
      const row = rowMap.get(item.objectId) || createRow(item.objectId, `对象#${item.objectId}`);
      countVoteStatus(row, item.status);
      rowMap.set(item.objectId, row);
    }

    const nextRows = Array.from(rowMap.values()).map((item) => {
      const extraNet = item.extraAdd - item.extraDeduct;
      return {
        ...item,
        extraNet,
        totalScore: item.directScore + extraNet,
      };
    });

    const categoryFilter = contextStore.objectCategory;
    const scopedRows =
      categoryFilter === "all"
        ? nextRows
        : nextRows.filter((item) => item.objectCategory === categoryFilter);

    scopedRows.sort((a, b) => {
      if (b.totalScore !== a.totalScore) {
        return b.totalScore - a.totalScore;
      }
      return a.objectName.localeCompare(b.objectName, "zh-CN");
    });

    rows.value = scopedRows;
    summary.value = computeSummary(scopedRows);
  } catch (error) {
    const message = error instanceof Error ? error.message : "概览数据加载失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
    await loadOverview();
  } catch (error) {
    const message = error instanceof Error ? error.message : "页面初始化失败";
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
