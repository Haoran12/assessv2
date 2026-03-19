<template>
  <div class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="subtitle">{{ contextText }}</div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadData">刷新</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="contextWarning"
        :title="contextWarning"
        type="warning"
        :closable="false"
        class="mb-12"
      />

      <el-alert
        v-if="bindingNotice"
        :title="bindingNotice"
        type="info"
        :closable="false"
        class="mb-12"
      />

      <el-skeleton v-if="loadingFiles" :rows="8" animated />
      <el-empty v-else-if="!currentRule" description="当前场次暂无规则文件" />
      <template v-else>
        <div class="section-block">
          <div class="section-head">
            <strong>分数模块</strong>
            <div class="inline-actions">
              <span class="muted">当前范围：{{ currentScopeLabel }}</span>
              <el-button size="small" :disabled="!canEditRule || !activeScopedRule" @click="addScoreModule">新增模块</el-button>
            </div>
          </div>
          <el-empty
            v-if="!activeScopedRule"
            description="请先在顶部选择考核周期和考核对象分组"
          />
          <template v-else>
            <el-table :data="activeScopedRule.scoreModules" border>
              <el-table-column label="拖动排序" width="96" align="center">
                <template #default="{ $index }">
                  <div
                    class="drag-handle"
                    :class="{ 'is-disabled': !canEditRule, 'is-dragging': draggingModuleIndex === $index }"
                    :draggable="canEditRule"
                    @dragstart="onModuleDragStart($index, $event)"
                    @dragover="onModuleDragOver($event)"
                    @drop.prevent="onModuleDrop($index)"
                    @dragend="onModuleDragEnd"
                  >
                    <el-icon><Rank /></el-icon>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="模块名" min-width="200">
                <template #default="{ row }">
                  <el-input v-model="row.moduleName" :disabled="!canEditRule" />
                </template>
              </el-table-column>
              <el-table-column label="权重" width="140">
                <template #default="{ row }">
                  <el-input-number v-model="row.weight" :disabled="!canEditRule" :min="0" :step="1" />
                </template>
              </el-table-column>
              <el-table-column label="计分方式" width="170">
                <template #default="{ row }">
                  <el-select
                    v-model="row.calculationMethod"
                    :disabled="!canEditRule"
                    style="width: 150px"
                    @change="handleMethodChange(row)"
                  >
                    <el-option label="直接录入" value="direct_input" />
                    <el-option label="投票模式" value="vote" />
                    <el-option label="自定义脚本" value="custom_script" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="140">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openModuleDetail(row)">详情</el-button>
                  <el-button link type="danger" :disabled="!canEditRule" @click="removeScoreModule(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div
              v-if="canEditRule"
              class="module-drop-tail"
              @dragover="onModuleDragOver($event)"
              @drop.prevent="onModuleDropToEnd"
            >
              拖到这里可移到末尾
            </div>
            <div class="formula-text">
              总分 = Σ(模块分数 * 模块权重 / 总权重) + 额外加减分；当前总权重：{{ totalWeight.toFixed(2) }}
            </div>
          </template>
        </div>

        <template v-if="activeScopedRule">
          <div class="section-block">
            <div class="section-head">
              <strong>等第规则（按行顺序从高到低匹配）</strong>
              <el-button size="small" :disabled="!canEditRule" @click="addGrade">新增等第</el-button>
            </div>
            <el-table :data="activeScopedRule.grades" border>
              <el-table-column label="等第标题" width="130">
                <template #default="{ row }">
                  <el-input v-model="row.title" :disabled="!canEditRule" />
                </template>
              </el-table-column>
              <el-table-column label="上限" width="250">
                <template #default="{ row }">
                  <div class="grade-node-cell">
                    <el-switch v-model="row.scoreNode.hasUpperLimit" :disabled="!canEditRule" />
                    <el-select
                      v-model="row.scoreNode.upperOperator"
                      :disabled="!canEditRule || !row.scoreNode.hasUpperLimit"
                      style="width: 72px"
                    >
                      <el-option label="<" value="<" />
                      <el-option label="≤" value="<=" />
                    </el-select>
                    <el-input-number
                      v-model="row.scoreNode.upperScore"
                      :disabled="!canEditRule || !row.scoreNode.hasUpperLimit"
                      :step="0.1"
                    />
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="下限" width="250">
                <template #default="{ row }">
                  <div class="grade-node-cell">
                    <el-switch v-model="row.scoreNode.hasLowerLimit" :disabled="!canEditRule" />
                    <el-select
                      v-model="row.scoreNode.lowerOperator"
                      :disabled="!canEditRule || !row.scoreNode.hasLowerLimit"
                      style="width: 72px"
                    >
                      <el-option label=">" value=">" />
                      <el-option label="≥" value=">=" />
                    </el-select>
                    <el-input-number
                      v-model="row.scoreNode.lowerScore"
                      :disabled="!canEditRule || !row.scoreNode.hasLowerLimit"
                      :step="0.1"
                    />
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="额外条件脚本" min-width="200">
                <template #default="{ row }">
                  <el-input
                    v-model="row.extraConditionScript"
                    type="textarea"
                    :rows="2"
                    :disabled="!canEditRule"
                    placeholder="可为空，复用自定义脚本"
                  />
                </template>
              </el-table-column>
              <el-table-column label="区间/条件" width="130">
                <template #default="{ row }">
                  <el-select v-model="row.conditionLogic" :disabled="!canEditRule" style="width: 108px">
                    <el-option label="AND" value="and" />
                    <el-option label="OR" value="or" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="人数上限比例(%)" width="150">
                <template #default="{ row }">
                  <el-input-number
                    v-model="row.maxRatioPercent"
                    :disabled="!canEditRule"
                    :min="0"
                    :max="100"
                    :step="0.1"
                    placeholder="不限制"
                  />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="90">
                <template #default="{ row }">
                  <el-button link type="danger" :disabled="!canEditRule" @click="removeGrade(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </template>

        <el-alert
          type="info"
          :closable="false"
          class="section-block"
          title="等第分配规则：先按顺序做首轮匹配，再按人数上限迭代回退到更低等第，直到各等第上限满足。"
        />

        <div class="editor-actions">
          <el-button type="primary" :disabled="!canEditRule || saving || !activeScopedRule" :loading="saving" @click="saveRule">
            保存规则
          </el-button>
        </div>

        <el-collapse class="json-preview">
          <el-collapse-item title="JSON预览（只读）" name="preview">
            <el-input :model-value="structuredJsonPreview" type="textarea" :rows="12" readonly />
          </el-collapse-item>
        </el-collapse>
      </template>
    </el-card>

    <el-dialog
      v-model="moduleDetailVisible"
      :title="moduleDetailTitle"
      width="760px"
      destroy-on-close
    >
      <template v-if="moduleDetailTarget">
        <template v-if="moduleDetailTarget.calculationMethod === 'custom_script'">
          <div class="field-label">脚本内容</div>
          <el-input
            v-model="moduleDetailDraft.customScript"
            type="textarea"
            :rows="12"
            :disabled="!canEditRule"
            placeholder="请输入该模块的脚本内容"
          />
        </template>

        <template v-else-if="moduleDetailTarget.calculationMethod === 'vote'">
          <div class="field-label">投票详情（文本或 JSON）</div>
          <el-input
            v-model="moduleDetailDraft.voteConfigJson"
            type="textarea"
            :rows="12"
            :disabled="!canEditRule"
            placeholder="可填写投票维度、权重、说明等配置"
          />
        </template>

        <el-empty v-else description="直接录入方式暂无额外详情配置" />
      </template>
      <template #footer>
        <el-button @click="closeModuleDetail">关闭</el-button>
        <el-button
          type="primary"
          :disabled="!canEditRule"
          @click="applyModuleDetail"
        >
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { Rank } from "@element-plus/icons-vue";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useContextStore } from "@/stores/context";
import {
  listRuleFiles,
  updateRuleFile,
} from "@/api/rules";
import type { RuleFileItem } from "@/types/rules";

