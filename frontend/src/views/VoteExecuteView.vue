<template>
  <div class="vote-execute-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>M4 分数采集 - 执行投票（我的任务）</strong>
          <el-button :loading="loadingTasks" @click="loadTasks">刷新</el-button>
        </div>
      </template>

      <div class="filters">
        <el-select v-model="contextYearId" placeholder="年度" style="width: 150px">
          <el-option
            v-for="item in contextStore.years"
            :key="item.id"
            :label="`${item.year} - ${item.yearName}`"
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

      <el-alert
        :title="`待处理任务 ${pendingCount} 条，已完成 ${completedCount} 条`"
        type="info"
        :closable="false"
        class="stats-alert"
      />

      <el-table v-loading="loadingTasks" :data="tasks" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="模块" min-width="180">
          <template #default="{ row }">{{ row.moduleName }}</template>
        </el-table-column>
        <el-table-column label="投票组" min-width="220">
          <template #default="{ row }">{{ row.groupName }} ({{ row.groupCode }})</template>
        </el-table-column>
        <el-table-column label="考核对象" min-width="180">
          <template #default="{ row }">{{ objectNameMap[row.objectId] || `对象#${row.objectId}` }}</template>
        </el-table-column>
        <el-table-column label="当前档位" width="100">
          <template #default="{ row }">{{ gradeText(row.gradeOption) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="170">
          <template #default="{ row }">{{ formatTimestamp(row.completedAt || row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              :disabled="row.status !== 'pending'"
              @click="openVoteDialog(row)"
            >
              投票
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="voteDialogVisible" title="执行投票" width="560px">
      <el-descriptions :column="1" border size="small" class="vote-summary">
        <el-descriptions-item label="任务ID">{{ currentTask?.id }}</el-descriptions-item>
        <el-descriptions-item label="投票模块">{{ currentTask?.moduleName || "-" }}</el-descriptions-item>
        <el-descriptions-item label="投票组">
          {{ currentTask ? `${currentTask.groupName} (${currentTask.groupCode})` : "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="考核对象">
          {{ currentTask ? objectNameMap[currentTask.objectId] || `对象#${currentTask.objectId}` : "-" }}
        </el-descriptions-item>
      </el-descriptions>

      <el-form ref="voteFormRef" :model="voteForm" :rules="voteFormRules" label-width="90px" class="vote-form">
        <el-form-item label="投票档位" prop="gradeOption">
          <el-radio-group v-model="voteForm.gradeOption">
            <el-radio-button label="excellent">优</el-radio-button>
            <el-radio-button label="good">良</el-radio-button>
            <el-radio-button label="average">中</el-radio-button>
            <el-radio-button label="poor">差</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="评价意见">
          <el-input v-model="voteForm.remark" type="textarea" :rows="3" maxlength="300" show-word-limit />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="voteDialogVisible = false">取消</el-button>
        <el-button :loading="savingDraft" @click="handleSaveDraft">保存草稿</el-button>
        <el-button type="primary" :loading="submittingVote" @click="handleSubmitVote">提交锁定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage, ElMessageBox } from "element-plus";
import { useContextStore } from "@/stores/context";
import { listAssessmentObjects } from "@/api/assessment";
import { listRuleModuleOptions, type RuleModuleOption } from "@/api/rules";
import { listVoteTasks, saveVoteDraft, submitVote } from "@/api/vote";
import type { AssessmentObjectItem } from "@/types/assessment";
import type { ScorePeriodCode } from "@/types/score";
import type { VoteGradeOption, VoteTaskItem, VoteTaskStatus } from "@/types/vote";
import { PERIOD_OPTIONS, formatTimestamp, toObjectNameMap } from "@/utils/assessment";

const periodOptions = PERIOD_OPTIONS;
const contextStore = useContextStore();

const objects = ref<AssessmentObjectItem[]>([]);
const voteModules = ref<RuleModuleOption[]>([]);
const tasks = ref<VoteTaskItem[]>([]);
const loadingTasks = ref(false);

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

const objectNameMap = computed(() => toObjectNameMap(objects.value));
const pendingCount = computed(() => tasks.value.filter((item) => item.status === "pending").length);
const completedCount = computed(() => tasks.value.filter((item) => item.status === "completed").length);

const voteDialogVisible = ref(false);
const voteFormRef = ref<FormInstance>();
const currentTask = ref<VoteTaskItem | null>(null);
const savingDraft = ref(false);
const submittingVote = ref(false);
const voteForm = reactive({
  gradeOption: "good" as VoteGradeOption,
  remark: "",
});

const voteFormRules: FormRules = {
  gradeOption: [{ required: true, message: "请选择投票档位", trigger: "change" }],
};

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
      mine: true,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "我的投票任务加载失败";
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

function openVoteDialog(task: VoteTaskItem): void {
  currentTask.value = task;
  voteForm.gradeOption = (task.gradeOption as VoteGradeOption) || "good";
  voteForm.remark = task.remark || "";
  voteDialogVisible.value = true;
}

async function handleSaveDraft(): Promise<void> {
  if (!currentTask.value) {
    return;
  }
  const valid = await voteFormRef.value?.validate().catch(() => false);
  if (!valid) {
    return;
  }
  savingDraft.value = true;
  try {
    await saveVoteDraft(currentTask.value.id, {
      gradeOption: voteForm.gradeOption,
      remark: voteForm.remark.trim() || undefined,
    });
    ElMessage.success("草稿已保存");
    voteDialogVisible.value = false;
    await loadTasks();
  } catch (error) {
    const message = error instanceof Error ? error.message : "草稿保存失败";
    ElMessage.error(message);
  } finally {
    savingDraft.value = false;
  }
}

async function handleSubmitVote(): Promise<void> {
  if (!currentTask.value) {
    return;
  }
  const valid = await voteFormRef.value?.validate().catch(() => false);
  if (!valid) {
    return;
  }
  try {
    await ElMessageBox.confirm("提交后将锁定该任务，无法再次修改。确认提交吗？", "提交确认", {
      type: "warning",
    });
  } catch (_error) {
    return;
  }
  submittingVote.value = true;
  try {
    await submitVote(currentTask.value.id, {
      gradeOption: voteForm.gradeOption,
      remark: voteForm.remark.trim() || undefined,
    });
    ElMessage.success("投票提交成功");
    voteDialogVisible.value = false;
    await loadTasks();
  } catch (error) {
    const message = error instanceof Error ? error.message : "投票提交失败";
    ElMessage.error(message);
  } finally {
    submittingVote.value = false;
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
.vote-execute-view {
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

.stats-alert {
  margin-bottom: 12px;
}

.vote-summary {
  margin-bottom: 16px;
}

.vote-form {
  margin-top: 12px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}
</style>
