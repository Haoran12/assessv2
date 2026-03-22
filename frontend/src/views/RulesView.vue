<template>
  <div ref="rulesViewRef" class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="card-title">规则管理</div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadData">刷新</el-button>
            <el-button
              class="save-button"
              type="primary"
              :disabled="!canEditRule || saving || !activeScopedRule"
              :loading="saving"
              @click="saveRule"
            >
              保存规则
            </el-button>
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

      <el-skeleton v-if="loadingFiles" :rows="8" animated />
      <el-empty v-else-if="!currentRule" description="当前场次暂无规则文件" />
      <template v-else>
        <el-tabs v-model="activeEditTab" class="editor-tabs">
          <el-tab-pane label="分数模块" name="modules">
            <div class="section-block">
              <el-empty
                v-if="!activeScopedRule"
                description="请先在顶部选择考核周期和考核对象分组"
              />
              <template v-else>
                <el-table
                  :data="activeScopedRule.scoreModules"
                  class="rules-table module-table"
                  :row-class-name="moduleRowClassName"
                >
                  <el-table-column label="拖动排序" width="96" align="center">
                    <template #default="{ $index }">
                      <div
                        class="drag-handle"
                        :class="{
                          'is-disabled': !canEditRule,
                          'is-dragging': draggingModuleIndex === $index,
                          'is-drop-target': moduleDropTargetIndex === $index && draggingModuleIndex !== $index,
                        }"
                        :draggable="canEditRule"
                        @dragstart="onModuleDragStart($index, $event)"
                        @dragover="onModuleDragOver($event)"
                        @dragenter.prevent="onModuleDragEnter($index, $event)"
                        @drop.prevent="onModuleDrop($index)"
                        @dragend="onModuleDragEnd"
                      >
                        <el-icon><Rank /></el-icon>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="模块名" min-width="240">
                    <template #default="{ row }">
                      <el-input v-model="row.moduleName" :disabled="!canEditRule" />
                    </template>
                  </el-table-column>
                  <el-table-column label="权重" min-width="160">
                    <template #default="{ row }">
                      <el-input-number
                        v-model="row.weight"
                        class="module-weight-input"
                        :disabled="!canEditRule"
                        :min="0"
                        :step="1"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column label="计分方式" min-width="200">
                    <template #default="{ row }">
                      <el-select
                        v-model="row.calculationMethod"
                        class="module-method-select"
                        :disabled="!canEditRule"
                        @change="handleMethodChange(row)"
                      >
                        <el-option label="直接录入" value="direct_input" />
                        <el-option label="投票模式" value="vote" />
                        <el-option label="自定义脚本" value="custom_script" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="190" fixed="right">
                    <template #default="{ row }">
                      <div class="table-row-actions">
                        <el-button size="small" type="primary" plain @click="openModuleDetail(row)">详情</el-button>
                        <el-button size="small" type="danger" plain :disabled="!canEditRule" @click="removeScoreModule(row)">删除</el-button>
                      </div>
                    </template>
                  </el-table-column>
                </el-table>
                <div
                  v-if="canEditRule"
                  class="module-drop-tail"
                  :class="{ 'is-active': moduleDropTargetIndex === -1 }"
                  @dragover="onModuleDragOverTail($event)"
                  @dragenter.prevent="onModuleDragEnterTail($event)"
                  @drop.prevent="onModuleDropToEnd"
                >
                  拖到这里可移到末尾
                </div>
                <div v-if="canEditRule" class="table-footer-actions">
                  <el-button type="primary" @click="addScoreModule">新增模块</el-button>
                  <el-button type="warning" plain :disabled="!activeScopedRule" @click="openCopyDialog">
                    从其他范围复制规则
                  </el-button>
                </div>
                <div class="formula-text">
                  总分 = Σ(模块分数 * 模块权重 / 总权重) + 额外加减分；当前总权重：{{ totalWeight.toFixed(2) }}
                </div>
              </template>
            </div>
          </el-tab-pane>

          <el-tab-pane label="等第划分" name="grades">
            <el-empty
              v-if="!activeScopedRule"
              description="请先在顶部选择考核周期和考核对象分组"
            />
            <template v-else>
              <div class="section-block">
                <div class="section-head">
                  <strong>等第规则（按行顺序从高到低匹配）</strong>
                </div>
                <el-table :data="activeScopedRule.grades" class="rules-table">
                  <el-table-column label="等第标题" min-width="170">
                    <template #default="{ row }">
                      <el-input v-model="row.title" :disabled="!canEditRule" />
                    </template>
                  </el-table-column>
                  <el-table-column label="上限" min-width="300">
                    <template #default="{ row }">
                      <div class="grade-node-cell">
                        <el-switch v-model="row.scoreNode.hasUpperLimit" :disabled="!canEditRule" />
                        <el-select
                          v-model="row.scoreNode.upperOperator"
                          class="grade-operator-select"
                          :disabled="!canEditRule || !row.scoreNode.hasUpperLimit"
                        >
                          <el-option label="<" value="<" />
                          <el-option label="≤" value="<=" />
                        </el-select>
                        <el-input-number
                          v-model="row.scoreNode.upperScore"
                          class="grade-score-input"
                          :disabled="!canEditRule || !row.scoreNode.hasUpperLimit"
                          :step="0.1"
                        />
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="下限" min-width="300">
                    <template #default="{ row }">
                      <div class="grade-node-cell">
                        <el-switch v-model="row.scoreNode.hasLowerLimit" :disabled="!canEditRule" />
                        <el-select
                          v-model="row.scoreNode.lowerOperator"
                          class="grade-operator-select"
                          :disabled="!canEditRule || !row.scoreNode.hasLowerLimit"
                        >
                          <el-option label=">" value=">" />
                          <el-option label="≥" value=">=" />
                        </el-select>
                        <el-input-number
                          v-model="row.scoreNode.lowerScore"
                          class="grade-score-input"
                          :disabled="!canEditRule || !row.scoreNode.hasLowerLimit"
                          :step="0.1"
                        />
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="区间/条件" min-width="150">
                    <template #default="{ row }">
                      <el-select v-model="row.conditionLogic" class="grade-logic-select" :disabled="!canEditRule">
                        <el-option label="AND" value="and" />
                        <el-option label="OR" value="or" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="人数上限比例(%)" min-width="180">
                    <template #default="{ row }">
                      <el-input-number
                        v-model="row.maxRatioPercent"
                        class="grade-ratio-input"
                        :disabled="!canEditRule"
                        :min="0"
                        :max="100"
                        :step="0.1"
                        placeholder="不限制"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="190" fixed="right">
                    <template #default="{ row }">
                      <div class="table-row-actions">
                        <el-button size="small" type="primary" plain @click="openGradeDetail(row)">详情</el-button>
                        <el-button size="small" type="danger" plain :disabled="!canEditRule" @click="removeGrade(row)">删除</el-button>
                      </div>
                    </template>
                  </el-table-column>
                </el-table>
                <div v-if="canEditRule" class="table-footer-actions">
                  <el-button type="primary" @click="addGrade">新增等第</el-button>
                </div>
              </div>
              <el-alert
                type="info"
                :closable="false"
                class="section-block"
                title="等第分配规则：先按顺序做首轮匹配，再按人数上限迭代回退到更低等第，直到各等第上限满足。"
              />
            </template>
          </el-tab-pane>
        </el-tabs>

        <el-collapse class="json-preview">
          <el-collapse-item title="JSON预览（只读）" name="preview">
            <el-input :model-value="structuredJsonPreview" type="textarea" :rows="12" readonly />
          </el-collapse-item>
        </el-collapse>
      </template>
    </el-card>

    <el-dialog
      v-model="copyDialogVisible"
      title="从其他考核范围复制规则"
      width="640px"
      destroy-on-close
    >
      <el-form label-width="108px" class="copy-form">
        <el-form-item label="来源场次">
          <el-select
            v-model="copySourceSessionId"
            filterable
            placeholder="请选择来源场次"
            style="width: 100%"
            @change="onCopySourceSessionChange"
          >
            <el-option
              v-for="item in sourceSessionOptions"
              :key="item.id"
              :label="item.displayName || item.assessmentName"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="来源周期">
          <el-select
            v-model="copySourcePeriodCode"
            :disabled="copySourceDetailLoading || !copySourceSessionId"
            placeholder="请选择来源周期"
            style="width: 100%"
          >
            <el-option
              v-for="item in copySourcePeriods"
              :key="item.id"
              :label="item.periodName"
              :value="item.periodCode"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="来源对象分组">
          <el-select
            v-model="copySourceObjectGroupCode"
            :disabled="copySourceDetailLoading || !copySourceSessionId"
            placeholder="请选择来源对象分组"
            style="width: 100%"
          >
            <el-option
              v-for="item in sourceObjectGroupOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <el-alert
        type="warning"
        :closable="false"
        title="复制会覆盖当前周期与对象分组下的分数模块和等第规则，请先确认来源范围。"
      />
      <template #footer>
        <el-button @click="closeCopyDialog">取消</el-button>
        <el-button
          type="primary"
          :loading="copyingFromSource"
          :disabled="!canEditRule || !activeScopedRule"
          @click="applyCopyFromSource"
        >
          覆盖复制
        </el-button>
      </template>
    </el-dialog>

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

    <el-dialog
      v-model="gradeDetailVisible"
      :title="gradeDetailTitle"
      width="760px"
      destroy-on-close
    >
      <template v-if="gradeDetailTarget">
        <div class="field-label">额外条件脚本</div>
        <el-input
          v-model="gradeDetailDraft.extraConditionScript"
          type="textarea"
          :rows="12"
          :disabled="!canEditRule"
          placeholder="可为空，复用自定义脚本"
        />
      </template>
      <template #footer>
        <el-button @click="closeGradeDetail">关闭</el-button>
        <el-button
          type="primary"
          :disabled="!canEditRule"
          @click="applyGradeDetail"
        >
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { Rank } from "@element-plus/icons-vue";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox, ElNotification } from "element-plus";
import { getAssessmentSession } from "@/api/assessment";
import { useContextStore } from "@/stores/context";
import { useUnsavedStore } from "@/stores/unsaved";
import {
  checkRuleDependencies,
  listRuleFiles,
  updateRuleFile,
} from "@/api/rules";
import type {
  AssessmentObjectGroupItem,
  AssessmentSessionItem,
  AssessmentSessionPeriodItem,
} from "@/types/assessment";
import type { RuleDependencyCheckResult, RuleFileItem } from "@/types/rules";

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
const unsavedStore = useUnsavedStore();
const dirtySourceId = "rules:editor";
const rulesViewRef = ref<HTMLElement | null>(null);

