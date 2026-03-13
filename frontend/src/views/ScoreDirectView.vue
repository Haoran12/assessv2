<template>
  <div class="score-direct-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>M4 分数采集 - 直接录入</strong>
          <div class="header-actions">
            <el-button :loading="loadingRows" @click="loadRows">刷新</el-button>
            <el-button type="primary" :disabled="!canEdit" @click="openCreateDialog">单条录入</el-button>
            <el-button type="success" :disabled="!canEdit" @click="openBatchDialog">批量录入</el-button>
          </div>
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
          placeholder="直接录入模块"
          style="width: 260px"
          @change="loadRows"
        >
          <el-option
            v-for="item in directModules"
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
          @change="loadRows"
        >
          <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
        </el-select>
      </div>

      <el-alert
        v-if="filters.moduleId"
        :title="`当前模块分数范围：${scoreRangeText(filters.moduleId)}`"
        type="info"
        :closable="false"
        class="range-tip"
      />

      <el-table v-loading="loadingRows" :data="rows" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="模块" min-width="220">
          <template #default="{ row }">
            {{ moduleLabel(row.moduleId) }}
          </template>
        </el-table-column>
        <el-table-column label="考核对象" min-width="180">
          <template #default="{ row }">
            {{ objectNameMap[row.objectId] || `对象#${row.objectId}` }}
          </template>
        </el-table-column>
        <el-table-column prop="score" label="分数" width="120">
          <template #default="{ row }">{{ formatFloat(row.score) }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="180">
          <template #default="{ row }">{{ row.remark || "-" }}</template>
        </el-table-column>
        <el-table-column label="录入人" width="100">
          <template #default="{ row }">{{ row.inputBy }}</template>
        </el-table-column>
        <el-table-column label="录入时间" min-width="170">
          <template #default="{ row }">{{ formatTimestamp(row.inputAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="!canEdit" @click="openEditDialog(row)">编辑</el-button>
            <el-button link type="danger" :disabled="!canEdit" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="singleDialogVisible" :title="singleDialogTitle" width="560px">
      <el-form ref="singleFormRef" :model="singleForm" :rules="singleFormRules" label-width="110px">
        <el-form-item label="年度">
          <el-input :model-value="currentYearLabel" disabled />
        </el-form-item>
        <el-form-item label="周期">
          <el-input :model-value="singleForm.periodCode" disabled />
        </el-form-item>
        <el-form-item label="模块" prop="moduleId">
          <el-select
            v-model="singleForm.moduleId"
            filterable
            :disabled="singleMode === 'edit'"
            placeholder="请选择直接录入模块"
            style="width: 100%"
          >
            <el-option
              v-for="item in directModules"
              :key="item.id"
              :label="`${item.moduleName} (${item.moduleKey})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="考核对象" prop="objectId">
          <el-select
            v-model="singleForm.objectId"
            filterable
            :disabled="singleMode === 'edit'"
            placeholder="请选择考核对象"
            style="width: 100%"
          >
            <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="分数" prop="score">
          <el-input-number
            v-model="singleForm.score"
            :min="0"
            :max="singleFormMaxScore"
            :precision="6"
            controls-position="right"
          />
          <span class="score-hint">范围 {{ scoreRangeText(singleForm.moduleId) }}</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="singleForm.remark" type="textarea" :rows="3" maxlength="300" show-word-limit />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="singleDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="singleSubmitting" @click="submitSingleForm">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="batchDialogVisible" title="批量录入分数" width="860px">
      <el-form label-width="130px">
        <el-form-item label="年度 / 周期">
          <el-input :model-value="`${currentYearLabel} / ${contextPeriodCode}`" disabled />
        </el-form-item>
        <el-form-item label="模块">
          <el-select v-model="batchForm.moduleId" filterable placeholder="请选择模块" style="width: 100%">
            <el-option
              v-for="item in directModules"
              :key="item.id"
              :label="`${item.moduleName} (${item.moduleKey})`"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="覆盖重复记录">
          <el-switch v-model="batchForm.overwrite" />
          <span class="batch-tip">{{ batchForm.overwrite ? "已开启覆盖更新" : "重复对象将自动跳过" }}</span>
        </el-form-item>
      </el-form>

      <div class="batch-header">
        <strong>录入明细</strong>
        <el-button @click="appendBatchEntry">新增行</el-button>
      </div>

      <el-table :data="batchForm.entries" border>
        <el-table-column label="#" width="70">
          <template #default="{ $index }">{{ $index + 1 }}</template>
        </el-table-column>
        <el-table-column label="考核对象" min-width="260">
          <template #default="{ row }">
            <el-select v-model="row.objectId" filterable placeholder="请选择对象" style="width: 100%">
              <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="分数" width="190">
          <template #default="{ row }">
            <el-input-number
              v-model="row.score"
              :min="0"
              :max="batchModuleMaxScore"
              :precision="6"
              controls-position="right"
            />
          </template>
        </el-table-column>
        <el-table-column label="备注" min-width="220">
          <template #default="{ row }">
            <el-input v-model="row.remark" placeholder="可选" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="{ $index }">
            <el-button link type="danger" @click="removeBatchEntry($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchSubmitting" @click="submitBatchForm">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { listAssessmentObjects } from "@/api/assessment";