type ScoreMethod = "direct_input" | "vote" | "custom_script";
type ConditionLogic = "and" | "or";
type UpperOperator = "<" | "<=";
type LowerOperator = ">" | ">=";

interface ScoreModule {
  id: string;
  moduleKey: string;
  moduleName: string;
  weight: number;
  calculationMethod: ScoreMethod;
  customScript: string;
  voteConfigJson: string;
}

interface GradeScoreNode {
  hasUpperLimit: boolean;
  upperScore: number | null;
  upperOperator: UpperOperator;
  hasLowerLimit: boolean;
  lowerScore: number | null;
  lowerOperator: LowerOperator;
}

interface GradeRule {
  id: string;
  title: string;
  scoreNode: GradeScoreNode;
  extraConditionScript: string;
  conditionLogic: ConditionLogic;
  maxRatioPercent: number | null;
}

interface ScopedRule {
  id: string;
  applicablePeriods: string[];
  applicableObjectGroups: string[];
  scoreModules: ScoreModule[];
  grades: GradeRule[];
}

interface StructuredRuleContent {
  version: number;
  scopedRules: ScopedRule[];
}

const contextStore = useContextStore();

const loading = ref(false);
const loadingFiles = ref(false);
const saving = ref(false);
const draggingModuleIndex = ref<number | null>(null);

