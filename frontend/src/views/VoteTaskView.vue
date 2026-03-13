<template>
  <div class="vote-task-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>M4 分数采集 - 投票任务</strong>
          <el-button :loading="loadingTasks" @click="loadTasks">刷新</el-button>
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
          clearable
          filterable
          placeholder="投票模块"
          style="width: 260px"
          @change="loadTasks"
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
          placeholder="考核对象"
          style="width: 260px"
          @change="loadTasks"
        >
          <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
        </el-select>
        <el-select v-model="filters.status" clearable placeholder="任务状态" style="width: 140px" @change="loadTasks">
          <el-option label="待处理" value="pending" />
          <el-option label="已完成" value="completed" />
          <el-option label="已过期" value="expired" />
        </el-select>
      </div>

      <el-divider content-position="left">发起投票任务</el-divider>
      <div class="generator">
        <el-switch v-model="generateAllObjects" />
        <span>对当前年度周期下全部考核对象生成任务</span>
        <el-select
          v-model="generateObjectIds"
          multiple
          collapse-tags
          filterable
          :disabled="generateAllObjects"
          placeholder="指定考核对象（可多选）"
          style="width: 320px"
        >
          <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
        </el-select>
        <el-button type="primary" :disabled="!canEdit" :loading="generating" @click="handleGenerate">
          生成任务
        </el-button>
      </div>

      <el-table v-loading="loadingTasks" :data="tasks" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="模块" min-width="200">
          <template #default="{ row }">{{ row.moduleName }}</template>
        </el-table-column>
        <el-table-column label="投票组" min-width="220">
          <template #default="{ row }">{{ row.groupName }} ({{ row.groupCode }})</template>
        </el-table-column>
        <el-table-column label="考核对象" min-width="180">
          <template #default="{ row }">{{ objectNameMap[row.objectId] || `对象#${row.objectId}` }}</template>
        </el-table-column>
        <el-table-column label="投票人" width="100">
          <template #default="{ row }">{{ row.voterId }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="档位" width="100">
          <template #default="{ row }">{{ gradeText(row.gradeOption) }}</template>
        </el-table-column>
        <el-table-column label="提交时间" min-width="170">
          <template #default="{ row }">{{ formatTimestamp(row.completedAt || row.votedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="warning" :disabled="!canResetTask(row)" @click="handleReset(row.id)">重置</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { listAssessmentObjects } from "@/api/assessment";
import { listRuleModuleOptions, type RuleModuleOption } from "@/api/rules";
import { generateVoteTasks, listVoteTasks, resetVoteTask } from "@/api/vote";
import type { AssessmentObjectItem } from "@/types/assessment";
import type { ScorePeriodCode } from "@/types/score";
import type { VoteTaskItem, VoteTaskStatus } from "@/types/vote";
import { PERIOD_OPTIONS, formatAssessmentYearLabel, formatTimestamp, toObjectNameMap } from "@/utils/assessment";

const appStore = useAppStore();
const contextStore = useContextStore();
const canEdit = computed(() => appStore.hasPermission("score:update"));
const isRoot = computed(() => appStore.primaryRole === "root");
const periodOptions = PERIOD_OPTIONS;

const objects = ref<AssessmentObjectItem[]>([]);
const voteModules = ref<RuleModuleOption[]>([]);
const tasks = ref<VoteTaskItem[]>([]);
const loadingTasks = ref(false);
const generating = ref(false);

const filters = reactive({
  moduleId: undefined as number | undefined,
  objectId: undefined as number | undefined,
  status: undefined as VoteTaskStatus | undefined,
});

const contextYearId = computed({
  get: () => contextStore.yearId,
  set: (value) => contextStore.setYear(value),
});
const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value) => contextStore.setPeriodCode(value),
});

const generateAllObjects = ref(true);
const generateObjectIds = ref<number[]>([]);
const objectNameMap = computed(() => toObjectNameMap(objects.value));

function statusText(status: VoteTaskStatus): string {
  switch (status) {
    case "pending":
      return "待处理";
    case "completed":
      return "已完成";
    case "expired":
      return "已过期";
    default:
      return status;
  }
}

function statusTagType(status: VoteTaskStatus): "warning" | "success" | "danger" {
  switch (status) {
    case "pending":
      return "warning";
    case "completed":
      return "success";
    case "expired":
      return "danger";
    default:
      return "warning";
  }
}

function gradeText(grade?: string): string {
  switch (grade) {
    case "excellent":
      return "优";
    case "good":
      return "良";
    case "average":
      return "中";
    case "poor":
      return "差";
    default:
      return "-";
  }
}

function canResetTask(row: VoteTaskItem): boolean {
  return isRoot.value && row.status !== "pending";
}

async function loadObjectsForYear(yearId: number): Promise<void> {
  objects.value = await listAssessmentObjects(yearId);
  if (filters.objectId && !objects.value.some((item) => item.id === filters.objectId)) {
    filters.objectId = undefined;
  }
  const objectSet = new Set(objects.value.map((item) => item.id));
  generateObjectIds.value = generateObjectIds.value.filter((item) => objectSet.has(item));
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
  if (filters.moduleId && !voteModules.value.some((item) => item.id === filters.moduleId)) {
    filters.moduleId = undefined;
  }
}

async function loadTasks(): Promise<void> {
  if (!contextStore.yearId) {
    tasks.value = [];
    return;
  }
  loadingTasks.value = true;
  try {
    tasks.value = await listVoteTasks({
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      moduleId: filters.moduleId,
      objectId: filters.objectId,
      status: filters.status,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "投票任务加载失败";
    ElMessage.error(message);
  } finally {
    loadingTasks.value = false;
  }
}

async function handleContextChanged(): Promise<void> {
  if (!contextStore.yearId) {
    return;
  }
  try {
    await Promise.all([loadObjectsForYear(contextStore.yearId), loadVoteModules()]);
    await loadTasks();
  } catch (error) {
    const message = error instanceof Error ? error.message : "上下文加载失败";
    ElMessage.error(message);
  }
}

async function handleGenerate(): Promise<void> {
  if (!contextStore.yearId) {
    ElMessage.warning("请先选择年度");
    return;
  }
  if (!filters.moduleId) {
    ElMessage.warning("请选择投票模块");
    return;
  }
  if (!generateAllObjects.value && generateObjectIds.value.length === 0) {
    ElMessage.warning("请至少选择一个考核对象");
    return;
  }
  generating.value = true;
  try {
    const result = await generateVoteTasks({
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      moduleId: filters.moduleId,
      objectIds: generateAllObjects.value ? [] : generateObjectIds.value,
    });
    ElMessage.success(
      `任务生成完成：新增 ${result.created}，已存在 ${result.skipped}，覆盖对象 ${result.objectCount}，投票人 ${result.voterCount}`,
    );
    await loadTasks();
  } catch (error) {
    const message = error instanceof Error ? error.message : "生成投票任务失败";
    ElMessage.error(message);
  } finally {
    generating.value = false;
  }
}

async function handleReset(taskId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("重置后该任务将回到待处理状态，确认继续吗？", "重置确认", {
      type: "warning",
    });
    await resetVoteTask(taskId);
    ElMessage.success("任务已重置");
    await loadTasks();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "重置任务失败";
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
.vote-task-view {
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

.generator {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
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