const loading = ref(false);
const loadingFiles = ref(false);
const saving = ref(false);
const draggingModuleIndex = ref<number | null>(null);
const draggingModuleId = ref("");
const moduleDropTargetIndex = ref<number | null>(null);

const currentRule = ref<RuleFileItem | null>(null);
const activeScopedRuleId = ref("");
const activeEditTab = ref<"modules" | "grades">("modules");

const moduleDetailVisible = ref(false);
const moduleDetailTargetId = ref("");
const moduleDetailDraft = reactive({
  customScript: "",
  voteConfigJson: "",
});
const gradeDetailVisible = ref(false);
const gradeDetailTargetId = ref("");
const gradeDetailDraft = reactive({
  extraConditionScript: "",
});
const copyDialogVisible = ref(false);
const copyingFromSource = ref(false);
const copySourceDetailLoading = ref(false);
const copySourceSessionId = ref<number | undefined>(undefined);
const copySourcePeriodCode = ref("");
const copySourceObjectGroupCode = ref("");
const copySourcePeriods = ref<AssessmentSessionPeriodItem[]>([]);
const copySourceObjectGroups = ref<AssessmentObjectGroupItem[]>([]);
const ruleEditorBaseline = ref("");

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

const sourceSessionOptions = computed<AssessmentSessionItem[]>(() => contextStore.sessions);