const currentRule = ref<RuleFileItem | null>(null);
const activeScopedRuleId = ref("");

const moduleDetailVisible = ref(false);
const moduleDetailTargetId = ref("");
const moduleDetailDraft = reactive({
  customScript: "",
  voteConfigJson: "",
});

const ruleContent = reactive<StructuredRuleContent>(defaultRuleContent(true));

const contextWarning = ref("");
const canEditRule = computed(() => !!currentRule.value?.canEdit);

const activeScopedRule = computed(() =>
  ruleContent.scopedRules.find((item) => item.id === activeScopedRuleId.value) || null,
);

const totalWeight = computed(() =>
  (activeScopedRule.value?.scoreModules || []).reduce((sum, item) => sum + asNumber(item.weight, 0), 0),
);

const structuredJsonPreview = computed(() => JSON.stringify(normalizeRuleContent(cloneDeep(ruleContent)), null, 2));

const contextText = computed(() => {
  const sessionText = contextStore.currentSession?.displayName || "未选择场次";
  const periodText = contextStore.currentPeriod?.periodName || "未选择周期";
  const groupText = contextStore.currentObjectGroup?.groupName || "未选择对象分组";
  return `当前影响范围：${sessionText} / ${periodText} / ${groupText}`;
});

function resolveRulePeriodCode(periodCode?: string): string {
  const normalized = String(periodCode || "").trim().toUpperCase();
  if (!normalized) {
    return "";
  }
  const period = contextStore.periods.find((item) => item.periodCode === normalized);
  const binding = String(period?.ruleBindingKey || "").trim().toUpperCase();
  return binding || normalized;
}

const currentRulePeriodCode = computed(() => resolveRulePeriodCode(contextStore.periodCode));

const bindingNotice = computed(() => {
  const source = String(contextStore.periodCode || "").trim().toUpperCase();
  const target = currentRulePeriodCode.value;
  if (!source || !target || source === target) {
    return "";
  }
  return `Rule binding: ${source} -> ${target}. Rules are shared only; score data remains independent.`;
});

const currentScopeLabel = computed(() => {
  const source = String(contextStore.periodCode || "").trim().toUpperCase();
  const target = currentRulePeriodCode.value;
  const periodText = contextStore.currentPeriod?.periodName || "Period not selected";
  const bindingText = source && target && source !== target ? `${periodText} -> rules by ${target}` : periodText;
  const groupText = contextStore.currentObjectGroup?.groupName || "Group not selected";
  return `${bindingText} / ${groupText}`;
});

const moduleDetailTarget = computed(() =>
  activeScopedRule.value?.scoreModules.find((item) => item.id === moduleDetailTargetId.value) || null,
);

const moduleDetailTitle = computed(() => {
  const moduleName = moduleDetailTarget.value?.moduleName?.trim() || "模块";
  return `${moduleName}详情`;
});

function uuid(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function asNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return parsed;
}

function toNullableNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return null;
  }
  return parsed;
}

function cloneDeep<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function normalizeMethod(value: unknown): ScoreMethod {
  const text = String(value || "").trim().toLowerCase();
  if (text === "vote" || text === "voting") {
    return "vote";
  }
  if (text === "custom_script" || text === "script" || text === "formula" || text === "custom") {
    return "custom_script";
  }
  return "direct_input";
}

function normalizeLogic(value: unknown): ConditionLogic {
  return String(value || "").trim().toLowerCase() === "or" ? "or" : "and";
}

function normalizeUpperOperator(value: unknown): UpperOperator {
  return String(value || "").trim() === "<" ? "<" : "<=";
}

function normalizeLowerOperator(value: unknown): LowerOperator {
  return String(value || "").trim() === ">" ? ">" : ">=";
}

