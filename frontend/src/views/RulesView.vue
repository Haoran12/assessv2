<template>
  <div class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div>
            <strong>规则管理</strong>
            <div class="subtitle">{{ contextText }}</div>
          </div>
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

      <el-row :gutter="12">
        <el-col :md="10" :sm="24" :xs="24">
          <el-card shadow="never" class="inner-card">
            <template #header>
              <div class="inner-header">
                <strong>当前场次规则</strong>
              </div>
            </template>

            <el-table v-loading="loadingFiles" :data="ruleFiles" border height="680" @row-click="pickRule">
              <el-table-column label="名称" min-width="180">
                <template #default="{ row }">
                  <div class="rule-name-cell">
                    <span>{{ row.ruleName }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="updatedAt" label="更新时间" width="160" />
            </el-table>
          </el-card>
        </el-col>

        <el-col :md="14" :sm="24" :xs="24">
          <el-card shadow="never" class="inner-card">
            <template #header>
              <div class="inner-header">
                <strong>场次规则编辑</strong>
              </div>
            </template>

            <el-empty v-if="!selectedRule" description="请选择一个规则文件" />
            <template v-else>
              <el-form label-width="90px" class="rule-meta-form">
                <el-form-item label="规则名">
                  <el-input v-model="editForm.ruleName" :disabled="!canEditRule" />
                </el-form-item>
                <el-form-item label="说明">
                  <el-input
                    v-model="editForm.description"
                    type="textarea"
                    :rows="2"
                    :disabled="!canEditRule"
                  />
                </el-form-item>
              </el-form>

              <div class="section-block">
                <div class="section-head">
                  <strong>具体规则（按周期/对象分组）</strong>
                  <div class="inline-actions">
                    <el-button size="small" :disabled="!canEditRule" @click="addScopedRule">新增具体规则</el-button>
                  </div>
                </div>
                <el-table
                  :data="ruleContent.scopedRules"
                  border
                  max-height="260"
                  row-key="id"
                  :row-class-name="scopedRuleRowClass"
                  @row-click="selectScopedRule"
                >
                  <el-table-column label="#" width="66">
                    <template #default="{ $index }">{{ $index + 1 }}</template>
                  </el-table-column>
                  <el-table-column label="适用周期" min-width="180">
                    <template #default="{ row }">
                      <div class="tag-wrap">
                        <el-tag v-for="code in row.applicablePeriods" :key="`${row.id}_p_${code}`" size="small">
                          {{ periodName(code) }}
                        </el-tag>
                        <span v-if="row.applicablePeriods.length === 0" class="muted">未设置</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="适用对象分组" min-width="200">
                    <template #default="{ row }">
                      <div class="tag-wrap">
                        <el-tag v-for="code in row.applicableObjectGroups" :key="`${row.id}_g_${code}`" size="small" type="success">
                          {{ groupName(code) }}
                        </el-tag>
                        <span v-if="row.applicableObjectGroups.length === 0" class="muted">未设置</span>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="模块" width="72">
                    <template #default="{ row }">{{ row.scoreModules.length }}</template>
                  </el-table-column>
                  <el-table-column label="等第" width="72">
                    <template #default="{ row }">{{ row.grades.length }}</template>
                  </el-table-column>
                  <el-table-column label="操作" width="150" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" :disabled="!canEditRule" @click.stop="duplicateScopedRule(row)">复制</el-button>
                      <el-button link type="danger" :disabled="!canEditRule" @click.stop="removeScopedRule(row)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>

              <template v-if="activeScopedRule">
                <div class="section-block">
                  <div class="section-head">
                    <strong>适用范围</strong>
                  </div>
                  <el-row :gutter="12">
                    <el-col :span="12">
                      <div class="field-label">适用考核周期</div>
                      <el-select
                        v-model="activeScopedRule.applicablePeriods"
                        multiple
                        filterable
                        collapse-tags
                        collapse-tags-tooltip
                        style="width: 100%"
                        placeholder="请选择周期"
                        :disabled="!canEditRule"
                      >
                        <el-option
                          v-for="item in contextStore.periods"
                          :key="item.id"
                          :label="item.periodName"
                          :value="item.periodCode"
                        />
                      </el-select>
                    </el-col>
                    <el-col :span="12">
                      <div class="field-label">适用考核对象分组</div>
                      <el-select
                        v-model="activeScopedRule.applicableObjectGroups"
                        multiple
                        filterable
                        collapse-tags
                        collapse-tags-tooltip
                        style="width: 100%"
                        placeholder="请选择对象分组"
                        :disabled="!canEditRule"
                      >
                        <el-option
                          v-for="item in contextStore.objectGroups"
                          :key="item.id"
                          :label="groupOptionLabel(item.groupCode)"
                          :value="item.groupCode"
                        />
                      </el-select>
                    </el-col>
                  </el-row>
                </div>

                <div class="section-block">
                  <div class="section-head">
                    <strong>分数模块</strong>
                    <el-button size="small" :disabled="!canEditRule" @click="addScoreModule">新增模块</el-button>
                  </div>
                  <el-table :data="activeScopedRule.scoreModules" border>
                    <el-table-column label="模块名称" min-width="180">
                      <template #default="{ row }">
                        <el-input v-model="row.moduleName" :disabled="!canEditRule" />
                      </template>
                    </el-table-column>
                    <el-table-column label="模块权重" width="120">
                      <template #default="{ row }">
                        <el-input-number v-model="row.weight" :disabled="!canEditRule" :min="0" :step="1" />
                      </template>
                    </el-table-column>
                    <el-table-column label="模块计算方式" width="160">
                      <template #default="{ row }">
                        <el-select v-model="row.calculationMethod" :disabled="!canEditRule" style="width: 140px">
                          <el-option label="直接录入" value="direct_input" />
                          <el-option label="投票模式" value="vote" />
                          <el-option label="自定义脚本" value="custom_script" />
                        </el-select>
                      </template>
                    </el-table-column>
                    <el-table-column label="自定义脚本" min-width="220">
                      <template #default="{ row }">
                        <el-input
                          v-if="row.calculationMethod === 'custom_script'"
                          v-model="row.customScript"
                          type="textarea"
                          :rows="2"
                          :disabled="!canEditRule"
                          placeholder="可复用脚本"
                        />
                        <span v-else class="muted">-</span>
                      </template>
                    </el-table-column>
                    <el-table-column label="操作" width="90">
                      <template #default="{ row }">
                        <el-button link type="danger" :disabled="!canEditRule" @click="removeScoreModule(row)">删除</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                  <div class="formula-text">
                    总分 = Σ(模块分数 * 模块权重 / 总权重) + 额外加减分；当前总权重：{{ totalWeight.toFixed(2) }}
                  </div>
                </div>

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
                <el-button type="primary" :disabled="!canEditRule || saving" :loading="saving" @click="saveRule">
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
        </el-col>
      </el-row>
    </el-card>

  </div>
</template>

<script setup lang="ts">
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

const ruleFiles = ref<RuleFileItem[]>([]);
const selectedRule = ref<RuleFileItem | null>(null);
const activeScopedRuleId = ref("");

const editForm = reactive({
  ruleName: "",
  description: "",
});

const ruleContent = reactive<StructuredRuleContent>(defaultRuleContent(true));

const contextWarning = ref("");
const canEditRule = computed(() => !!selectedRule.value?.canEdit);

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
  return `当前上下文：${sessionText} / ${periodText} / ${groupText}`;
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

function newScoreModule(seed = "模块", weight = 100): ScoreModule {
  const id = uuid("module");
  return {
    id,
    moduleKey: id,
    moduleName: seed,
    weight,
    calculationMethod: "direct_input",
    customScript: "",
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
    applicablePeriods: withContext && contextStore.periodCode ? [contextStore.periodCode] : [],
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
    editForm.ruleName = "";
    editForm.description = "";
    Object.assign(ruleContent, defaultRuleContent(true));
    activeScopedRuleId.value = ruleContent.scopedRules[0]?.id || "";
    return;
  }
  editForm.ruleName = rule.ruleName;
  editForm.description = rule.description || "";
  const parsed = parseRuleContent(rule.contentJson || "", true);
  Object.assign(ruleContent, parsed);
  activeScopedRuleId.value = ruleContent.scopedRules[0]?.id || "";
}

function pickRule(row: RuleFileItem): void {
  selectedRule.value = row;
  fillEditor(row);
}

function validateContextForLoad(): string {
  if (!contextStore.sessionId) {
    return "请先选择考核场次";
  }
  return "";
}

async function loadFilesOnly(): Promise<void> {
  if (!contextStore.sessionId) {
    ruleFiles.value = [];
    selectedRule.value = null;
    fillEditor(null);
    return;
  }
  loadingFiles.value = true;
  try {
    const items = await listRuleFiles(contextStore.sessionId, false);
    ruleFiles.value = items;
    if (!selectedRule.value) {
      if (items.length > 0) {
        pickRule(items[0]);
      }
      return;
    }
    const hit = items.find((item) => item.id === selectedRule.value?.id);
    if (hit) {
      pickRule(hit);
      return;
    }
    selectedRule.value = null;
    fillEditor(null);
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
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载规则管理数据失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

function groupOptionLabel(groupCode: string): string {
  const target = contextStore.objectGroups.find((item) => item.groupCode === groupCode);
  if (!target) {
    return groupCode;
  }
  return `${target.objectType === "team" ? "团队" : "个人"} - ${target.groupName}`;
}

function periodName(periodCode: string): string {
  return contextStore.periods.find((item) => item.periodCode === periodCode)?.periodName || periodCode;
}

function groupName(groupCode: string): string {
  return contextStore.objectGroups.find((item) => item.groupCode === groupCode)?.groupName || groupCode;
}

function scopedRuleRowClass(args: any): string {
  return args?.row?.id === activeScopedRuleId.value ? "active-scoped-row" : "";
}

function selectScopedRule(row: ScopedRule): void {
  activeScopedRuleId.value = row.id;
}

function addScopedRule(): void {
  if (!canEditRule.value) {
    return;
  }
  const row = defaultScopedRule(true);
  ruleContent.scopedRules.push(row);
  activeScopedRuleId.value = row.id;
}

function duplicateScopedRule(row: ScopedRule): void {
  if (!canEditRule.value) {
    return;
  }
  const copied = cloneDeep(row);
  copied.id = uuid("scoped");
  copied.scoreModules = copied.scoreModules.map((module) => ({
    ...module,
    id: uuid("module"),
    moduleKey: uuid("module_key"),
  }));
  copied.grades = copied.grades.map((grade) => ({
    ...grade,
    id: uuid("grade"),
  }));
  ruleContent.scopedRules.push(copied);
  activeScopedRuleId.value = copied.id;
}

function removeScopedRule(row: ScopedRule): void {
  if (!canEditRule.value) {
    return;
  }
  const index = ruleContent.scopedRules.findIndex((item) => item.id === row.id);
  if (index < 0) {
    return;
  }
  ruleContent.scopedRules.splice(index, 1);
  if (activeScopedRuleId.value === row.id) {
    activeScopedRuleId.value = ruleContent.scopedRules[0]?.id || "";
  }
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
  const normalizedModules = row.scoreModules.map((module, index) => ({
    id: module.id || uuid("module"),
    moduleKey: String(module.moduleKey || module.id || `module_${index + 1}`).trim() || `module_${index + 1}`,
    moduleName: String(module.moduleName || "").trim(),
    weight: Math.max(0, asNumber(module.weight, 0)),
    calculationMethod: normalizeMethod(module.calculationMethod),
    customScript: String(module.customScript || "").trim(),
  }));

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
  if (content.scopedRules.length === 0) {
    return "请至少配置一条具体规则";
  }

  for (let index = 0; index < content.scopedRules.length; index += 1) {
    const scoped = content.scopedRules[index];
    const title = `第${index + 1}条具体规则`;

    if (scoped.applicablePeriods.length === 0) {
      return `${title}未设置适用周期`;
    }
    if (scoped.applicableObjectGroups.length === 0) {
      return `${title}未设置适用对象分组`;
    }

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
  if (!selectedRule.value) {
    return;
  }
  if (!canEditRule.value) {
    ElMessage.warning("当前规则不可编辑");
    return;
  }
  if (!editForm.ruleName.trim()) {
    ElMessage.warning("规则名不能为空");
    return;
  }

  const normalizedContent: StructuredRuleContent = {
    version: 3,
    scopedRules: ruleContent.scopedRules.map((item) => normalizeRuleForSave(item)),
  };

  const validationError = validateRuleContent(normalizedContent);
  if (validationError) {
    ElMessage.warning(validationError);
    return;
  }

  saving.value = true;
  try {
    const updated = await updateRuleFile(selectedRule.value.id, {
      assessmentId: selectedRule.value.assessmentId,
      ruleName: editForm.ruleName.trim(),
      description: editForm.description.trim(),
      contentJson: JSON.stringify(normalizedContent, null, 2),
    });
    ElMessage.success("规则已保存");
    await loadFilesOnly();
    const hit = ruleFiles.value.find((item) => item.id === updated.id);
    if (hit) {
      pickRule(hit);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存规则失败";
    ElMessage.error(message);
  } finally {
    saving.value = false;
  }
}

watch(
  () => [contextStore.sessionId, contextStore.periodCode, contextStore.objectGroupCode],
  () => {
    contextWarning.value = validateContextForLoad();
    void loadData();
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
  margin-top: 4px;
  color: #606266;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.inner-card {
  height: 100%;
}

.inner-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.rule-meta-form {
  margin-bottom: 8px;
}

.rule-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.section-block {
  margin-top: 14px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.inline-actions {
  display: flex;
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

.tag-wrap {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
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

:deep(.active-scoped-row > td) {
  background-color: #f0f9eb !important;
}
</style>