const sourceObjectGroupOptions = computed(() =>
  [...copySourceObjectGroups.value]
    .sort((a, b) => {
      if (a.sortOrder !== b.sortOrder) {
        return a.sortOrder - b.sortOrder;
      }
      return a.id - b.id;
    })
    .map((item) => ({
      value: item.groupCode,
      label: `${item.objectType === "team" ? "团体" : "个人"} - ${item.groupName}`,
    })),
);

const moduleDetailTarget = computed(() =>
  activeScopedRule.value?.scoreModules.find((item) => item.id === moduleDetailTargetId.value) || null,
);

const moduleDetailTitle = computed(() => {
  const moduleName = moduleDetailTarget.value?.moduleName?.trim() || "模块";
  return `${moduleName}详情`;
});

const gradeDetailTarget = computed(() =>
  activeScopedRule.value?.grades.find((item) => item.id === gradeDetailTargetId.value) || null,
);

const gradeDetailTitle = computed(() => {
  const gradeName = gradeDetailTarget.value?.title?.trim() || "等第";
  return `${gradeName}详情`;
});

function ruleEditorSignature(): string {
  return JSON.stringify({
    ruleID: currentRule.value?.id ?? null,
    content: normalizeRuleContent(cloneDeep(ruleContent)),
  });
}