function normalizedCodeList(value: unknown, uppercase = false): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of value) {
    const text = String(item || "").trim();
    if (!text) {
      continue;
    }
    const normalized = uppercase ? text.toUpperCase() : text;
    if (seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
}

function unknownToText(value: unknown): string {
  if (value === null || value === undefined || value === "") {
    return "";
  }
  if (typeof value === "string") {
    return value.trim();
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch (_error) {
    return String(value);
  }
}

function parseJsonOrText(value: string): unknown {
  const text = String(value || "").trim();
  if (!text) {
    return "";
  }
  try {
    return JSON.parse(text);
  } catch (_error) {
    return text;
  }
}

function newScoreModule(seed = "模块", weight = 100): ScoreModule {
  const id = uuid("module");
  return {
    id,
    moduleKey: id,
    moduleName: seed,
    weight,
    calculationMethod: "direct_input",
    customScript: "",
    voteConfigJson: "",
  };
}

function newGrade(seed = "A"): GradeRule {
  return {
    id: uuid("grade"),
    title: seed,
    scoreNode: {
      hasUpperLimit: true,
      upperScore: 100,
      upperOperator: "<=",
      hasLowerLimit: true,
      lowerScore: 90,
      lowerOperator: ">=",
    },
    extraConditionScript: "",
    conditionLogic: "and",
    maxRatioPercent: null,
  };
}

function defaultScopedRule(withContext: boolean): ScopedRule {
  return {
    id: uuid("scoped"),
    applicablePeriods: withContext && currentRulePeriodCode.value ? [currentRulePeriodCode.value] : [],
    applicableObjectGroups: withContext && contextStore.objectGroupCode ? [contextStore.objectGroupCode] : [],
    scoreModules: [newScoreModule("基础绩效", 100)],
    grades: [
      newGrade("A"),
      {
        ...newGrade("B"),
        scoreNode: {
          hasUpperLimit: true,
          upperScore: 89.99,
          upperOperator: "<=",
          hasLowerLimit: true,
          lowerScore: 80,
          lowerOperator: ">=",
        },
      },
      {
        ...newGrade("C"),
        scoreNode: {
          hasUpperLimit: true,
          upperScore: 79.99,
          upperOperator: "<=",
          hasLowerLimit: false,
          lowerScore: null,
          lowerOperator: ">=",
        },
      },
    ],
  };
}

function defaultRuleContent(withContext: boolean): StructuredRuleContent {
  return {
    version: 3,
    scopedRules: [defaultScopedRule(withContext)],
  };
}

function normalizeScoreModule(raw: any, index: number): ScoreModule {
  const id = String(raw?.id || raw?.moduleKey || `module_${index + 1}`).trim() || uuid("module");
  return {
    id,
    moduleKey: String(raw?.moduleKey || id).trim() || id,
    moduleName: String(raw?.moduleName || raw?.name || `模块${index + 1}`).trim(),
    weight: Math.max(0, asNumber(raw?.weight, 0)),
    calculationMethod: normalizeMethod(raw?.calculationMethod || raw?.method),
    customScript: String(raw?.customScript || raw?.detail?.customScript?.script || "").trim(),
    voteConfigJson: unknownToText(raw?.voteConfig ?? raw?.detail?.voteConfig ?? raw?.detail?.vote ?? raw?.detail?.voteDetail),
  };
}

function normalizeGrade(raw: any, index: number): GradeRule {
  const scoreNode = raw?.scoreNode || {};
  const hasUpperFromLegacy = raw?.max !== null && raw?.max !== undefined && raw?.max !== "";
  const hasLowerFromLegacy = raw?.min !== null && raw?.min !== undefined && raw?.min !== "";
  const maxRatio =
    raw?.maxRatioPercent !== undefined
      ? toNullableNumber(raw?.maxRatioPercent)
      : raw?.quota !== undefined
        ? asNumber(raw?.quota, 0) * 100
        : raw?.maxRatio !== undefined
          ? asNumber(raw?.maxRatio, 0) * 100
          : null;

  return {
    id: String(raw?.id || `grade_${index + 1}`) || uuid("grade"),
    title: String(raw?.title || raw?.grade || `等第${index + 1}`).trim(),
    scoreNode: {
      hasUpperLimit: Boolean(scoreNode?.hasUpperLimit ?? hasUpperFromLegacy),
      upperScore: toNullableNumber(scoreNode?.upperScore ?? raw?.max),
      upperOperator: normalizeUpperOperator(scoreNode?.upperOperator ?? scoreNode?.maxOp ?? "<="),
      hasLowerLimit: Boolean(scoreNode?.hasLowerLimit ?? hasLowerFromLegacy),
      lowerScore: toNullableNumber(scoreNode?.lowerScore ?? raw?.min),
      lowerOperator: normalizeLowerOperator(scoreNode?.lowerOperator ?? scoreNode?.minOp ?? ">="),
    },
    extraConditionScript: String(raw?.extraConditionScript || "").trim(),
    conditionLogic: normalizeLogic(raw?.conditionLogic || "and"),
    maxRatioPercent: maxRatio,
  };
}

function normalizeScopedRule(raw: any, index: number): ScopedRule {
  const sourceModules = Array.isArray(raw?.scoreModules)
    ? raw.scoreModules
    : Array.isArray(raw?.scoreCalculation?.modules)
      ? raw.scoreCalculation.modules
      : [];
  const modules = sourceModules
    .filter((item: any) => !Boolean(item?.isExtra))
    .map((item: any, moduleIndex: number) => normalizeScoreModule(item, moduleIndex));

  const sourceGrades = Array.isArray(raw?.grades)
    ? raw.grades
    : Array.isArray(raw?.gradeRules)
      ? raw.gradeRules
      : Array.isArray(raw?.grade?.rules)
        ? raw.grade.rules
        : [];
  const grades = sourceGrades.map((item: any, gradeIndex: number) => normalizeGrade(item, gradeIndex));

  return {
    id: String(raw?.id || `scoped_${index + 1}`) || uuid("scoped"),
    applicablePeriods: normalizedCodeList(raw?.applicablePeriods ?? raw?.periodCodes, true),
    applicableObjectGroups: normalizedCodeList(raw?.applicableObjectGroups ?? raw?.objectGroupCodes, false),
    scoreModules: modules.length > 0 ? modules : [newScoreModule(`模块${index + 1}`, 100)],
    grades: grades.length > 0 ? grades : [newGrade("A")],
  };
}

function normalizeRuleContent(input: StructuredRuleContent | Record<string, any>): StructuredRuleContent {
  const raw = input as any;

  let scopedRulesRaw: any[] = [];
  if (Array.isArray(raw?.scopedRules)) {
    scopedRulesRaw = raw.scopedRules;
  } else if (Array.isArray(raw?.rules)) {
    scopedRulesRaw = raw.rules;
  } else {
    scopedRulesRaw = [
      {
        applicablePeriods: normalizedCodeList(raw?.applicablePeriods ?? raw?.periodCodes, true),
        applicableObjectGroups: normalizedCodeList(raw?.applicableObjectGroups ?? raw?.objectGroupCodes, false),
        scoreModules: raw?.scoreModules,
        grades: raw?.grades ?? raw?.gradeRules,
      },
    ];
  }

  const scopedRules = scopedRulesRaw.map((item, index) => normalizeScopedRule(item, index));

  return {
    version: Math.max(3, asNumber(raw?.version, 3)),
    scopedRules: scopedRules.length > 0 ? scopedRules : [defaultScopedRule(true)],
  };
}

function parseRuleContent(raw: string, withContext: boolean): StructuredRuleContent {
  const text = String(raw || "").trim();
  if (!text) {
    return cloneDeep(defaultRuleContent(withContext));
  }
  try {
    const parsed = JSON.parse(text);
    return normalizeRuleContent(parsed as Record<string, any>);
  } catch (_error) {
    return cloneDeep(defaultRuleContent(withContext));
  }
}

function fillEditor(rule: RuleFileItem | null): void {
  if (!rule) {
    Object.assign(ruleContent, defaultRuleContent(true));
    activeScopedRuleId.value = "";
    return;
  }
  const parsed = parseRuleContent(rule.contentJson || "", true);
  Object.assign(ruleContent, parsed);
  syncActiveScopedRuleWithContext();
}

function setCurrentRule(rule: RuleFileItem | null): void {
  currentRule.value = rule;
  fillEditor(rule);
}

function validateContextForLoad(): string {
  if (!contextStore.sessionId) {
    return "请先在顶部选择考核场次";
  }
  if (!contextStore.periodCode || !contextStore.objectGroupCode) {
    return "请先在顶部选择考核周期和考核对象分组";
  }
  return "";
}

function syncActiveScopedRuleWithContext(): void {
  if (!currentRule.value) {
    activeScopedRuleId.value = "";
    return;
  }
  const periodCode = currentRulePeriodCode.value;
  const groupCode = contextStore.objectGroupCode;
  if (!periodCode || !groupCode) {
    activeScopedRuleId.value = "";
    return;
  }

  const target = ruleContent.scopedRules.find(
    (item) =>
      item.applicablePeriods.includes(periodCode) &&
      item.applicableObjectGroups.includes(groupCode),
  );
  if (target) {
    activeScopedRuleId.value = target.id;
    return;
  }

  const row = defaultScopedRule(false);
  row.applicablePeriods = [periodCode];
  row.applicableObjectGroups = [groupCode];
  ruleContent.scopedRules.push(row);
  activeScopedRuleId.value = row.id;
}

async function loadFilesOnly(): Promise<void> {
  if (!contextStore.sessionId) {
    setCurrentRule(null);
    return;
  }
  loadingFiles.value = true;
  try {
    const items = await listRuleFiles(contextStore.sessionId, false);
    if (items.length === 0) {
      setCurrentRule(null);
      return;
    }

    const existingID = currentRule.value?.id;
    const next = existingID ? items.find((item) => item.id === existingID) || items[0] : items[0];
    setCurrentRule(next);
  } finally {
    loadingFiles.value = false;
  }
}

async function loadData(): Promise<void> {
  loading.value = true;
  try {
    await contextStore.ensureInitialized();
    contextWarning.value = validateContextForLoad();
    await loadFilesOnly();
    syncActiveScopedRuleWithContext();
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载规则管理数据失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

function handleMethodChange(module: ScoreModule): void {
  module.calculationMethod = normalizeMethod(module.calculationMethod);
  if (module.calculationMethod !== "custom_script") {
    module.customScript = "";
  }
  if (module.calculationMethod !== "vote") {
    module.voteConfigJson = "";
  }
}

function onModuleDragStart(index: number, event: DragEvent): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    event.preventDefault();
    return;
  }
  draggingModuleIndex.value = index;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", String(index));
  }
}

function onModuleDragOver(event: DragEvent): void {
  if (!canEditRule.value || draggingModuleIndex.value === null) {
    return;
  }
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "move";
  }
}

