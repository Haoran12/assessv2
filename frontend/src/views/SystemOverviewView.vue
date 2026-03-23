<template>
  <div ref="overviewViewRef" class="overview-view">
    <el-tabs v-model="activeTab">
      <el-tab-pane label="当前考核数据" name="summary">
        <el-card>
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <strong>考核结果</strong>
                <span class="context-text">{{ contextSummaryText }}</span>
              </div>
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
      </el-tab-pane>

      <el-tab-pane label="分数录入" name="entry">
        <el-card>
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <strong>分数录入</strong>
                <span class="context-text">{{ contextSummaryText }}</span>
              </div>
              <div class="header-actions">
                <el-tag type="info">待保存 {{ pendingScoreCount }} 项</el-tag>
                <el-button size="small" :loading="loadingTable" @click="loadAssessmentTableData">刷新</el-button>
                <el-button
                  type="primary"
                  size="small"
                  :disabled="!canEditScores || pendingScoreCount === 0 || savingScores"
                  :loading="savingScores"
                  @click="saveModuleScores"
                >
                  保存录入
                </el-button>
              </div>
            </div>
          </template>

          <el-alert
            v-if="!isContextReady"
            title="请先在顶部选择完整的考核场次、周期和对象分组。"
            type="warning"
            :closable="false"
          />
          <template v-else>
            <el-alert
              v-if="!canEditScores"
              title="当前账号无分数录入权限，仅可查看。"
              type="info"
              :closable="false"
              class="entry-readonly-alert"
            />
            <el-table :data="assessmentRows" border stripe v-loading="loadingTable">
              <el-table-column prop="objectName" label="考核对象名称" min-width="220" fixed="left" />
              <el-table-column
                v-for="module in moduleColumns"
                :key="`entry_${module.moduleKey}`"
                min-width="180"
              >
                <template #header>
                  <div class="entry-header">
                    <span>{{ module.moduleName }}</span>
                    <el-tag v-if="module.calculationMethod === 'vote'" size="small" type="warning">票决(线下)</el-tag>
                    <el-tag v-else-if="module.calculationMethod === 'custom_script'" size="small">脚本</el-tag>
                    <el-tag v-else size="small" type="success">直录</el-tag>
                  </div>
                </template>
                <template #default="{ row }">
                  <template v-if="module.calculationMethod === 'direct_input'">
                    <el-input-number
                      :model-value="toInputNumberValue(row.moduleScores[module.moduleKey])"
                      :min="0"
                      :max="100"
                      :step="0.1"
                      :precision="2"
                      :controls="false"
                      style="width: 100%"
                      :disabled="!canEditScores"
                      @change="onDirectScoreChange(row, module, $event)"
                    />
                  </template>
                  <template v-else-if="module.calculationMethod === 'vote'">
                    <el-button
                      link
                      type="primary"
                      :disabled="!canEditScores"
                      @click="openVoteDialog(row, module)"
                    >
                      {{ formatScoreAction(row.moduleScores[module.moduleKey]) }}
                    </el-button>
                  </template>
                  <template v-else>
                    <span class="readonly-score">{{ formatScore(row.moduleScores[module.moduleKey]) }}</span>
                  </template>
                </template>
              </el-table-column>
            </el-table>
            <el-empty v-if="!loadingTable && assessmentRows.length === 0" description="当前分组暂无可录入对象" />
          </template>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="分数模块" name="rule-modules">
        <RulesView
          v-if="activeTab === 'rule-modules'"
          :initial-edit-tab="'modules'"
          :lock-edit-tab="true"
          :header-title="'分数计算规则'"
        />
      </el-tab-pane>

      <el-tab-pane label="等第划分" name="rule-grades">
        <RulesView
          v-if="activeTab === 'rule-grades'"
          :initial-edit-tab="'grades'"
          :lock-edit-tab="true"
          :header-title="'等第划分规则'"
          :hide-grade-inner-title="true"
        />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="voteDialogVisible" title="线下票决结果录入" width="620px" destroy-on-close>
      <el-form label-width="96px">
        <el-form-item label="考核对象">
          <span>{{ voteDialog.objectName }}</span>
        </el-form-item>
        <el-form-item label="分数模块">
          <span>{{ voteDialog.moduleName }}</span>
        </el-form-item>
        <el-form-item label="纸质票数">
          <div class="vote-count-grid">
            <el-input-number
              v-model="voteDialog.excellentCount"
              :min="0"
              :step="1"
              :precision="0"
              :controls="false"
              placeholder="优秀票"
            />
            <el-input-number
              v-model="voteDialog.goodCount"
              :min="0"
              :step="1"
              :precision="0"
              :controls="false"
              placeholder="良好票"
            />
            <el-input-number
              v-model="voteDialog.averageCount"
              :min="0"
              :step="1"
              :precision="0"
              :controls="false"
              placeholder="中等票"
            />
            <el-input-number
              v-model="voteDialog.poorCount"
              :min="0"
              :step="1"
              :precision="0"
              :controls="false"
              placeholder="较差票"
            />
          </div>
        </el-form-item>
        <el-form-item>
          <div class="vote-convert-row">
            <span class="vote-convert-text">
              折算分值：
              <strong>{{ formatScore(voteConvertedScore) }}</strong>
            </span>
            <el-button plain type="primary" :disabled="voteConvertedScore === null" @click="applyVoteConvertedScore">
              使用折算分值
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="最终分值" required>
          <el-input-number
            v-model="voteDialog.score"
            :min="0"
            :max="100"
            :step="0.1"
            :precision="2"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item>
          <el-alert
            type="info"
            :closable="false"
            title="线上投票功能当前仅保留占位。请按线下纸质票结果录入模块最终分值。"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="voteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="applyVoteDialogScore">保存到表格</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { listCalculatedAssessmentSessionObjects, upsertAssessmentModuleScores } from "@/api/assessment";
