<template>
  <div class="assessment-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>年度管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingYears" @click="loadYears">刷新</el-button>
            <el-button type="primary" :disabled="!canEdit" @click="openCreateYearDialog">创建年度</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingYears" :data="years" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="year" label="年度" width="100" />
        <el-table-column prop="yearName" label="年度名称" min-width="180" />
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
        <el-table-column label="操作" min-width="200" fixed="right">
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
        <el-table-column prop="periodName" label="周期名称" min-width="160" />
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
            <span v-else class="status-disabled">已锁定或无可用流转</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <strong>
            考核对象
            <span class="subtitle" v-if="selectedYear">- 共 {{ filteredObjects.length }} 条</span>
          </strong>
        </div>
      </template>

      <div class="objects-filter">
        <el-select v-model="objectFilter.objectType" clearable placeholder="对象类型">
          <el-option label="团体" value="team" />
          <el-option label="个人" value="individual" />
        </el-select>
        <el-select v-model="objectFilter.objectCategory" clearable placeholder="对象分类">
          <el-option v-for="item in objectCategoryOptions" :key="item" :label="assessmentCategoryLabel(item)" :value="item" />
        </el-select>
        <el-input v-model="objectFilter.keyword" clearable placeholder="按对象名称搜索" />
      </div>

      <el-empty v-if="!selectedYear" description="请选择年度后查看考核对象" />
      <el-table v-else v-loading="loadingObjects" :data="filteredObjects" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="objectName" label="对象名称" min-width="180" />
        <el-table-column prop="objectType" label="对象类型" width="110" />
        <el-table-column label="对象分类" min-width="140">
          <template #default="{ row }">{{ assessmentCategoryLabel(row.objectCategory) }}</template>
        </el-table-column>
        <el-table-column prop="targetType" label="目标类型" width="110" />
        <el-table-column prop="targetId" label="目标编号" width="100" />
        <el-table-column prop="parentObjectId" label="上级对象编号" width="110">
          <template #default="{ row }">{{ row.parentObjectId || "-" }}</template>
        </el-table-column>
        <el-table-column label="是否参与" width="100">
          <template #default="{ row }">
            <el-tag :type="row.isActive ? 'success' : 'info'">{{ row.isActive ? "是" : "否" }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createYearDialogVisible" width="620px" title="创建考核年度">
      <el-form label-width="110px">
        <el-form-item label="年度" required>
          <el-input-number v-model="createYearForm.year" :min="2000" :max="9999" controls-position="right" />
        </el-form-item>
        <el-form-item label="年度名称">
          <el-input v-model="createYearForm.yearName" placeholder="可留空，系统自动生成" />
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
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import {
  createAssessmentYear,
  listAssessmentObjects,
  listAssessmentPeriods,
  listAssessmentYears,
  updateAssessmentPeriodStatus,
  updateAssessmentYearStatus,
} from "@/api/assessment";
import type {
  AssessmentObjectItem,
  AssessmentPeriodItem,
  AssessmentPeriodStatus,
  AssessmentYearItem,
  AssessmentYearStatus,
} from "@/types/assessment";
import { formatAssessmentYearLabel } from "@/utils/assessment";
import { assessmentCategoryLabel } from "@/constants/assessmentCategories";

const appStore = useAppStore();
const contextStore = useContextStore();
const canEdit = computed(() => appStore.hasPermission("assessment:update"));

const loadingYears = ref(false);
const years = ref<AssessmentYearItem[]>([]);
const selectedYear = ref<AssessmentYearItem | null>(null);

const loadingPeriods = ref(false);
const periods = ref<AssessmentPeriodItem[]>([]);

const loadingObjects = ref(false);
const objects = ref<AssessmentObjectItem[]>([]);

const objectFilter = reactive({
  objectType: "",
  objectCategory: "",
  keyword: "",
});

const createYearDialogVisible = ref(false);
const creatingYear = ref(false);
const createYearForm = reactive({
  year: new Date().getFullYear(),
  yearName: "",
  startDate: "",
  endDate: "",
  copyFromYearId: undefined as number | undefined,
  description: "",
});

const copyFromYearOptions = computed(() =>
  years.value.filter((item) => item.year !== createYearForm.year),
);

const objectCategoryOptions = computed(() => {
  const set = new Set<string>();
  for (const item of objects.value) {
    set.add(item.objectCategory);
  }
  return Array.from(set.values());
});

const filteredObjects = computed(() => {
  return objects.value.filter((item) => {
    if (objectFilter.objectType && item.objectType !== objectFilter.objectType) {
      return false;
    }
    if (objectFilter.objectCategory && item.objectCategory !== objectFilter.objectCategory) {
      return false;
    }
    if (objectFilter.keyword.trim()) {
      const kw = objectFilter.keyword.trim().toLowerCase();
      if (!item.objectName.toLowerCase().includes(kw)) {
        return false;
      }
    }
    return true;
  });
});

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
    case "ended":
      return "已结束";
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
    case "ended":
      return "success";
    default:
      return "info";
  }
}