function onModuleDrop(targetIndex: number): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    draggingModuleIndex.value = null;
    return;
  }
  const fromIndex = draggingModuleIndex.value;
  if (fromIndex === null || fromIndex === targetIndex) {
    draggingModuleIndex.value = null;
    return;
  }
  const modules = activeScopedRule.value.scoreModules;
  if (fromIndex < 0 || fromIndex >= modules.length || targetIndex < 0 || targetIndex >= modules.length) {
    draggingModuleIndex.value = null;
    return;
  }
  const [moved] = modules.splice(fromIndex, 1);
  const insertIndex = fromIndex < targetIndex ? targetIndex - 1 : targetIndex;
  modules.splice(insertIndex, 0, moved);
  draggingModuleIndex.value = null;
}

function onModuleDropToEnd(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    draggingModuleIndex.value = null;
    return;
  }
  const fromIndex = draggingModuleIndex.value;
  if (fromIndex === null) {
    return;
  }
  const modules = activeScopedRule.value.scoreModules;
  if (fromIndex < 0 || fromIndex >= modules.length) {
    draggingModuleIndex.value = null;
    return;
  }
  const [moved] = modules.splice(fromIndex, 1);
  modules.push(moved);
  draggingModuleIndex.value = null;
}

function onModuleDragEnd(): void {
  draggingModuleIndex.value = null;
}

