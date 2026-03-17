<template>
  <div class="result-overview-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <strong>结果总览</strong>
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
              手动重算
            </el-button>
            <el-button :loading="loading" @click="loadOverview">刷新</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="!contextStore.yearId"
        title="请先选择考核年度"
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
          <el-statistic title="考核对象数" :value="summary.objectCount" />
        </el-col>
      </el-row>

      <div class="table-filter">
        <el-input v-model="keyword" clearable placeholder="按对象名称筛选" style="width: 260px" />
      </div>

      <el-table v-loading="loading" :data="filteredRows" border row-key="objectId">
        <el-table-column label="排名" width="100">
          <template #default="{ row }">{{ row.categoryRank ?? "-" }}</template>
        </el-table-column>
        <el-table-column prop="objectName" label="对象名称" min-width="200" />
        <el-table-column label="所属单位-部门" min-width="210">
          <template #default="{ row }">{{ row.unitDepartment || "-" }}</template>
        </el-table-column>
        <el-table-column label="考核总分" width="120">
          <template #default="{ row }">
            <strong>{{ formatFloat(row.finalScore, 2) }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="等第" width="120">
          <template #default="{ row }">{{ row.grade || "-" }}</template>
        </el-table-column>
        <el-table-column v-for="column in moduleColumns" :key="column.key" :label="column.label" width="130">
          <template #default="{ row }">
            {{ row.moduleScores[column.key] === null ? "-" : formatFloat(row.moduleScores[column.key] ?? 0, 2) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
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
import { listDepartments, listEmployees, listOrganizations } from "@/api/org";
import type { CalculatedScoreItem } from "@/types/calc";
import type {
  AssessmentObjectCategory,
  AssessmentObjectItem,
  AssessmentObjectType,
  GlobalAssessmentObjectCategory,
} from "@/types/assessment";
import type { DepartmentItem, EmployeeItem, OrganizationItem } from "@/types/org";
import type { ScorePeriodCode } from "@/types/score";
import { formatAssessmentYearLabel, formatFloat, periodDisplayLabel, periodStatusText } from "@/utils/assessment";

interface ObjectScoreRow {
  objectId: number;
  objectName: string;
  objectType?: AssessmentObjectType;
  objectCategory?: AssessmentObjectCategory;
  targetId?: number;
  targetType?: string;
  calculatedScoreId?: number;
  finalScore: number;
  categoryRank?: number;
  grade: string;
  unitDepartment: string;
  moduleScores: Record<string, number | null>;
}

interface OverviewSummary {
  objectCount: number;
}

interface ModuleColumn {
  key: string;
  label: string;
}

const appStore = useAppStore();
const contextStore = useContextStore();

const canRecalculate = computed(() => appStore.hasPermission("score:update"));
const loading = ref(false);
const recalculating = ref(false);
const keyword = ref("");
const rows = ref<ObjectScoreRow[]>([]);
const moduleColumns = ref<ModuleColumn[]>([]);

const summary = ref<OverviewSummary>({
  objectCount: 0,
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
  const periodText = periodDisplayLabel(contextStore.periodCode, contextStore.currentPeriod?.periodName);
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

function objectCategoryText(value: GlobalAssessmentObjectCategory): string {
  if (value === "all") {
    return "全部分类";
  }
  return assessmentCategoryLabel(value);
}

function emptySummary(): OverviewSummary {
  return {
    objectCount: 0,
  };
}

function createRowFromObject(item: AssessmentObjectItem): ObjectScoreRow {
  return {
    objectId: item.id,
    objectName: item.objectName,
    objectType: item.objectType,
    objectCategory: item.objectCategory,
    targetId: item.targetId,
    targetType: item.targetType,
    calculatedScoreId: undefined,
    finalScore: 0,
    categoryRank: undefined,
    grade: "-",
    unitDepartment: "-",
    moduleScores: {},
  };
}

function createFallbackRowFromScore(item: CalculatedScoreItem): ObjectScoreRow {
  return {
    objectId: item.objectId,
    objectName: item.objectName,
    objectType: item.objectType,
    objectCategory: item.objectCategory,
    targetId: undefined,
    targetType: undefined,
    calculatedScoreId: item.id,
    finalScore: item.finalScore,
    categoryRank: item.overallRank,
    grade: extractGradeFromDetail(item.detailJson),
    unitDepartment: "-",
    moduleScores: {},
  };
}

function extractGradeFromDetail(detailJson: string): string {
  if (!detailJson) {
    return "-";
  }
  try {
    const parsed = JSON.parse(detailJson) as Record<string, unknown>;
    const candidates = [
      parsed.grade,
      parsed.gradeName,
      parsed.gradeLevel,
      parsed.level,
      parsed.rankGrade,
    ];
    for (const item of candidates) {
      if (typeof item === "string" && item.trim()) {
        return item.trim();
      }
    }
    const nested = parsed.result;
    if (nested && typeof nested === "object") {
      const resultGrade = (nested as Record<string, unknown>).grade;
      if (typeof resultGrade === "string" && resultGrade.trim()) {
        return resultGrade.trim();
      }
    }
  } catch (_error) {
    return "-";
  }
  return "-";
}

function buildUnitDepartmentText(
  row: ObjectScoreRow,
  employeeMap: Map<number, EmployeeItem>,
  organizationMap: Map<number, OrganizationItem>,
  departmentMap: Map<number, DepartmentItem>,
): string {
  if (row.objectType !== "individual" || row.targetType !== "employee" || !row.targetId) {
    return "-";
  }
  const employee = employeeMap.get(row.targetId);
  if (!employee) {
    return "-";
  }

  const orgName = organizationMap.get(employee.organizationId)?.orgName ?? "-";
  if (!employee.departmentId) {
    return orgName;
  }
  const deptName = departmentMap.get(employee.departmentId)?.deptName;
  return deptName ? `${orgName}-${deptName}` : orgName;
}

function sortRows(items: ObjectScoreRow[]): ObjectScoreRow[] {
  return [...items].sort((a, b) => {
    const aRank = a.categoryRank;
    const bRank = b.categoryRank;
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
}

function resetOverview(): void {
  rows.value = [];
  moduleColumns.value = [];
  summary.value = emptySummary();
}

async function loadOverview(): Promise<void> {
  if (!contextStore.yearId) {
    resetOverview();
    return;
  }

  loading.value = true;
  try {
    const yearId = contextStore.yearId;
    const periodCode = contextStore.periodCode as ScorePeriodCode;
    const objectCategory = contextStore.objectCategory === "all" ? undefined : contextStore.objectCategory;

    const [objects, calculatedScores, rankings, orgResult, deptResult, employeeResult] = await Promise.allSettled([
      listAssessmentObjects(yearId),
      listCalculatedScores({ yearId, periodCode, objectCategory }),
      listRankings({ yearId, periodCode, scope: "overall", objectCategory }),
      listOrganizations({}),
      listDepartments({}),
      listEmployees({}),
    ]);

    if (objects.status !== "fulfilled" || calculatedScores.status !== "fulfilled" || rankings.status !== "fulfilled") {
      throw new Error("结果总览关键数据加载失败");
    }

    const orgFetch = orgResult.status === "fulfilled" ? orgResult.value : [];
    const deptFetch = deptResult.status === "fulfilled" ? deptResult.value : [];
    const employeeFetch = employeeResult.status === "fulfilled" ? employeeResult.value : [];

    const rankMap = new Map<number, number>();
    for (const item of rankings.value) {
      rankMap.set(item.objectId, item.rankNo);
    }

    const objectMap = new Map<number, AssessmentObjectItem>();
    const rowMap = new Map<number, ObjectScoreRow>();
    for (const item of objects.value) {
      objectMap.set(item.id, item);
      if (objectCategory && item.objectCategory !== objectCategory) {
        continue;
      }
      rowMap.set(item.id, createRowFromObject(item));
    }

    for (const item of calculatedScores.value) {
      if (objectCategory && item.objectCategory !== objectCategory) {
        continue;
      }
      const baseObject = objectMap.get(item.objectId);
      const row = rowMap.get(item.objectId) || createFallbackRowFromScore(item);
      row.objectName = item.objectName;
      row.objectType = item.objectType;
      row.objectCategory = item.objectCategory;
      row.calculatedScoreId = item.id;
      row.finalScore = item.finalScore;
      row.categoryRank = rankMap.get(item.objectId) ?? item.overallRank;
      row.grade = extractGradeFromDetail(item.detailJson);
      if (baseObject) {
        row.targetId = baseObject.targetId;
        row.targetType = baseObject.targetType;
      }
      rowMap.set(item.objectId, row);
    }

    const organizationMap = new Map<number, OrganizationItem>();
    for (const item of orgFetch) {
      organizationMap.set(item.id, item);
    }

    const departmentMap = new Map<number, DepartmentItem>();
    for (const item of deptFetch) {
      departmentMap.set(item.id, item);
    }

    const employeeMap = new Map<number, EmployeeItem>();
    for (const item of employeeFetch) {
      employeeMap.set(item.id, item);
    }

    const nextRows = Array.from(rowMap.values());
    for (const row of nextRows) {
      row.unitDepartment = buildUnitDepartmentText(row, employeeMap, organizationMap, departmentMap);
    }

    const moduleMetaMap = new Map<string, { sortOrder: number; moduleName: string }>();
    await Promise.all(
      nextRows.map(async (row) => {
        if (!row.calculatedScoreId) {
          return;
        }
        try {
          const modules = await listCalculatedModuleScores(row.calculatedScoreId);
          for (const module of modules) {
            row.moduleScores[module.moduleKey] = module.weightedScore;
            if (!moduleMetaMap.has(module.moduleKey)) {
              moduleMetaMap.set(module.moduleKey, {
                sortOrder: module.sortOrder,
                moduleName: module.moduleName,
              });
            }
          }
        } catch (_error) {
          // If one object's module details fail, keep base row data visible.
        }
      }),
    );

    const orderedModuleKeys = Array.from(moduleMetaMap.entries())
      .sort((a, b) => {
        if (a[1].sortOrder !== b[1].sortOrder) {
          return a[1].sortOrder - b[1].sortOrder;
        }
        return a[1].moduleName.localeCompare(b[1].moduleName, "zh-CN");
      })
      .map(([key]) => key);

    moduleColumns.value = orderedModuleKeys.map((key, index) => ({
      key,
      label: `模块${index + 1}分数`,
    }));

    for (const row of nextRows) {
      for (const key of orderedModuleKeys) {
        if (!(key in row.moduleScores)) {
          row.moduleScores[key] = null;
        }
      }
    }

    rows.value = sortRows(nextRows);
    summary.value = { objectCount: nextRows.length };
  } catch (error) {
    const message = error instanceof Error ? error.message : "结果总览加载失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

async function handleManualRecalculate(): Promise<void> {
  if (!contextStore.yearId) {
    ElMessage.warning("请先选择考核年度");
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
      `已完成重算：${result.calculatedObjects}/${result.totalObjects} 个对象，用时 ${result.durationMs}ms`,
    );
    await loadOverview();
  } catch (error) {
    const message = error instanceof Error ? error.message : "手动重算失败";
    ElMessage.error(message);
  } finally {
    recalculating.value = false;
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

.table-filter {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
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