import { listRuleModuleOptions, type RuleModuleOption } from "@/api/rules";
import {
  batchUpsertDirectScores,
  createDirectScore,
  deleteDirectScore,
  listDirectScores,
  updateDirectScore,
} from "@/api/score";
import type { AssessmentObjectItem } from "@/types/assessment";
import type { ScorePeriodCode, DirectScoreItem } from "@/types/score";
import { PERIOD_OPTIONS, formatAssessmentYearLabel, formatFloat, formatTimestamp, toObjectNameMap } from "@/utils/assessment";

interface BatchEntryForm {
  objectId?: number;
  score: number;
  remark: string;
}

const appStore = useAppStore();
const contextStore = useContextStore();
const canEdit = computed(() => appStore.hasPermission("score:update"));
const periodOptions = PERIOD_OPTIONS;

const loadingRows = ref(false);
const objects = ref<AssessmentObjectItem[]>([]);
const directModules = ref<RuleModuleOption[]>([]);
const rows = ref<DirectScoreItem[]>([]);

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

const objectNameMap = computed(() => toObjectNameMap(objects.value));
const currentYearLabel = computed(() => {
  const hit = contextStore.currentYear;
  if (!hit) {
    return "-";
  }
  return formatAssessmentYearLabel(hit);
});

const singleDialogVisible = ref(false);
const singleSubmitting = ref(false);
const singleMode = ref<"create" | "edit">("create");
const singleFormRef = ref<FormInstance>();
const singleForm = reactive({
  id: 0,
  yearId: 0,
  periodCode: "Q1" as ScorePeriodCode,
  moduleId: undefined as number | undefined,
  objectId: undefined as number | undefined,
  score: 0,
  remark: "",
});

const batchDialogVisible = ref(false);
const batchSubmitting = ref(false);
const batchForm = reactive({
  moduleId: undefined as number | undefined,
  overwrite: false,
  entries: [] as BatchEntryForm[],
});

const singleDialogTitle = computed(() => (singleMode.value === "create" ? "单条录入分数" : "编辑录入分数"));
const singleFormMaxScore = computed(() => moduleMaxScore(singleForm.moduleId));
const batchModuleMaxScore = computed(() => moduleMaxScore(batchForm.moduleId));

const singleFormRules: FormRules = {
  moduleId: [{ required: true, message: "请选择模块", trigger: "change" }],
  objectId: [{ required: true, message: "请选择考核对象", trigger: "change" }],
  score: [
    {
      validator: (_rule, value: unknown, callback: (error?: Error) => void) => {
        const score = Number(value);
        if (!Number.isFinite(score)) {
          callback(new Error("请输入有效分数"));
          return;
        }
        const maxScore = singleFormMaxScore.value;
        if (score < 0 || score > maxScore) {
          callback(new Error(`分数范围必须在 0~${maxScore}`));
          return;
        }
        callback();
      },
      trigger: "blur",
    },
  ],
};

function moduleLabel(moduleId: number): string {
  const hit = directModules.value.find((item) => item.id === moduleId);
  if (!hit) {
    return `模块#${moduleId}`;
  }
  return `${hit.moduleName} (${hit.moduleKey})`;
}

function moduleMaxScore(moduleId?: number): number {
  if (!moduleId) {
    return 100;
  }
  const hit = directModules.value.find((item) => item.id === moduleId);
  if (hit?.maxScore && hit.maxScore > 0) {
    return hit.maxScore;
  }
  return 100;
}

function scoreRangeText(moduleId?: number): string {
  return `0 ~ ${formatFloat(moduleMaxScore(moduleId), 2)}`;
}

function defaultBatchEntry(): BatchEntryForm {
  return {
    objectId: undefined,
    score: 0,
    remark: "",
  };
}

async function loadObjectsForYear(yearId: number): Promise<void> {
  objects.value = await listAssessmentObjects(yearId);
  if (filters.objectId && !objects.value.some((item) => item.id === filters.objectId)) {
    filters.objectId = undefined;
  }
}

async function loadDirectModules(): Promise<void> {
  if (!contextStore.yearId) {
    directModules.value = [];
    return;
  }
  directModules.value = await listRuleModuleOptions(
    {
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode,
    },
    ["direct"],
  );
  if (filters.moduleId && !directModules.value.some((item) => item.id === filters.moduleId)) {
    filters.moduleId = undefined;
  }
}

async function loadRows(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    return;
  }
  loadingRows.value = true;
  try {
    rows.value = await listDirectScores({
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode,
      moduleId: filters.moduleId,
      objectId: filters.objectId,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "直接录入列表加载失败";
    ElMessage.error(message);
  } finally {
    loadingRows.value = false;
  }
}

async function handleContextChanged(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    directModules.value = [];
    objects.value = [];
    return;
  }
  try {
    await Promise.all([loadObjectsForYear(contextStore.yearId), loadDirectModules()]);
    await loadRows();
  } catch (error) {
    const message = error instanceof Error ? error.message : "上下文加载失败";
    ElMessage.error(message);
  }
}