function openModuleDetail(module: ScoreModule): void {
  moduleDetailTargetId.value = module.id;
  moduleDetailDraft.customScript = module.customScript || "";
  moduleDetailDraft.voteConfigJson = module.voteConfigJson || "";
  moduleDetailVisible.value = true;
}

function closeModuleDetail(): void {
  moduleDetailVisible.value = false;
  moduleDetailTargetId.value = "";
  moduleDetailDraft.customScript = "";
  moduleDetailDraft.voteConfigJson = "";
}

function applyModuleDetail(): void {
  const target = moduleDetailTarget.value;
  if (!target) {
    closeModuleDetail();
    return;
  }
  if (target.calculationMethod === "custom_script") {
    target.customScript = String(moduleDetailDraft.customScript || "");
    target.voteConfigJson = "";
  } else if (target.calculationMethod === "vote") {
    target.voteConfigJson = String(moduleDetailDraft.voteConfigJson || "");
    target.customScript = "";
  } else {
    target.customScript = "";
    target.voteConfigJson = "";
  }
  closeModuleDetail();
}

function addScoreModule(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.scoreModules.push(newScoreModule(`模块${activeScopedRule.value.scoreModules.length + 1}`, 0));
}

function removeScoreModule(module: ScoreModule): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.scoreModules = activeScopedRule.value.scoreModules.filter((item) => item.id !== module.id);
}

