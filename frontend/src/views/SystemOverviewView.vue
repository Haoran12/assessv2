<template>
  <div class="overview-view">
    <el-card>
      <template #header>
        <strong>系统概览</strong>
      </template>

      <div class="summary-grid">
        <div class="summary-item">
          <span class="summary-label">用户</span>
          <span class="summary-value">{{ appStore.username || "-" }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">角色</span>
          <span class="summary-value">{{ appStore.primaryRole || "-" }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">场次</span>
          <span class="summary-value">{{ contextStore.currentSession?.displayName || "未选择" }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">组织</span>
          <span class="summary-value">{{ contextStore.currentSession?.organizationName || "未选择" }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">周期</span>
          <span class="summary-value">{{ contextStore.currentPeriod?.periodName || "未选择" }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">对象分组</span>
          <span class="summary-value">{{ contextStore.currentObjectGroup?.groupName || "未选择" }}</span>
        </div>
      </div>

      <el-alert
        class="mt-12"
        title="当前版本已按“考核场次独立”架构运行，规则绑定与对象分组均基于所选场次生效。"
        type="info"
        :closable="false"
      />
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <strong>当前考核数据</strong>
          <el-button size="small" :loading="loadingTable" @click="loadAssessmentTableData">刷新</el-button>
        </div>
      </template>

      <el-alert
        v-if="!isContextReady"
        title="请先在顶部选择完整的考核场次、周期和对象分组。"
        type="warning"
        :closable="false"
      />
      <template v-else>
        <div class="table-caption">
          {{ contextLabel }}
        </div>
        <el-table :data="assessmentRows" border stripe v-loading="loadingTable">
          <el-table-column prop="rank" label="排名" width="88" />
          <el-table-column prop="objectName" label="考核对象名称" min-width="220" />
          <el-table-column label="总分" width="120">
            <template #default="{ row }">
              {{ formatScore(row.totalScore) }}
            </template>
          </el-table-column>
          <el-table-column prop="grade" label="等第" width="120" />
          <el-table-column
            v-for="module in moduleColumns"
            :key="module.moduleKey"
            :label="module.moduleName"
            min-width="140"
          >
            <template #default="{ row }">
              {{ formatScore(row.moduleScores[module.moduleKey]) }}
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!loadingTable && assessmentRows.length === 0" description="当前分组暂无可展示的考核对象" />
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { listAssessmentSessionObjects } from "@/api/assessment";
import { listRuleFiles } from "@/api/rules";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import type { AssessmentSessionObjectItem } from "@/types/assessment";
import type { RuleFileItem } from "@/types/rules";

interface TableModuleColumn {
  moduleKey: string;
  moduleName: string;
}

interface TableRow {
  rank: number;
  objectName: string;
  totalScore: number | null;
  grade: string;
  moduleScores: Record<string, number | null>;
}

const appStore = useAppStore();
const contextStore = useContextStore();
const moduleColumns = ref<TableModuleColumn[]>([]);
const assessmentRows = ref<TableRow[]>([]);
const loadingTable = ref(false);
let fetchSequence = 0;

const isContextReady = computed(() =>
  Boolean(contextStore.sessionId && contextStore.periodCode && contextStore.objectGroupCode),
);

const contextLabel = computed(() => {
  const sessionName = contextStore.currentSession?.displayName || "未选择场次";
  const periodName = contextStore.currentPeriod?.periodName || "未选择周期";
  const groupName = contextStore.currentObjectGroup?.groupName || "未选择分组";
  return `场次：${sessionName} ｜ 周期：${periodName} ｜ 对象分组：${groupName}`;
});

function toNumberOrNull(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return null;
}

function formatScore(value: number | null): string {
  if (value === null || !Number.isFinite(value)) {
    return "-";
  }
  return value.toFixed(2);
}

function normalizeScoreModules(raw: unknown): TableModuleColumn[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  const seen = new Set<string>();
  const normalized: TableModuleColumn[] = [];
  raw.forEach((item, index) => {
    if (!item || typeof item !== "object") {
      return;
    }
    const row = item as Record<string, unknown>;
    const moduleKeyRaw = String(row.moduleKey || row.id || "").trim();
    const moduleKey = moduleKeyRaw || `module_${index + 1}`;
    if (seen.has(moduleKey)) {
      return;
    }
    seen.add(moduleKey);
    const moduleName = String(row.moduleName || row.name || moduleKey).trim() || moduleKey;
    normalized.push({
      moduleKey,
      moduleName,
    });
  });
  return normalized;
}

function resolveModulesByContext(
  ruleFiles: RuleFileItem[],
  periodCode: string,
  objectGroupCode: string,
): TableModuleColumn[] {
  for (const item of ruleFiles) {
    const raw = String(item.contentJson || "").trim();
    if (!raw) {
      continue;
    }
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      if (Array.isArray(parsed.scopedRules)) {
        const matchedScope = parsed.scopedRules.find((scope) => {
          if (!scope || typeof scope !== "object") {
            return false;
          }
          const scoped = scope as Record<string, unknown>;
          const periods = Array.isArray(scoped.applicablePeriods) ? scoped.applicablePeriods : [];
          const groups = Array.isArray(scoped.applicableObjectGroups) ? scoped.applicableObjectGroups : [];
          return periods.includes(periodCode) && groups.includes(objectGroupCode);
        });
        if (matchedScope && typeof matchedScope === "object") {
          const scoped = matchedScope as Record<string, unknown>;
          return normalizeScoreModules(scoped.scoreModules);
        }
      }

      const fallbackModules = normalizeScoreModules(parsed.scoreModules);
      if (fallbackModules.length > 0) {
        return fallbackModules;
      }
    } catch (_error) {
      continue;
    }
  }
  return [];
}

function compareObjectOrder(left: AssessmentSessionObjectItem, right: AssessmentSessionObjectItem): number {
  if (left.sortOrder !== right.sortOrder) {
    return left.sortOrder - right.sortOrder;
  }
  return left.id - right.id;
}

async function loadAssessmentTableData(): Promise<void> {
  const currentSeq = ++fetchSequence;
  if (!isContextReady.value || !contextStore.sessionId) {
    moduleColumns.value = [];
    assessmentRows.value = [];
    return;
  }

  loadingTable.value = true;
  try {
    const [objects, ruleFiles] = await Promise.all([
      listAssessmentSessionObjects(contextStore.sessionId),
      listRuleFiles(contextStore.sessionId, false),
    ]);
    if (currentSeq !== fetchSequence) {
      return;
    }

    const modules = resolveModulesByContext(ruleFiles, contextStore.periodCode, contextStore.objectGroupCode);
    const filteredObjects = objects
      .filter((item) => item.groupCode === contextStore.objectGroupCode && item.isActive)
      .sort(compareObjectOrder);

    moduleColumns.value = modules;
    assessmentRows.value = filteredObjects.map((item, index) => {
      const source = item as unknown as Record<string, unknown>;
      const sourceModuleScores = source.moduleScores;
      const moduleScores: Record<string, number | null> = {};
      modules.forEach((module) => {
        if (sourceModuleScores && typeof sourceModuleScores === "object") {
          const rawValue = (sourceModuleScores as Record<string, unknown>)[module.moduleKey];
          moduleScores[module.moduleKey] = toNumberOrNull(rawValue);
          return;
        }
        moduleScores[module.moduleKey] = null;
      });

      const rankValue = toNumberOrNull(source.rank);
      const gradeRaw = typeof source.grade === "string" ? source.grade.trim() : "";
      return {
        rank: rankValue ? Math.max(1, Math.floor(rankValue)) : index + 1,
        objectName: item.objectName,
        totalScore: toNumberOrNull(source.totalScore),
        grade: gradeRaw || "-",
        moduleScores,
      };
    });
  } catch (error) {
    if (currentSeq !== fetchSequence) {
      return;
    }
    moduleColumns.value = [];
    assessmentRows.value = [];
    const message = error instanceof Error ? error.message : "加载考核数据失败";
    ElMessage.error(message);
  } finally {
    if (currentSeq === fetchSequence) {
      loadingTable.value = false;
    }
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载上下文失败";
    ElMessage.error(message);
  }
});

watch(
  () => [contextStore.sessionId, contextStore.periodCode, contextStore.objectGroupCode],
  () => {
    void loadAssessmentTableData();
  },
  { immediate: true },
);
</script>

<style scoped>
.overview-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.summary-item {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 10px 12px;
  display: grid;
  gap: 6px;
}

.summary-label {
  font-size: 12px;
  color: #909399;
  line-height: 1;
}

.summary-value {
  font-size: 14px;
  color: #303133;
  font-weight: 600;
  line-height: 1.4;
  word-break: break-word;
}

.table-caption {
  margin-bottom: 10px;
  color: #606266;
  font-size: 13px;
}

.mt-12 {
  margin-top: 12px;
}

@media (max-width: 1200px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .summary-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