function resetRuleEditorBaseline(): void {
  ruleEditorBaseline.value = ruleEditorSignature();
  unsavedStore.clearDirty(dirtySourceId);
}

function syncRuleEditorDirty(): void {
  if (!currentRule.value || !ruleEditorBaseline.value || !canEditRule.value) {
    unsavedStore.clearDirty(dirtySourceId);
    return;
  }
  const current = ruleEditorSignature();
  if (current === ruleEditorBaseline.value) {
    unsavedStore.clearDirty(dirtySourceId);
    return;
  }
  unsavedStore.markDirty(dirtySourceId);
}

function isDialogCancel(error: unknown): boolean {
  return (
    error === "cancel" ||
    error === "close" ||
    (error instanceof Error && (error.message === "cancel" || error.message === "close"))
  );
}

function hasBlockingDialogOpen(): boolean {
  return copyDialogVisible.value || moduleDetailVisible.value || gradeDetailVisible.value;
}

function isRulesViewShortcutScope(event: KeyboardEvent): boolean {
  const root = rulesViewRef.value;
  const target = event.target;
  if (!root || !(target instanceof Node)) {
    return false;
  }
  return root.contains(target);
}

function handleGlobalEditorKeydown(event: KeyboardEvent): void {
  const ctrlOrMeta = event.ctrlKey || event.metaKey;
  if (!ctrlOrMeta || event.altKey) {
    return;
  }
  if (!isRulesViewShortcutScope(event)) {
    return;
  }
  if (hasBlockingDialogOpen()) {
    return;
  }

  const key = String(event.key || "").toLowerCase();
  if (key === "s") {
    event.preventDefault();
    void saveRule();
    return;
  }
  if (key === "n") {
    event.preventDefault();
    if (activeEditTab.value === "modules") {
      addScoreModule();
      return;
    }
    addGrade();
  }
}

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
    ruleEditorBaseline.value = "";
    unsavedStore.clearDirty(dirtySourceId);
    return;
  }
  const parsed = parseRuleContent(rule.contentJson || "", true);
  Object.assign(ruleContent, parsed);
  syncActiveScopedRuleWithContext();
  resetRuleEditorBaseline();
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
  const periodCode = contextStore.periodCode;
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

function clearModuleDragState(): void {
  draggingModuleIndex.value = null;
  draggingModuleId.value = "";
  moduleDropTargetIndex.value = null;
}

function moveModuleToIndex(targetIndex: number): void {
  if (!activeScopedRule.value) {
    return;
  }
  const fromIndex = draggingModuleIndex.value;
  if (fromIndex === null || fromIndex === targetIndex) {
    return;
  }
  const modules = activeScopedRule.value.scoreModules;
  if (fromIndex < 0 || fromIndex >= modules.length || targetIndex < 0 || targetIndex >= modules.length) {
    return;
  }
  const [moved] = modules.splice(fromIndex, 1);
  const insertIndex = targetIndex;
  modules.splice(insertIndex, 0, moved);
  draggingModuleIndex.value = insertIndex;
  moduleDropTargetIndex.value = insertIndex;
}