function addGrade(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.grades.push(newGrade(`等第${activeScopedRule.value.grades.length + 1}`));
}

function removeGrade(grade: GradeRule): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.grades = activeScopedRule.value.grades.filter((item) => item.id !== grade.id);
}

function normalizeRuleForSave(row: ScopedRule): ScopedRule {
  const normalizedModules = row.scoreModules.map((module, index) => {
    const normalized: any = {
      id: module.id || uuid("module"),
      moduleKey: String(module.moduleKey || module.id || `module_${index + 1}`).trim() || `module_${index + 1}`,
      moduleName: String(module.moduleName || "").trim(),
      weight: Math.max(0, asNumber(module.weight, 0)),
      calculationMethod: normalizeMethod(module.calculationMethod),
      customScript: String(module.customScript || "").trim(),
    };

    if (normalized.calculationMethod === "vote" && String(module.voteConfigJson || "").trim()) {
      normalized.detail = {
        voteConfig: parseJsonOrText(String(module.voteConfigJson || "")),
      };
    }

    return normalized;
  });

  const normalizedGrades = row.grades.map((grade) => ({
    id: grade.id || uuid("grade"),
    title: String(grade.title || "").trim(),
    scoreNode: {
      hasUpperLimit: Boolean(grade.scoreNode?.hasUpperLimit),
      upperScore: toNullableNumber(grade.scoreNode?.upperScore),
      upperOperator: normalizeUpperOperator(grade.scoreNode?.upperOperator),
      hasLowerLimit: Boolean(grade.scoreNode?.hasLowerLimit),
      lowerScore: toNullableNumber(grade.scoreNode?.lowerScore),
      lowerOperator: normalizeLowerOperator(grade.scoreNode?.lowerOperator),
    },
    extraConditionScript: String(grade.extraConditionScript || "").trim(),
    conditionLogic: normalizeLogic(grade.conditionLogic),
    maxRatioPercent: toNullableNumber(grade.maxRatioPercent),
  }));

  return {
    id: row.id || uuid("scoped"),
    applicablePeriods: normalizedCodeList(row.applicablePeriods, true),
    applicableObjectGroups: normalizedCodeList(row.applicableObjectGroups, false),
    scoreModules: normalizedModules,
    grades: normalizedGrades,
  };
}

function validateRuleContent(content: StructuredRuleContent): string {
  const effectiveScopedRules = content.scopedRules.filter(
    (item) => item.applicablePeriods.length > 0 && item.applicableObjectGroups.length > 0,
  );
  if (effectiveScopedRules.length === 0) {
    return "当前上下文尚未生成可保存的具体规则";
  }

  for (let index = 0; index < effectiveScopedRules.length; index += 1) {
    const scoped = effectiveScopedRules[index];
    const title = `第${index + 1}条具体规则`;

    if (scoped.scoreModules.length === 0) {
      return `${title}至少需要一个分数模块`;
    }
    const total = scoped.scoreModules.reduce((sum, item) => sum + item.weight, 0);
    if (total <= 0) {
      return `${title}的模块总权重必须大于 0`;
    }

    for (const module of scoped.scoreModules) {
      if (!module.moduleName.trim()) {
        return `${title}存在空模块名称`;
      }
      if (module.weight <= 0) {
        return `${title}中模块「${module.moduleName}」权重必须大于 0`;
      }
      if (module.calculationMethod === "custom_script" && !module.customScript.trim()) {
        return `${title}中模块「${module.moduleName}」使用脚本方式时脚本不能为空`;
      }
    }

    if (scoped.grades.length === 0) {
      return `${title}至少需要一个等第`;
    }

    for (const grade of scoped.grades) {
      if (!grade.title.trim()) {
        return `${title}存在空等第标题`;
      }
      const node = grade.scoreNode;
      if (!node.hasLowerLimit && !node.hasUpperLimit && !grade.extraConditionScript.trim()) {
        return `${title}中等第「${grade.title}」必须配置分数节点或额外条件`;
      }
      if (node.hasLowerLimit && node.lowerScore === null) {
        return `${title}中等第「${grade.title}」下限分值不能为空`;
      }
      if (node.hasUpperLimit && node.upperScore === null) {
        return `${title}中等第「${grade.title}」上限分值不能为空`;
      }
      if (node.hasLowerLimit && node.hasUpperLimit && node.lowerScore !== null && node.upperScore !== null) {
        if (node.lowerScore > node.upperScore) {
          return `${title}中等第「${grade.title}」下限分值不能大于上限分值`;
        }
        if (
          node.lowerScore === node.upperScore &&
          (node.lowerOperator === ">" || node.upperOperator === "<")
        ) {
          return `${title}中等第「${grade.title}」上下限分值相等时符号组合无可行区间`;
        }
      }
      if (grade.maxRatioPercent !== null && (grade.maxRatioPercent <= 0 || grade.maxRatioPercent > 100)) {
        return `${title}中等第「${grade.title}」人数上限比例必须在 (0, 100] 之间`;
      }
    }
  }

  return "";
}