import { listRuleFiles } from "@/api/rules";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import RulesView from "@/views/RulesView.vue";
import type { AssessmentSessionObjectItem } from "@/types/assessment";
import type { RuleFileItem } from "@/types/rules";

type ScoreMethod = "direct_input" | "vote" | "custom_script";

interface TableModuleColumn {
  moduleKey: string;
  moduleName: string;
  calculationMethod: ScoreMethod;
}

interface TableRow {
  objectId: number;
  rank: number;
  objectName: string;
  totalScore: number | null;
  grade: string;
  moduleScores: Record<string, number | null>;
}

interface PendingScoreItem {
  periodCode: string;
  objectId: number;
  moduleKey: string;
  score: number;
}

const contextStore = useContextStore();
const appStore = useAppStore();
const overviewViewRef = ref<HTMLElement>();
const activeTab = ref<"summary" | "entry" | "rule-modules" | "rule-grades">("summary");
const moduleColumns = ref<TableModuleColumn[]>([]);
const assessmentRows = ref<TableRow[]>([]);
const loadingTable = ref(false);
const savingScores = ref(false);
const pendingScoreMap = ref<Record<string, PendingScoreItem>>({});
const voteDialogVisible = ref(false);
const voteDialog = ref<{
  objectId: number;
  objectName: string;
  moduleKey: string;
  moduleName: string;
  excellentCount?: number;
  goodCount?: number;
  averageCount?: number;
  poorCount?: number;
  score?: number;
}>({
  objectId: 0,
  objectName: "",
  moduleKey: "",
  moduleName: "",
  excellentCount: undefined,
  goodCount: undefined,
  averageCount: undefined,
  poorCount: undefined,
  score: undefined,
});
let fetchSequence = 0;
const voteGradeScores = {
  excellent: 100,
  good: 85,
  average: 70,
  poor: 60,
} as const;

