<template>
  <div class="assessment-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>年度管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingYears" @click="loadYears">刷新</el-button>
            <el-button :disabled="!canEdit" @click="openTemplateDialog">周期模板</el-button>
            <el-button type="primary" :disabled="!canEdit" @click="openCreateYearDialog">创建年度</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingYears" :data="years" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="year" label="年度" width="100" />
        <el-table-column label="开始日期" width="120">
          <template #default="{ row }">{{ dateText(row.startDate) }}</template>
        </el-table-column>
        <el-table-column label="结束日期" width="120">
          <template #default="{ row }">{{ dateText(row.endDate) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="yearStatusTagType(row.status)">{{ yearStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="说明" min-width="180">
          <template #default="{ row }">{{ row.description || "-" }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="220" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button link type="primary" @click="selectYear(row)">查看周期</el-button>
              <el-dropdown
                v-if="canEdit && availableYearTransitions(row.status).length > 0"
                @command="(command) => handleYearStatusChange(row, String(command))"
              >
                <span class="status-trigger">状态流转</span>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="status in availableYearTransitions(row.status)"
                      :key="`${row.id}-${status}`"
                      :command="status"
                    >
                      设为{{ yearStatusText(status) }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <strong>
            周期管理
            <span class="subtitle" v-if="selectedYear">- {{ selectedYear.year }} 年</span>
          </strong>
          <el-button :disabled="!selectedYear" :loading="loadingPeriods" @click="reloadCurrentYearData">
            刷新
          </el-button>
        </div>
      </template>

      <el-empty v-if="!selectedYear" description="请选择一个年度查看周期" />
      <el-table v-else v-loading="loadingPeriods" :data="periods" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="periodCode" label="周期编码" width="120" />
        <el-table-column label="周期名称" min-width="160">
          <template #default="{ row }">{{ periodDisplayLabel(row.periodCode, row.periodName) }}</template>
        </el-table-column>
        <el-table-column label="开始日期" width="120">
          <template #default="{ row }">{{ dateText(row.startDate) }}</template>
        </el-table-column>
        <el-table-column label="结束日期" width="120">
          <template #default="{ row }">{{ dateText(row.endDate) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="periodStatusTagType(row.status)">{{ periodStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="180" fixed="right">
          <template #default="{ row }">
            <el-dropdown
              v-if="canEdit && availablePeriodTransitions(row.status).length > 0"
              @command="(command) => handlePeriodStatusChange(row, String(command))"
            >
              <span class="status-trigger">状态流转</span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-for="status in availablePeriodTransitions(row.status)"
                    :key="`${row.id}-${status}`"
                    :command="status"
                  >
                    设为{{ periodStatusText(status) }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <span v-else class="status-disabled">当前无可用流转</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="templateDialogVisible" width="760px" title="周期模板设置">
      <el-alert
        title="模板仅影响后续新建年度；已创建年度的周期不会自动变更。"
        type="info"
        :closable="false"
        class="template-alert"
      />
      <el-table :data="periodTemplates" border>
        <el-table-column type="index" label="#" width="60" />
        <el-table-column label="周期编码" width="140">
          <template #default="{ row }">
            <el-input v-model="row.periodCode" placeholder="例如 M01 / Q1" />
          </template>
        </el-table-column>
        <el-table-column label="周期名称" min-width="150">
          <template #default="{ row }">
            <el-input v-model="row.periodName" placeholder="例如 1月 / 一季度" />
          </template>
        </el-table-column>
        <el-table-column label="开始(月-日)" width="140">
          <template #default="{ row }">
            <el-input v-model="row.startDay" placeholder="MM-DD" />
          </template>
        </el-table-column>
        <el-table-column label="结束(月-日)" width="140">
          <template #default="{ row }">
            <el-input v-model="row.endDay" placeholder="MM-DD" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ $index }">
            <el-button link type="danger" @click="removeTemplateRow($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="template-actions">
        <el-button @click="addTemplateRow">新增周期</el-button>
      </div>
      <template #footer>
        <el-button @click="templateDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="templateSaving" @click="savePeriodTemplates">保存模板</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="createYearDialogVisible" width="620px" title="创建考核年度">
      <el-form label-width="110px">
        <el-form-item label="年度" required>
          <el-input-number v-model="createYearForm.year" :min="2000" :max="9999" controls-position="right" />
        </el-form-item>
        <el-form-item label="开始日期">
          <el-date-picker
            v-model="createYearForm.startDate"
            type="date"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="结束日期">
          <el-date-picker
            v-model="createYearForm.endDate"
            type="date"
            value-format="YYYY-MM-DD"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="复制上年对象">
          <el-select v-model="createYearForm.copyFromYearId" clearable filterable style="width: 100%">
            <el-option
              v-for="year in copyFromYearOptions"
              :key="year.id"
              :label="formatAssessmentYearLabel(year)"
              :value="year.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="createYearForm.description"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createYearDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingYear" @click="submitCreateYear">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { useUnsavedStore } from "@/stores/unsaved";
import {
  createAssessmentYear,
  listAssessmentPeriodTemplates,
  listAssessmentPeriods,
  listAssessmentYears,
  updateAssessmentPeriodTemplates,
  updateAssessmentPeriodStatus,
  updateAssessmentYearStatus,
} from "@/api/assessment";
import type {
  AssessmentPeriodItem,
  AssessmentPeriodStatus,
  AssessmentPeriodTemplateItem,
  AssessmentYearItem,
  AssessmentYearStatus,
} from "@/types/assessment";
import { formatAssessmentYearLabel, periodDisplayLabel } from "@/utils/assessment";

const appStore = useAppStore();
const contextStore = useContextStore();
const unsavedStore = useUnsavedStore();
const canEdit = computed(() => appStore.hasPermission("assessment:update"));
const dirtySourceId = "assessment:create-year";

const loadingYears = ref(false);
const years = ref<AssessmentYearItem[]>([]);
const selectedYear = ref<AssessmentYearItem | null>(null);

const loadingPeriods = ref(false);
const periods = ref<AssessmentPeriodItem[]>([]);
const templateDialogVisible = ref(false);
const templateSaving = ref(false);
const periodTemplates = ref<AssessmentPeriodTemplateItem[]>([]);

const createYearDialogVisible = ref(false);
const creatingYear = ref(false);
const createYearForm = reactive({
  year: new Date().getFullYear(),
  startDate: "",
  endDate: "",
  copyFromYearId: undefined as number | undefined,
  description: "",
});
const createYearBaseline = ref("");

const copyFromYearOptions = computed(() =>
  years.value.filter((item) => item.year !== createYearForm.year),
);

function createYearFormSignature(): string {
  return JSON.stringify({
    year: createYearForm.year,
    startDate: createYearForm.startDate,
    endDate: createYearForm.endDate,
    copyFromYearId: createYearForm.copyFromYearId,
    description: createYearForm.description,
  });
}

function resetCreateYearBaseline(): void {
  createYearBaseline.value = createYearFormSignature();
  unsavedStore.clearDirty(dirtySourceId);
}

function dateText(value?: string): string {
  if (!value) {
    return "-";
  }
  if (value.includes("T")) {
    return value.slice(0, 10);
  }
  return value;
}

function yearStatusText(status: AssessmentYearStatus): string {
  switch (status) {
    case "preparing":
      return "筹备中";
    case "active":
      return "进行中";
    case "completed":
      return "已完成";
    default:
      return status;
  }
}

function yearStatusTagType(status: AssessmentYearStatus): "info" | "warning" | "success" {
  switch (status) {
    case "preparing":
      return "info";
    case "active":
      return "warning";
    case "completed":
      return "success";
    default:
      return "info";
  }
}

function periodStatusText(status: AssessmentPeriodStatus): string {
  switch (status) {
    case "preparing":
      return "筹备中";
    case "active":
      return "进行中";
    case "completed":
      return "已完成";
    default:
      return status;
  }
}

function periodStatusTagType(status: AssessmentPeriodStatus): "info" | "warning" | "success" {
  switch (status) {
    case "preparing":
      return "info";
    case "active":
      return "warning";
    case "completed":
      return "success";
    default:
      return "info";
  }
}

function availableYearTransitions(status: AssessmentYearStatus): AssessmentYearStatus[] {
  return ["preparing", "active", "completed"].filter((item) => item !== status) as AssessmentYearStatus[];
}

function availablePeriodTransitions(status: AssessmentPeriodStatus): AssessmentPeriodStatus[] {
  return ["preparing", "active", "completed"].filter((item) => item !== status) as AssessmentPeriodStatus[];
}

async function loadPeriodTemplates(): Promise<void> {
  try {
    periodTemplates.value = await listAssessmentPeriodTemplates();
  } catch (_error) {
    ElMessage.error("周期模板加载失败");
  }
}

function openTemplateDialog(): void {
  if (!canEdit.value) {
    return;
  }
  if (periodTemplates.value.length === 0) {
    void loadPeriodTemplates();
  }
  templateDialogVisible.value = true;
}

function addTemplateRow(): void {
  periodTemplates.value.push({
    periodCode: "",
    periodName: "",
    startDay: "",
    endDay: "",
    sortOrder: periodTemplates.value.length + 1,
  });
}

function removeTemplateRow(index: number): void {
  periodTemplates.value.splice(index, 1);
}

async function savePeriodTemplates(): Promise<void> {
  if (periodTemplates.value.length === 0) {
    ElMessage.warning("请至少保留一个周期模板");
    return;
  }

  const normalized = periodTemplates.value.map((item, index) => ({
    periodCode: item.periodCode.trim().toUpperCase(),
    periodName: item.periodName.trim(),
    startDay: item.startDay?.trim() || undefined,
    endDay: item.endDay?.trim() || undefined,
    sortOrder: index + 1,
  }));

  if (normalized.some((item) => !item.periodCode || !item.periodName)) {
    ElMessage.warning("周期编码和周期名称不能为空");
    return;
  }

  templateSaving.value = true;
  try {
    periodTemplates.value = await updateAssessmentPeriodTemplates(normalized);
    ElMessage.success("周期模板已保存，将在新建年度时生效");
    templateDialogVisible.value = false;
  } catch (error) {
    const message = error instanceof Error ? error.message : "周期模板保存失败";
    ElMessage.error(message);
  } finally {
    templateSaving.value = false;
  }
}

async function loadYears(): Promise<void> {
  loadingYears.value = true;
  try {
    years.value = await listAssessmentYears();
    if (!selectedYear.value && years.value.length > 0) {
      const preferred = years.value.find((item) => item.id === contextStore.yearId);
      await selectYear(preferred ?? years.value[0]);
      return;
    }

    if (selectedYear.value) {
      const latest = years.value.find((item) => item.id === selectedYear.value?.id) ?? null;
      selectedYear.value = latest;
      if (!latest && years.value.length > 0) {
        await selectYear(years.value[0]);
      }
    }
  } catch (_error) {
    ElMessage.error("年度列表加载失败");
  } finally {
    loadingYears.value = false;
  }
}

async function loadPeriods(yearId: number): Promise<void> {
  loadingPeriods.value = true;
  try {
    periods.value = await listAssessmentPeriods(yearId);
  } catch (_error) {
    ElMessage.error("周期列表加载失败");
  } finally {
    loadingPeriods.value = false;
  }
}

async function selectYear(row: AssessmentYearItem): Promise<void> {
  selectedYear.value = row;
  await loadPeriods(row.id);
  if (contextStore.yearId !== row.id) {
    try {
      await contextStore.setYear(row.id);
    } catch (_error) {
      ElMessage.error("全局年度切换失败");
    }
  }
}

async function reloadCurrentYearData(): Promise<void> {
  if (!selectedYear.value) {
    return;
  }
  await loadPeriods(selectedYear.value.id);
}

function openCreateYearDialog(): void {
  createYearForm.year = new Date().getFullYear();
  createYearForm.startDate = "";
  createYearForm.endDate = "";
  createYearForm.copyFromYearId = undefined;
  createYearForm.description = "";
  resetCreateYearBaseline();
  createYearDialogVisible.value = true;
}

async function submitCreateYear(): Promise<void> {
  if (!createYearForm.year || createYearForm.year < 2000 || createYearForm.year > 9999) {
    ElMessage.warning("请填写有效年度");
    return;
  }
  if (createYearForm.startDate && createYearForm.endDate && createYearForm.startDate > createYearForm.endDate) {
    ElMessage.warning("开始日期不能晚于结束日期");
    return;
  }

  creatingYear.value = true;
  try {
    const result = await createAssessmentYear({
      year: createYearForm.year,
      startDate: createYearForm.startDate || undefined,
      endDate: createYearForm.endDate || undefined,
      copyFromYearId: createYearForm.copyFromYearId,
      description: createYearForm.description.trim() || undefined,
    });
    ElMessage.success(`年度创建成功，自动生成 ${result.periods.length} 个周期`);
    createYearDialogVisible.value = false;
    await contextStore.ensureInitialized(true);
    await contextStore.setYear(result.year.id);
    await loadYears();
    const latest = years.value.find((item) => item.id === result.year.id) ?? result.year;
    await selectYear(latest);
  } catch (error) {
    const message = error instanceof Error ? error.message : "创建年度失败";
    ElMessage.error(message);
  } finally {
    creatingYear.value = false;
  }
}

async function handleYearStatusChange(row: AssessmentYearItem, statusRaw: string): Promise<void> {
  const status = statusRaw as AssessmentYearStatus;
  try {
    await ElMessageBox.confirm(
      `确认将 ${row.year} 年状态切换为「${yearStatusText(status)}」吗？`,
      "状态确认",
      { type: "warning" },
    );
    await updateAssessmentYearStatus(row.id, status);
    ElMessage.success("年度状态已更新");
    await loadYears();
    if (selectedYear.value?.id === row.id) {
      const latest = years.value.find((item) => item.id === row.id);
      if (latest) {
        selectedYear.value = latest;
      }
    }
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "年度状态更新失败";
    ElMessage.error(message);
  }
}

async function handlePeriodStatusChange(row: AssessmentPeriodItem, statusRaw: string): Promise<void> {
  const status = statusRaw as AssessmentPeriodStatus;
  try {
    await ElMessageBox.confirm(
      `确认将 ${periodDisplayLabel(row.periodCode, row.periodName)} 状态切换为「${periodStatusText(status)}」吗？`,
      "状态确认",
      { type: "warning" },
    );
    await updateAssessmentPeriodStatus(row.id, status);
    ElMessage.success("周期状态已更新");
    await reloadCurrentYearData();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "周期状态更新失败";
    ElMessage.error(message);
  }
}

watch(
  () => contextStore.yearId,
  async (yearId) => {
    if (!yearId) {
      selectedYear.value = null;
      periods.value = [];
      return;
    }

    if (selectedYear.value?.id === yearId) {
      return;
    }

    const hit = years.value.find((item) => item.id === yearId);
    if (hit) {
      await selectYear(hit);
      return;
    }

    await loadYears();
  },
);

onMounted(async () => {
  unsavedStore.setSourceMeta(dirtySourceId, {
    label: "考核年度创建",
    save: submitCreateYear,
  });
  await contextStore.ensureInitialized();
  await Promise.all([
    loadYears(),
    canEdit.value ? loadPeriodTemplates() : Promise.resolve(),
  ]);
});

watch(createYearDialogVisible, (visible) => {
  if (visible) {
    resetCreateYearBaseline();
    return;
  }
  createYearBaseline.value = "";
  unsavedStore.clearDirty(dirtySourceId);
});

watch(
  createYearForm,
  () => {
    if (!createYearDialogVisible.value) {
      unsavedStore.clearDirty(dirtySourceId);
      return;
    }
    const current = createYearFormSignature();
    if (!createYearBaseline.value || current === createYearBaseline.value) {
      unsavedStore.clearDirty(dirtySourceId);
      return;
    }
    unsavedStore.markDirty(dirtySourceId);
  },
  { deep: true },
);

onBeforeUnmount(() => {
  unsavedStore.unregisterSource(dirtySourceId);
});
</script>

<style scoped>
.assessment-view {
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

.template-alert {
  margin-bottom: 12px;
}

.template-actions {
  margin-top: 10px;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-trigger {
  color: #409eff;
  cursor: pointer;
}

.status-disabled {
  color: #909399;
}

.subtitle {
  margin-left: 6px;
  color: #606266;
  font-size: 13px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