async function saveRule(): Promise<void> {
  if (!currentRule.value) {
    return;
  }
  if (!canEditRule.value) {
    ElMessage.warning("当前规则不可编辑");
    return;
  }
  if (!activeScopedRule.value) {
    ElMessage.warning("请先在顶部选择考核周期和考核对象分组");
    return;
  }

  const normalizedScopedRules = ruleContent.scopedRules
    .map((item) => normalizeRuleForSave(item))
    .filter((item) => item.applicablePeriods.length > 0 && item.applicableObjectGroups.length > 0);

  const normalizedContent: StructuredRuleContent = {
    version: 3,
    scopedRules: normalizedScopedRules,
  };

  const validationError = validateRuleContent(normalizedContent);
  if (validationError) {
    ElMessage.warning(validationError);
    return;
  }

  saving.value = true;
  try {
    const updated = await updateRuleFile(currentRule.value.id, {
      assessmentId: currentRule.value.assessmentId,
      ruleName: currentRule.value.ruleName,
      description: currentRule.value.description || "",
      contentJson: JSON.stringify(normalizedContent, null, 2),
    });
    ElMessage.success("规则已保存");
    await loadFilesOnly();
    if (currentRule.value?.id !== updated.id) {
      setCurrentRule(updated);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存规则失败";
    ElMessage.error(message);
  } finally {
    saving.value = false;
  }
}

watch(
  () => contextStore.sessionId,
  () => {
    contextWarning.value = validateContextForLoad();
    void loadData();
  },
);

watch(
  () => [contextStore.periodCode, contextStore.objectGroupCode],
  () => {
    contextWarning.value = validateContextForLoad();
    syncActiveScopedRuleWithContext();
    closeModuleDetail();
  },
);

watch(
  () => activeScopedRuleId.value,
  () => {
    closeModuleDetail();
  },
);

onMounted(async () => {
  await loadData();
});
</script>

<style scoped>
.rules-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.subtitle {
  color: #606266;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.section-block {
  margin-top: 14px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  gap: 8px;
}

.inline-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.formula-text {
  margin-top: 8px;
  color: #606266;
  font-size: 13px;
}

.field-label {
  margin-bottom: 6px;
  font-size: 13px;
  color: #606266;
}

.muted {
  color: #909399;
}

.grade-node-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.editor-actions {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.json-preview {
  margin-top: 10px;
}

.mb-12 {
  margin-bottom: 12px;
}

.drag-handle {
  width: 28px;
  height: 28px;
  border: 1px dashed #c0c4cc;
  border-radius: 4px;
  margin: 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: move;
  color: #606266;
  transition: all 0.2s;
}

.drag-handle:hover {
  border-color: #409eff;
  color: #409eff;
}

.drag-handle.is-dragging {
  background: #ecf5ff;
  border-color: #409eff;
}

.drag-handle.is-disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.module-drop-tail {
  margin-top: 8px;
  border: 1px dashed #dcdfe6;
  border-radius: 4px;
  font-size: 12px;
  color: #909399;
  text-align: center;
  padding: 8px;
}
</style>