function moveModuleToEnd(): void {
  if (!activeScopedRule.value) {
    return;
  }
  const fromIndex = draggingModuleIndex.value;
  if (fromIndex === null) {
    return;
  }
  const modules = activeScopedRule.value.scoreModules;
  if (fromIndex < 0 || fromIndex >= modules.length) {
    return;
  }
  const [moved] = modules.splice(fromIndex, 1);
  modules.push(moved);
  draggingModuleIndex.value = modules.length - 1;
  moduleDropTargetIndex.value = -1;
}

function onModuleDragStart(index: number, event: DragEvent): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    event.preventDefault();
    return;
  }
  draggingModuleIndex.value = index;
  draggingModuleId.value = activeScopedRule.value.scoreModules[index]?.id || "";
  moduleDropTargetIndex.value = index;
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

function onModuleDragEnter(targetIndex: number, event: DragEvent): void {
  if (!canEditRule.value || draggingModuleIndex.value === null) {
    return;
  }
  event.preventDefault();
  moveModuleToIndex(targetIndex);
}

function onModuleDragOverTail(event: DragEvent): void {
  onModuleDragOver(event);
  if (!canEditRule.value || draggingModuleIndex.value === null) {
    return;
  }
  moduleDropTargetIndex.value = -1;
}

function onModuleDragEnterTail(event: DragEvent): void {
  if (!canEditRule.value || draggingModuleIndex.value === null) {
    return;
  }
  event.preventDefault();
  moveModuleToEnd();
}

function onModuleDrop(targetIndex: number): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    clearModuleDragState();
    return;
  }
  moveModuleToIndex(targetIndex);
  clearModuleDragState();
}

function onModuleDropToEnd(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    clearModuleDragState();
    return;
  }
  moveModuleToEnd();
  clearModuleDragState();
}

function onModuleDragEnd(): void {
  clearModuleDragState();
}