function openCreateDialog(): void {
  if (!contextStore.yearId) {
    ElMessage.warning("请先选择年度");
    return;
  }
  singleMode.value = "create";
  singleForm.id = 0;
  singleForm.yearId = contextStore.yearId;
  singleForm.periodCode = contextStore.periodCode as ScorePeriodCode;
  singleForm.moduleId = filters.moduleId;
  singleForm.objectId = filters.objectId;
  singleForm.score = 0;
  singleForm.remark = "";
  singleDialogVisible.value = true;
}

function openEditDialog(row: DirectScoreItem): void {
  if (!contextStore.yearId) {
    return;
  }
  singleMode.value = "edit";
  singleForm.id = row.id;
  singleForm.yearId = row.yearId;
  singleForm.periodCode = row.periodCode;
  singleForm.moduleId = row.moduleId;
  singleForm.objectId = row.objectId;
  singleForm.score = row.score;
  singleForm.remark = row.remark || "";
  singleDialogVisible.value = true;
}

async function submitSingleForm(): Promise<void> {
  if (!singleFormRef.value) {
    return;
  }
  const valid = await singleFormRef.value.validate().catch(() => false);
  if (!valid) {
    return;
  }
  singleSubmitting.value = true;
  try {
    if (singleMode.value === "create") {
      await createDirectScore({
        yearId: singleForm.yearId,
        periodCode: singleForm.periodCode,
        moduleId: singleForm.moduleId!,
        objectId: singleForm.objectId!,
        score: Number(singleForm.score),
        remark: singleForm.remark.trim() || undefined,
      });
      ElMessage.success("单条录入成功");
    } else {
      await updateDirectScore(singleForm.id, {
        score: Number(singleForm.score),
        remark: singleForm.remark.trim() || undefined,
      });
      ElMessage.success("分数更新成功");
    }
    singleDialogVisible.value = false;
    await loadRows();
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存失败";
    ElMessage.error(message);
  } finally {
    singleSubmitting.value = false;
  }
}

function openBatchDialog(): void {
  if (!contextStore.yearId) {
    ElMessage.warning("请先选择年度");
    return;
  }
  batchForm.moduleId = filters.moduleId;
  batchForm.overwrite = false;
  batchForm.entries = [defaultBatchEntry()];
  batchDialogVisible.value = true;
}

function appendBatchEntry(): void {
  batchForm.entries.push(defaultBatchEntry());
}

function removeBatchEntry(index: number): void {
  batchForm.entries.splice(index, 1);
  if (batchForm.entries.length === 0) {
    batchForm.entries.push(defaultBatchEntry());
  }
}

function validateBatchForm(): string | null {
  if (!contextStore.yearId) {
    return "年度不能为空";
  }
  if (!batchForm.moduleId) {
    return "请选择模块";
  }
  if (batchForm.entries.length === 0) {
    return "请至少添加一条明细";
  }
  const objectSet = new Set<number>();
  const maxScore = batchModuleMaxScore.value;
  for (const [index, item] of batchForm.entries.entries()) {
    if (!item.objectId) {
      return `第 ${index + 1} 行请选择考核对象`;
    }
    if (objectSet.has(item.objectId)) {
      return `第 ${index + 1} 行对象重复`;
    }
    objectSet.add(item.objectId);
    if (!Number.isFinite(item.score) || item.score < 0 || item.score > maxScore) {
      return `第 ${index + 1} 行分数必须在 0~${maxScore}`;
    }
  }
  return null;
}

async function submitBatchForm(): Promise<void> {
  const validationError = validateBatchForm();
  if (validationError) {
    ElMessage.warning(validationError);
    return;
  }
  batchSubmitting.value = true;
  try {
    const result = await batchUpsertDirectScores({
      yearId: contextStore.yearId!,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      moduleId: batchForm.moduleId!,
      overwrite: batchForm.overwrite,
      entries: batchForm.entries.map((item) => ({
        objectId: item.objectId!,
        score: Number(item.score),
        remark: item.remark.trim() || undefined,
      })),
    });
    ElMessage.success(`批量录入完成：新增 ${result.created}，更新 ${result.updated}，跳过 ${result.skipped}`);
    batchDialogVisible.value = false;
    await loadRows();
  } catch (error) {
    const message = error instanceof Error ? error.message : "批量录入失败";
    ElMessage.error(message);
  } finally {
    batchSubmitting.value = false;
  }
}

async function handleDelete(scoreId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("确认删除该条直接录入记录吗？", "删除确认", { type: "warning" });
    await deleteDirectScore(scoreId);
    ElMessage.success("删除成功");
    await loadRows();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "删除失败";
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
.score-direct-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 12px;
}

.range-tip {
  margin-bottom: 12px;
}

.score-hint {
  margin-left: 10px;
  color: #909399;
  font-size: 12px;
}

.batch-tip {
  margin-left: 10px;
  color: #909399;
}

.batch-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
