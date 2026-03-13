<template>
  <div class="vote-statistics-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>M4 分数采集 - 投票统计</strong>
          <el-button :loading="loadingStats" @click="loadStatistics">刷新</el-button>
        </div>
      </template>

      <div class="filters">
        <el-select v-model="contextYearId" placeholder="年度" style="width: 150px">
          <el-option
            v-for="item in contextStore.years"
            :key="item.id"
            :label="formatAssessmentYearLabel(item)"
            :value="item.id"
          />
        </el-select>
        <el-select v-model="contextPeriodCode" placeholder="周期" style="width: 140px">
          <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select
          v-model="filters.moduleId"
          filterable
          placeholder="投票模块"
          style="width: 260px"
          @change="loadStatistics"
        >
          <el-option
            v-for="item in voteModules"
            :key="item.id"
            :label="`${item.moduleName} (${item.moduleKey})`"
            :value="item.id"
          />
        </el-select>
        <el-select
          v-model="filters.objectId"
          clearable
          filterable
          placeholder="考核对象（可选）"
          style="width: 260px"
          @change="loadStatistics"
        >
          <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
        </el-select>
      </div>

      <el-row :gutter="12" class="overview-row">
        <el-col :span="6">
          <el-statistic title="总任务数" :value="stats.totalTasks" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="已完成" :value="stats.completedTasks" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="待处理" :value="stats.pendingTasks" />
        </el-col>
        <el-col :span="6">
          <el-statistic title="完成率" :value="completionRateText" />
        </el-col>
      </el-row>

      <el-progress :percentage="completionRatePercent" :stroke-width="16" status="success" class="progress" />

      <el-table v-loading="loadingStats" :data="stats.groupStatistics" border>
        <el-table-column prop="voteGroupId" label="组ID" width="90" />
        <el-table-column label="投票组" min-width="230">
          <template #default="{ row }">{{ row.groupName }} ({{ row.groupCode }})</template>
        </el-table-column>
        <el-table-column prop="totalTasks" label="总任务" width="100" />
        <el-table-column prop="completedTasks" label="已完成" width="100" />
        <el-table-column prop="pendingTasks" label="待处理" width="100" />
        <el-table-column prop="expiredTasks" label="已过期" width="100" />
        <el-table-column label="优" width="90">
          <template #default="{ row }">{{ row.gradeCounts.excellent || 0 }}</template>
        </el-table-column>
        <el-table-column label="良" width="90">
          <template #default="{ row }">{{ row.gradeCounts.good || 0 }}</template>
        </el-table-column>
        <el-table-column label="中" width="90">
          <template #default="{ row }">{{ row.gradeCounts.average || 0 }}</template>
        </el-table-column>
        <el-table-column label="差" width="90">
          <template #default="{ row }">{{ row.gradeCounts.poor || 0 }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useContextStore } from "@/stores/context";
import { listAssessmentObjects } from "@/api/assessment";
import { listRuleModuleOptions, type RuleModuleOption } from "@/api/rules";
import { getVoteStatistics } from "@/api/vote";
import type { AssessmentObjectItem } from "@/types/assessment";
import type { ScorePeriodCode } from "@/types/score";
import type { VoteStatistics } from "@/types/vote";
import { PERIOD_OPTIONS, formatAssessmentYearLabel } from "@/utils/assessment";

const periodOptions = PERIOD_OPTIONS;
const contextStore = useContextStore();

const objects = ref<AssessmentObjectItem[]>([]);
const voteModules = ref<RuleModuleOption[]>([]);
const loadingStats = ref(false);
const stats = ref<VoteStatistics>({
  totalTasks: 0,
  completedTasks: 0,
  pendingTasks: 0,
  expiredTasks: 0,
  completionRate: 0,
  groupStatistics: [],
});

const filters = reactive({
  moduleId: undefined as number | undefined,
  objectId: undefined as number | undefined,
});

const contextYearId = computed({
  get: () => contextStore.yearId,
  set: (value) => contextStore.setYear(value),
});
const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value) => contextStore.setPeriodCode(value),
});

const completionRatePercent = computed(() => Number((stats.value.completionRate * 100).toFixed(2)));
const completionRateText = computed(() => `${completionRatePercent.value}%`);

async function loadObjectsForYear(yearId: number): Promise<void> {
  objects.value = await listAssessmentObjects(yearId);
  if (filters.objectId && !objects.value.some((item) => item.id === filters.objectId)) {
    filters.objectId = undefined;
  }
}

async function loadVoteModules(): Promise<void> {
  if (!contextStore.yearId) {
    voteModules.value = [];
    return;
  }
  voteModules.value = await listRuleModuleOptions(
    {
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode,
    },
    ["vote"],
  );
  if (!filters.moduleId || !voteModules.value.some((item) => item.id === filters.moduleId)) {
    filters.moduleId = voteModules.value[0]?.id;
  }
}

async function loadStatistics(): Promise<void> {
  if (!contextStore.yearId || !filters.moduleId) {
    stats.value = {
      totalTasks: 0,
      completedTasks: 0,
      pendingTasks: 0,
      expiredTasks: 0,
      completionRate: 0,
      groupStatistics: [],
    };
    return;
  }
  loadingStats.value = true;
  try {
    stats.value = await getVoteStatistics({
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      moduleId: filters.moduleId,
      objectId: filters.objectId,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "投票统计加载失败";
    ElMessage.error(message);
  } finally {
    loadingStats.value = false;
  }
}

async function handleContextChanged(): Promise<void> {
  if (!contextStore.yearId) {
    return;
  }
  try {
    await Promise.all([loadObjectsForYear(contextStore.yearId), loadVoteModules()]);
    await loadStatistics();
  } catch (error) {
    const message = error instanceof Error ? error.message : "上下文加载失败";
    ElMessage.error(message);
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
    await handleContextChanged();
  } catch (error) {
    const message = error instanceof Error ? error.message : "页面初始化失败";
    ElMessage.error(message);
  }
});

watch(
  () => [contextStore.yearId, contextStore.periodCode],
  async () => {
    await handleContextChanged();
  },
);
</script>

<style scoped>
.vote-statistics-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 12px;
}

.overview-row {
  margin-bottom: 8px;
}

.progress {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}
</style>