function moduleRowClassName({
  row,
}: {
  row: ScoreModule;
  rowIndex: number;
}): string {
  if (!draggingModuleId.value) {
    return "";
  }
  return row.id === draggingModuleId.value ? "is-module-dragging-row" : "";
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

function openGradeDetail(grade: GradeRule): void {
  gradeDetailTargetId.value = grade.id;
  gradeDetailDraft.extraConditionScript = grade.extraConditionScript || "";
  gradeDetailVisible.value = true;
}

function closeGradeDetail(): void {
  gradeDetailVisible.value = false;
  gradeDetailTargetId.value = "";
  gradeDetailDraft.extraConditionScript = "";
}

function applyGradeDetail(): void {
  const target = gradeDetailTarget.value;
  if (!target) {
    closeGradeDetail();
    return;
  }
  target.extraConditionScript = String(gradeDetailDraft.extraConditionScript || "");
  closeGradeDetail();
}

function closeCopyDialog(): void {
  copyDialogVisible.value = false;
}

async function onCopySourceSessionChange(sessionID?: number): Promise<void> {
  copySourceSessionId.value = sessionID;
  copySourcePeriods.value = [];
  copySourceObjectGroups.value = [];
  copySourcePeriodCode.value = "";
  copySourceObjectGroupCode.value = "";

  if (!sessionID) {
    return;
  }

  copySourceDetailLoading.value = true;
  try {
    const detail = await getAssessmentSession(sessionID);
    copySourcePeriods.value = detail.periods || [];
    copySourceObjectGroups.value = detail.objectGroups || [];

    const preferredPeriod = contextStore.periodCode && copySourcePeriods.value.some((item) => item.periodCode === contextStore.periodCode)
      ? contextStore.periodCode
      : copySourcePeriods.value[0]?.periodCode || "";
    const preferredGroup =
      contextStore.objectGroupCode && copySourceObjectGroups.value.some((item) => item.groupCode === contextStore.objectGroupCode)
        ? contextStore.objectGroupCode
        : copySourceObjectGroups.value[0]?.groupCode || "";
    copySourcePeriodCode.value = preferredPeriod || "";
    copySourceObjectGroupCode.value = preferredGroup || "";
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载来源场次配置失败";
    ElMessage.error(message);
  } finally {
    copySourceDetailLoading.value = false;
  }
}

async function openCopyDialog(): Promise<void> {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  try {
    await contextStore.ensureInitialized();
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载场次列表失败";
    ElMessage.error(message);
    return;
  }
  if (sourceSessionOptions.value.length === 0) {
    ElMessage.warning("暂无可选来源场次");
    return;
  }

  const current = copySourceSessionId.value;
  const currentValid = current && sourceSessionOptions.value.some((item) => item.id === current);
  const defaultSessionID = currentValid
    ? current
    : sourceSessionOptions.value.find((item) => item.id !== contextStore.sessionId)?.id || sourceSessionOptions.value[0].id;

  copyDialogVisible.value = true;
  await onCopySourceSessionChange(defaultSessionID);
}

async function applyCopyFromSource(): Promise<void> {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  if (!copySourceSessionId.value || !copySourcePeriodCode.value || !copySourceObjectGroupCode.value) {
    ElMessage.warning("请先选择完整的来源场次、周期与对象分组");
    return;
  }

  copyingFromSource.value = true;
  try {
    const sourceItems = await listRuleFiles(copySourceSessionId.value, false);
    if (sourceItems.length === 0) {
      ElMessage.warning("来源场次暂无规则文件");
      return;
    }
    const sourceContent = parseRuleContent(sourceItems[0].contentJson || "", false);
    const sourceScopedRule = sourceContent.scopedRules.find(
      (item) =>
        item.applicablePeriods.includes(copySourcePeriodCode.value) &&
        item.applicableObjectGroups.includes(copySourceObjectGroupCode.value),
    );
    if (!sourceScopedRule) {
      ElMessage.warning("来源范围未配置规则，无法复制");
      return;
    }

    const sourceSessionName =
      sourceSessionOptions.value.find((item) => item.id === copySourceSessionId.value)?.displayName ||
      sourceSessionOptions.value.find((item) => item.id === copySourceSessionId.value)?.assessmentName ||
      `场次#${copySourceSessionId.value}`;
    await ElMessageBox.confirm(
      `确认从「${sourceSessionName} / ${copySourcePeriodCode.value} / ${copySourceObjectGroupCode.value}」复制并覆盖当前范围规则吗？`,
      "复制确认",
      {
        type: "warning",
        confirmButtonText: "覆盖复制",
        cancelButtonText: "取消",
      },
    );

    activeScopedRule.value.scoreModules = sourceScopedRule.scoreModules.map((item, index) =>
      normalizeScoreModule(
        {
          ...cloneDeep(item),
          id: uuid("module"),
          moduleKey: String(item.moduleKey || `module_${index + 1}`).trim() || `module_${index + 1}`,
        },
        index,
      ),
    );
    activeScopedRule.value.grades = sourceScopedRule.grades.map((item, index) =>
      normalizeGrade(
        {
          ...cloneDeep(item),
          id: uuid("grade"),
        },
        index,
      ),
    );
    closeModuleDetail();
    closeGradeDetail();
    closeCopyDialog();
    ElMessage.success("复制成功，请保存规则");
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    const message = error instanceof Error ? error.message : "复制规则失败";
    ElMessage.error(message);
  } finally {
    copyingFromSource.value = false;
  }
}

function addScoreModule(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.scoreModules.push(newScoreModule(`模块${activeScopedRule.value.scoreModules.length + 1}`, 0));
}

async function removeScoreModule(module: ScoreModule): Promise<void> {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  const moduleName = module.moduleName.trim() || "未命名模块";
  try {
    await ElMessageBox.confirm(`确认删除模块「${moduleName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    activeScopedRule.value.scoreModules = activeScopedRule.value.scoreModules.filter((item) => item.id !== module.id);
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("删除模块失败");
  }
}

function addGrade(): void {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  activeScopedRule.value.grades.push(newGrade(`等第${activeScopedRule.value.grades.length + 1}`));
}

async function removeGrade(grade: GradeRule): Promise<void> {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  const gradeName = grade.title.trim() || "未命名等第";
  try {
    await ElMessageBox.confirm(`确认删除等第「${gradeName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    activeScopedRule.value.grades = activeScopedRule.value.grades.filter((item) => item.id !== grade.id);
    if (gradeDetailTargetId.value === grade.id) {
      closeGradeDetail();
    }
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("删除等第失败");
  }
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

function formatDependencyIssueLine(result: RuleDependencyCheckResult, index: number): string {
  const issue = result.issues[index];
  if (!issue) {
    return "";
  }
  const pathText = Array.isArray(issue.path) && issue.path.length > 0 ? ` (${issue.path.join(" -> ")})` : "";
  return `${index + 1}. [${issue.severity}] ${issue.code}: ${issue.message}${pathText}`;
}

function notifyDependencyCheckResult(result: RuleDependencyCheckResult): void {
  const errorCount = Number(result?.summary?.errorCount || 0);
  const warningCount = Number(result?.summary?.warningCount || 0);
  const total = errorCount + warningCount;
  if (total <= 0) {
    return;
  }
  const showCount = Math.min(5, result.issues.length);
  const lines: string[] = [];
  for (let index = 0; index < showCount; index += 1) {
    const line = formatDependencyIssueLine(result, index);
    if (line) {
      lines.push(line);
    }
  }
  const remain = result.issues.length - showCount;
  if (remain > 0) {
    lines.push(`... and ${remain} more issue(s).`);
  }

  const title =
    errorCount > 0
      ? `Dependency check found ${errorCount} error(s), ${warningCount} warning(s)`
      : `Dependency check found ${warningCount} warning(s)`;
  ElNotification({
    title,
    type: errorCount > 0 ? "error" : "warning",
    duration: 12000,
    message: lines.join("\n"),
  });
}

async function runDependencyCheckAfterSave(ruleId: number): Promise<void> {
  try {
    const result = await checkRuleDependencies(ruleId);
    notifyDependencyCheckResult(result);
  } catch (error) {
    const message = error instanceof Error ? error.message : "unknown error";
    ElMessage.warning(`Rule saved, but dependency check failed: ${message}`);
  }
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
    void runDependencyCheckAfterSave(updated.id);
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
    closeCopyDialog();
    void loadData();
  },
);

watch(
  () => [contextStore.periodCode, contextStore.objectGroupCode],
  () => {
    contextWarning.value = validateContextForLoad();
    syncActiveScopedRuleWithContext();
    closeCopyDialog();
    closeModuleDetail();
    closeGradeDetail();
  },
);

watch(
  () => activeScopedRuleId.value,
  () => {
    closeCopyDialog();
    closeModuleDetail();
    closeGradeDetail();
  },
);

watch(
  () => ruleContent,
  () => {
    syncRuleEditorDirty();
  },
  { deep: true },
);

watch(
  () => [currentRule.value?.id, canEditRule.value],
  () => {
    syncRuleEditorDirty();
  },
);

onMounted(async () => {
  window.addEventListener("keydown", handleGlobalEditorKeydown);
  unsavedStore.setSourceMeta(dirtySourceId, {
    label: "规则管理",
    save: saveRule,
  });
  await loadData();
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleGlobalEditorKeydown);
  unsavedStore.unregisterSource(dirtySourceId);
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

.card-title {
  color: #303133;
  font-size: 16px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.save-button {
  min-width: 110px;
  font-weight: 600;
}

.editor-tabs {
  margin-top: 4px;
}

.editor-tabs :deep(.el-tabs__header) {
  margin-bottom: 8px;
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

.grade-node-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
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
  transform: scale(1.04);
}

.drag-handle.is-drop-target {
  background: #f0f9eb;
  border-color: #67c23a;
  color: #67c23a;
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
  transition: border-color 0.2s ease, color 0.2s ease, background-color 0.2s ease;
}

.module-drop-tail.is-active {
  border-color: #409eff;
  color: #409eff;
  background: #ecf5ff;
}

.rules-table {
  width: 100%;
}

.rules-table :deep(.module-weight-input),
.rules-table :deep(.module-method-select),
.rules-table :deep(.grade-logic-select),
.rules-table :deep(.grade-ratio-input) {
  width: 100%;
}

.rules-table :deep(.grade-operator-select) {
  width: 72px;
  flex: 0 0 72px;
}

.rules-table :deep(.grade-score-input) {
  width: auto;
  min-width: 0;
  flex: 1;
}

.rules-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f5f9ff;
}

.module-table :deep(.el-table__body tr.is-module-dragging-row > td.el-table__cell) {
  background: #ecf5ff;
  transition: background-color 0.2s ease;
}

.table-row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.table-footer-actions {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.copy-form {
  margin-bottom: 10px;
}
</style>