const isContextReady = computed(() =>
  Boolean(contextStore.sessionId && contextStore.periodCode && contextStore.objectGroupCode),
);
const canEditScores = computed(() => appStore.hasPermission("assessment:update"));
const pendingScoreCount = computed(() => Object.keys(pendingScoreMap.value).length);
const contextSummaryText = computed(() => {
  const sessionName = contextStore.currentSession?.displayName || "-";
  const periodName = contextStore.currentPeriod?.periodName || contextStore.periodCode || "-";
  const groupName = contextStore.currentObjectGroup?.groupName || contextStore.objectGroupCode || "-";
  return `场次：${sessionName} / 周期：${periodName} / 对象分组：${groupName}`;
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

function formatScoreAction(value: number | null): string {
  const text = formatScore(value);
  return text === "-" ? "点击录入" : text;
}

function toInputNumberValue(value: number | null): number | undefined {
  if (value === null || !Number.isFinite(value)) {
    return undefined;
  }
  return value;
}

function toVoteCount(value: unknown): number {
  const parsed = toNumberOrNull(value);
  if (parsed === null || parsed <= 0) {
    return 0;
  }
  return Math.floor(parsed);
}

const voteConvertedScore = computed(() => {
  const excellentCount = toVoteCount(voteDialog.value.excellentCount);
  const goodCount = toVoteCount(voteDialog.value.goodCount);
  const averageCount = toVoteCount(voteDialog.value.averageCount);
  const poorCount = toVoteCount(voteDialog.value.poorCount);
  const totalCount = excellentCount + goodCount + averageCount + poorCount;
  if (totalCount <= 0) {
    return null;
  }
  const weightedSum =
    excellentCount * voteGradeScores.excellent
    + goodCount * voteGradeScores.good
    + averageCount * voteGradeScores.average
    + poorCount * voteGradeScores.poor;
  return weightedSum / totalCount;
});

function normalizeMethod(value: unknown): ScoreMethod {
  const text = String(value || "").trim().toLowerCase();
  if (text === "vote") {
    return "vote";
  }
  if (text === "custom_script") {
    return "custom_script";
  }
  return "direct_input";
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
      calculationMethod: normalizeMethod(row.calculationMethod || row.method),
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

function moduleScorePendingKey(periodCode: string, objectId: number, moduleKey: string): string {
  return `${periodCode}|${objectId}|${moduleKey}`;
}

function clearPendingScores(): void {
  pendingScoreMap.value = {};
}

function setPendingScore(objectId: number, moduleKey: string, score: number | null): void {
  if (!contextStore.periodCode) {
    return;
  }
  const key = moduleScorePendingKey(contextStore.periodCode, objectId, moduleKey);
  if (score === null || !Number.isFinite(score)) {
    delete pendingScoreMap.value[key];
    return;
  }
  pendingScoreMap.value[key] = {
    periodCode: contextStore.periodCode.trim().toUpperCase(),
    objectId,
    moduleKey,
    score,
  };
}

function onDirectScoreChange(row: TableRow, module: TableModuleColumn, value: number | string | undefined): void {
  const score = toNumberOrNull(value);
  row.moduleScores[module.moduleKey] = score;
  setPendingScore(row.objectId, module.moduleKey, score);
}

function openVoteDialog(row: TableRow, module: TableModuleColumn): void {
  voteDialog.value = {
    objectId: row.objectId,
    objectName: row.objectName,
    moduleKey: module.moduleKey,
    moduleName: module.moduleName,
    excellentCount: undefined,
    goodCount: undefined,
    averageCount: undefined,
    poorCount: undefined,
    score: toInputNumberValue(row.moduleScores[module.moduleKey]),
  };
  voteDialogVisible.value = true;
}

function applyVoteConvertedScore(): void {
  const converted = voteConvertedScore.value;
  if (converted === null) {
    ElMessage.warning("请先录入至少一项票数");
    return;
  }
  voteDialog.value.score = Number(converted.toFixed(2));
}

function applyVoteDialogScore(): void {
  const objectId = voteDialog.value.objectId;
  const moduleKey = voteDialog.value.moduleKey;
  if (!objectId || !moduleKey) {
    ElMessage.warning("票决录入对象无效");
    return;
  }
  const row = assessmentRows.value.find((item) => item.objectId === objectId);
  if (!row) {
    ElMessage.warning("未找到对应考核对象");
    return;
  }
  const score = toNumberOrNull(voteDialog.value.score);
  if (score === null) {
    ElMessage.warning("请填写有效分数");
    return;
  }
  row.moduleScores[moduleKey] = score;
  setPendingScore(objectId, moduleKey, score);
  voteDialogVisible.value = false;
}

async function saveModuleScores(): Promise<void> {
  if (!contextStore.sessionId || !isContextReady.value) {
    ElMessage.warning("请先选择完整的场次、周期与对象分组");
    return;
  }
  if (!canEditScores.value) {
    ElMessage.warning("当前账号没有分数录入权限");
    return;
  }
  const items = Object.values(pendingScoreMap.value);
  if (items.length === 0) {
    ElMessage.info("没有待保存的分数项");
    return;
  }

  savingScores.value = true;
  try {
    await upsertAssessmentModuleScores(contextStore.sessionId, {
      items: items.map((item) => ({
        periodCode: item.periodCode,
        objectId: item.objectId,
        moduleKey: item.moduleKey,
        score: item.score,
      })),
    });
    ElMessage.success(`已保存 ${items.length} 项分数录入`);
    clearPendingScores();
    await loadAssessmentTableData();
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存分数录入失败";
    ElMessage.error(message);
  } finally {
    savingScores.value = false;
  }
}

function hasBlockingDialogOpen(): boolean {
  return voteDialogVisible.value;
}

function isSystemWindowActive(): boolean {
  return document.visibilityState === "visible" && document.hasFocus();
}

function isOverviewShortcutScope(event: KeyboardEvent): boolean {
  const root = overviewViewRef.value;
  const target = event.target;
  if (!root || !(target instanceof Node)) {
    return false;
  }
  if (target === document.body) {
    return true;
  }
  return root.contains(target);
}

function canTriggerSaveShortcut(): boolean {
  return (
    activeTab.value === "entry"
    && isContextReady.value
    && canEditScores.value
    && pendingScoreCount.value > 0
    && !savingScores.value
    && !loadingTable.value
  );
}

function handleOverviewKeydown(event: KeyboardEvent): void {
  const ctrlOrMeta = event.ctrlKey || event.metaKey;
  if (!ctrlOrMeta || event.altKey) {
    return;
  }
  if (!isSystemWindowActive()) {
    return;
  }
  if (!isOverviewShortcutScope(event)) {
    return;
  }
  if (hasBlockingDialogOpen()) {
    return;
  }
  const key = String(event.key || "").toLowerCase();
  if (key !== "s") {
    return;
  }
  if (!canTriggerSaveShortcut()) {
    return;
  }
  event.preventDefault();
  void saveModuleScores();
}

async function loadAssessmentTableData(): Promise<void> {
  const currentSeq = ++fetchSequence;
  if (!isContextReady.value || !contextStore.sessionId) {
    moduleColumns.value = [];
    assessmentRows.value = [];
    clearPendingScores();
    return;
  }

  loadingTable.value = true;
  try {
    const [objects, ruleFiles] = await Promise.all([
      listCalculatedAssessmentSessionObjects(contextStore.sessionId, contextStore.periodCode, contextStore.objectGroupCode),
      listRuleFiles(contextStore.sessionId, false),
    ]);
    if (currentSeq !== fetchSequence) {
      return;
    }

    const modules = resolveModulesByContext(ruleFiles, contextStore.periodCode, contextStore.objectGroupCode);
    const filteredObjects = objects.sort(compareObjectOrder);

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
        objectId: item.id,
        rank: rankValue ? Math.max(1, Math.floor(rankValue)) : index + 1,
        objectName: item.objectName,
        totalScore: toNumberOrNull(source.totalScore),
        grade: gradeRaw || "-",
        moduleScores,
      };
    });
    clearPendingScores();
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
  window.addEventListener("keydown", handleOverviewKeydown);
  try {
    await contextStore.ensureInitialized();
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载上下文失败";
    ElMessage.error(message);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleOverviewKeydown);
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

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.context-text {
  color: #909399;
  font-size: 13px;
}

.entry-readonly-alert {
  margin-bottom: 10px;
}

.entry-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}

.readonly-score {
  color: #606266;
}

.vote-count-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  width: 100%;
}

.vote-convert-row {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.vote-convert-text {
  color: #606266;
  font-size: 13px;
}

@media (max-width: 1280px) {
  .card-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .header-left,
  .header-actions {
    width: 100%;
  }

  .header-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .context-text {
    display: inline-block;
    max-width: 100%;
    word-break: break-all;
  }

  .vote-count-grid {
    grid-template-columns: 1fr;
  }

  .vote-convert-row {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