function periodStatusText(status: AssessmentPeriodStatus): string {
  switch (status) {
    case "not_started":
      return "未开始";
    case "active":
      return "进行中";
    case "ended":
      return "已结束";
    case "locked":
      return "已锁定";
    default:
      return status;
  }
}

function periodStatusTagType(status: AssessmentPeriodStatus): "info" | "warning" | "success" | "danger" {
  switch (status) {
    case "not_started":
      return "info";
    case "active":
      return "warning";
    case "ended":
      return "success";
    case "locked":
      return "danger";
    default:
      return "info";
  }
}

function availableYearTransitions(status: AssessmentYearStatus): AssessmentYearStatus[] {
  switch (status) {
    case "preparing":
      return ["active"];
    case "active":
      return ["ended"];
    default:
      return [];
  }
}

function availablePeriodTransitions(status: AssessmentPeriodStatus): AssessmentPeriodStatus[] {
  switch (status) {
    case "not_started":
      return ["active", "locked"];
    case "active":
      return ["ended", "locked"];
    case "ended":
      return ["locked"];
    default:
      return [];
  }
}

async function loadYears(): Promise<void> {
  loadingYears.value = true;
  try {
    years.value = await listAssessmentYears();
    if (!selectedYear.value && years.value.length > 0) {
      const preferred = years.value.find((item) => item.id === contextStore.yearId);
      await selectYear(preferred ?? years.value[0]);
    } else if (selectedYear.value) {
      const latest = years.value.find((item) => item.id === selectedYear.value?.id) || null;
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

async function loadObjects(yearId: number): Promise<void> {
  loadingObjects.value = true;
  try {
    objects.value = await listAssessmentObjects(yearId);
  } catch (_error) {
    ElMessage.error("考核对象加载失败");
  } finally {
    loadingObjects.value = false;
  }
}

async function selectYear(row: AssessmentYearItem): Promise<void> {
  selectedYear.value = row;
  await Promise.all([loadPeriods(row.id), loadObjects(row.id)]);
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
  await Promise.all([loadPeriods(selectedYear.value.id), loadObjects(selectedYear.value.id)]);
}

function openCreateYearDialog(): void {
  createYearForm.year = new Date().getFullYear();
  createYearForm.yearName = "";
  createYearForm.startDate = "";
  createYearForm.endDate = "";
  createYearForm.copyFromYearId = undefined;
  createYearForm.description = "";
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
      yearName: createYearForm.yearName.trim() || undefined,
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
      `确认将 ${row.periodCode} 状态切换为「${periodStatusText(status)}」吗？`,
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
      objects.value = [];
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
  await contextStore.ensureInitialized();
  await loadYears();
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

.objects-filter {
  display: grid;
  grid-template-columns: 180px 180px 1fr;
  gap: 12px;
  margin-bottom: 12px;
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

  .objects-filter {
    grid-template-columns: 1fr;
  }
}
</style>
