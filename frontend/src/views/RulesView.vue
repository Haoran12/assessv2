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
            <el-button
              v-if="isRoot"
              type="primary"
              :disabled="!contextStore.sessionId"
              @click="createGlobalRule"
            >
              新建基础规则
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

      <el-row :gutter="12">
        <el-col :md="10" :sm="24" :xs="24">
          <el-card shadow="never" class="inner-card">
            <template #header>
              <div class="inner-header">
                <strong>规则文件库</strong>
                <el-switch
                  v-model="includeHidden"
                  active-text="显示隐藏"
                  inactive-text="隐藏过滤"
                  @change="loadFilesOnly"
                />
              </div>
            </template>

            <el-table v-loading="loadingFiles" :data="ruleFiles" border height="600" @row-click="pickRule">
              <el-table-column prop="id" label="ID" width="80" />
              <el-table-column label="名称" min-width="180">
                <template #default="{ row }">
                  <div class="rule-name-cell">
                    <span>{{ row.ruleName }}</span>
                    <el-tag size="small" :type="row.isCopy ? 'success' : 'info'">
                      {{ row.isCopy ? "拷贝" : "基础" }}
                    </el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="190" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :disabled="!canBind" @click.stop="bindRule(row)">绑定</el-button>
                  <el-button
                    v-if="!row.isCopy && !row.hiddenByCurrentOrg && !isRoot"
                    link
                    @click.stop="hideRule(row)"
                  >
                    隐藏
                  </el-button>
                  <el-button
                    v-if="!row.isCopy && row.hiddenByCurrentOrg && !isRoot"
                    link
                    type="warning"
                    @click.stop="unhideRule(row)"
                  >
                    恢复
                  </el-button>
                  <el-button v-if="row.canDelete" link type="danger" @click.stop="removeRule(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </el-col>

        <el-col :md="14" :sm="24" :xs="24">
          <el-card shadow="never" class="inner-card">
            <template #header>
              <div class="inner-header">
                <strong>结构化规则编辑</strong>
                <el-tag v-if="activeBindingRuleName" type="warning">当前绑定：{{ activeBindingRuleName }}</el-tag>
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
                  <strong>分数计算（两级）</strong>
                  <el-button size="small" :disabled="!canEditRule" @click="addModule">新增模块</el-button>
                </div>
                <el-table :data="scoreModules" border>
                  <el-table-column label="模块名称" min-width="160">
                    <template #default="{ row }">
                      <span>{{ row.moduleName }}</span>
                      <el-tag v-if="row.isExtra" size="small" type="warning" class="ml-6">额外</el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="权重" width="110">
                    <template #default="{ row }">
                      <span>{{ row.isExtra ? "-" : row.weight }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="计分方式" width="130">
                    <template #default="{ row }">
                      {{ methodLabel(row.method) }}
                    </template>
                  </el-table-column>
                  <el-table-column label="二级子模块" width="120">
                    <template #default="{ row }">
                      {{ row.subModules.length }}
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="160" fixed="right">
                    <template #default="{ row }">
                      <el-button link type="primary" @click="openModuleDetail(row)">详情</el-button>
                      <el-button
                        link
                        type="danger"
                        :disabled="!canEditRule || row.isExtra"
                        @click="removeModule(row)"
                      >
                        删除
                      </el-button>
                    </template>
                  </el-table-column>
                </el-table>
                <div class="formula-text">
                  总权重：{{ totalWeight }}。总分 = Σ(单模块分数 * 权重 / Σ总权重) + 额外加减分
                </div>
              </div>

              <div class="section-block">
                <div class="section-head">
                  <strong>等第划分</strong>
                  <el-button size="small" :disabled="!canEditRule" @click="addGradeRule">新增等第</el-button>
                </div>
                <el-table :data="ruleContent.gradeRules" border>
                  <el-table-column label="等第" width="120">
                    <template #default="{ row }">
                      <el-input v-model="row.grade" :disabled="!canEditRule" />
                    </template>
                  </el-table-column>
                  <el-table-column label="最低分" width="140">
                    <template #default="{ row }">
                      <el-input-number v-model="row.min" :disabled="!canEditRule" :step="0.1" />
                    </template>
                  </el-table-column>
                  <el-table-column label="最高分" width="140">
                    <template #default="{ row }">
                      <el-input-number v-model="row.max" :disabled="!canEditRule" :step="0.1" />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="100">
                    <template #default="{ $index }">
                      <el-button link type="danger" :disabled="!canEditRule" @click="removeGradeRule($index)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>

              <div class="editor-actions">
                <el-button type="primary" :disabled="!canEditRule || saving" :loading="saving" @click="saveRule">
                  保存规则
                </el-button>
              </div>

              <el-collapse class="json-preview">
                <el-collapse-item title="JSON预览（只读）" name="preview">
                  <el-input :model-value="structuredJsonPreview" type="textarea" :rows="10" readonly />
                </el-collapse-item>
              </el-collapse>
            </template>
          </el-card>
        </el-col>
      </el-row>
    </el-card>

    <el-dialog v-model="moduleDialogVisible" title="模块详情" width="900px">
      <el-form label-width="110px" class="module-dialog-form">
        <el-form-item label="模块名称" required>
          <el-input v-model="moduleForm.moduleName" :disabled="!canEditRule" />
        </el-form-item>
        <el-form-item label="权重" v-if="!moduleForm.isExtra">
          <el-input-number v-model="moduleForm.weight" :disabled="!canEditRule" :min="0" :step="1" />
        </el-form-item>
        <el-form-item label="计分方式">
          <el-select v-model="moduleForm.method" :disabled="!canEditRule" style="width: 220px">
            <el-option label="直接录入" value="direct_input" />
            <el-option label="投票" value="vote" />
            <el-option label="自定义脚本" value="custom_script" />
          </el-select>
        </el-form-item>

        <el-form-item label="直接录入配置" v-if="moduleForm.method === 'direct_input'">
          <div class="inline-form-row">
            <el-input-number v-model="moduleForm.detail.directInput.min" :disabled="!canEditRule" :step="0.1" />
            <span class="inline-label">最小分</span>
            <el-input-number v-model="moduleForm.detail.directInput.max" :disabled="!canEditRule" :step="0.1" />
            <span class="inline-label">最大分</span>
          </div>
        </el-form-item>

        <el-form-item label="投票配置" v-if="moduleForm.method === 'vote'">
          <el-input
            v-model="moduleForm.detail.vote.ballotTemplate"
            :disabled="!canEditRule"
            type="textarea"
            :rows="2"
            placeholder="描述投票模板或规则说明"
          />
        </el-form-item>

        <el-form-item label="脚本配置" v-if="moduleForm.method === 'custom_script'">
          <el-input
            v-model="moduleForm.detail.customScript.script"
            :disabled="!canEditRule"
            type="textarea"
            :rows="6"
            placeholder="填写自定义评分脚本"
          />
        </el-form-item>

        <el-form-item label="二级子模块">
          <div class="submodule-editor">
            <div class="submodule-toolbar">
              <el-button size="small" :disabled="!canEditRule" @click="addSubModule">新增子模块</el-button>
            </div>
            <el-table :data="moduleForm.subModules" border>
              <el-table-column label="名称" min-width="180">
                <template #default="{ row }">
                  <el-input v-model="row.name" :disabled="!canEditRule" />
                </template>
              </el-table-column>
              <el-table-column label="权重" width="120">
                <template #default="{ row }">
                  <el-input-number v-model="row.weight" :disabled="!canEditRule" :min="0" :step="1" />
                </template>
              </el-table-column>
              <el-table-column label="计分方式" width="140">
                <template #default="{ row }">
                  <el-select v-model="row.method" :disabled="!canEditRule" style="width: 120px">
                    <el-option label="直接录入" value="direct_input" />
                    <el-option label="投票" value="vote" />
                    <el-option label="脚本" value="custom_script" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100">
                <template #default="{ $index }">
                  <el-button link type="danger" :disabled="!canEditRule" @click="removeSubModule($index)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="moduleDialogVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!canEditRule" @click="saveModuleDetail">保存模块</el-button>
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
  createRuleFile,
  deleteRuleFile,
  hideRuleFile,
  listRuleBindings,
  listRuleFiles,
  selectRuleBinding,
  unhideRuleFile,
  updateRuleFile,
} from "@/api/rules";
import type { RuleBindingItem, RuleFileItem } from "@/types/rules";

type ScoreMethod = "direct_input" | "vote" | "custom_script";

interface RuleDetailConfig {
  directInput: {
    min: number;
    max: number;
  };
  vote: {
    ballotTemplate: string;
  };
  customScript: {
    script: string;
  };
}

interface ScoreSubModule {
  id: string;
  name: string;
  weight: number;
  method: ScoreMethod;
}

interface ScoreModule {
  moduleKey: string;
  moduleName: string;
  weight: number;
  method: ScoreMethod;
  isExtra: boolean;
  subModules: ScoreSubModule[];
  detail: RuleDetailConfig;
}

interface GradeRule {
  grade: string;
  min: number;
  max: number;
}

interface StructuredRuleContent {
  version: number;
  scoreModules: ScoreModule[];
  gradeRules: GradeRule[];
}

const appStore = useAppStore();
const contextStore = useContextStore();

const loading = ref(false);
const loadingFiles = ref(false);
const saving = ref(false);

const includeHidden = ref(false);
const ruleFiles = ref<RuleFileItem[]>([]);
const bindings = ref<RuleBindingItem[]>([]);
const selectedRule = ref<RuleFileItem | null>(null);
const editForm = reactive({
  ruleName: "",
  description: "",
});

const ruleContent = reactive<StructuredRuleContent>(defaultRuleContent());

const moduleDialogVisible = ref(false);
const moduleEditingKey = ref("");
const moduleForm = reactive<ScoreModule>(newRegularModule("", 0));

const contextWarning = ref("");

const isRoot = computed(() => appStore.primaryRole === "root" || appStore.roles.includes("root"));
const canBind = computed(
  () =>
    appStore.hasPermission("rule:update") &&
    !!contextStore.sessionId &&
    !!contextStore.periodCode &&
    !!contextStore.objectGroupCode,
);
const canEditRule = computed(() => !!selectedRule.value?.canEdit);

const scoreModules = computed(() => ruleContent.scoreModules);
const totalWeight = computed(() =>
  ruleContent.scoreModules.filter((item) => !item.isExtra).reduce((sum, item) => sum + asNumber(item.weight, 0), 0),
);

const structuredJsonPreview = computed(() => JSON.stringify(normalizeRuleContent(ruleContent), null, 2));

const contextText = computed(() => {
  const sessionText = contextStore.currentSession?.displayName || "未选择场次";
  const periodText = contextStore.currentPeriod?.periodName || "未选择周期";
  const groupText = contextStore.currentObjectGroup?.groupName || "未选择对象类型";
  return `当前上下文：${sessionText} / ${periodText} / ${groupText}`;
});

const activeBinding = computed(() =>
  bindings.value.find(
    (item) => item.periodCode === contextStore.periodCode && item.objectGroupCode === contextStore.objectGroupCode,
  ),
);

const activeBindingRuleName = computed(() => activeBinding.value?.ruleFile?.ruleName || "");

function defaultDetailConfig(): RuleDetailConfig {
  return {
    directInput: {
      min: 0,
      max: 100,
    },
    vote: {
      ballotTemplate: "",
    },
    customScript: {
      script: "",
    },
  };
}

function newSubModule(seed: string, index: number): ScoreSubModule {
  return {
    id: `sub_${Date.now()}_${index}`,
    name: seed || `子模块${index + 1}`,
    weight: 100,
    method: "direct_input",
  };
}

function newRegularModule(seed: string, index: number): ScoreModule {
  const key = `module_${Date.now()}_${index}`;
  return {
    moduleKey: key,
    moduleName: seed || `模块${index + 1}`,
    weight: 10,
    method: "direct_input",
    isExtra: false,
    subModules: [newSubModule("", 0)],
    detail: defaultDetailConfig(),
  };
}

function newExtraModule(): ScoreModule {
  return {
    moduleKey: "extra_adjust",
    moduleName: "额外加减分模块",
    weight: 0,
    method: "direct_input",
    isExtra: true,
    subModules: [],
    detail: defaultDetailConfig(),
  };
}

function defaultRuleContent(): StructuredRuleContent {
  return {
    version: 2,
    scoreModules: [
      {
        moduleKey: "base_performance",
        moduleName: "基础绩效",
        weight: 100,
        method: "direct_input",
        isExtra: false,
        subModules: [
          {
            id: "base_daily",
            name: "日常工作",
            weight: 100,
            method: "direct_input",
          },
        ],
        detail: defaultDetailConfig(),
      },
      newExtraModule(),
    ],
    gradeRules: [
      { grade: "A", min: 90, max: 100 },
      { grade: "B", min: 80, max: 89.99 },
      { grade: "C", min: 70, max: 79.99 },
      { grade: "D", min: 0, max: 69.99 },
    ],
  };
}

function methodLabel(method: ScoreMethod): string {
  switch (method) {
    case "direct_input":
      return "直接录入";
    case "vote":
      return "投票";
    case "custom_script":
      return "自定义脚本";
    default:
      return method;
  }
}

function normalizeMethod(value: unknown): ScoreMethod {
  const text = String(value || "").trim().toLowerCase();
  if (text === "vote" || text === "voting") {
    return "vote";
  }
  if (text === "custom_script" || text === "script" || text === "custom") {
    return "custom_script";
  }
  return "direct_input";
}

function asNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return parsed;
}

function cloneContent(content: StructuredRuleContent): StructuredRuleContent {
  return JSON.parse(JSON.stringify(content)) as StructuredRuleContent;
}

function normalizeSubModule(raw: any, index: number): ScoreSubModule {
  return {
    id: String(raw?.id || `sub_${Date.now()}_${index}`),
    name: String(raw?.name || raw?.moduleName || `子模块${index + 1}`).trim(),
    weight: Math.max(0, asNumber(raw?.weight, 0)),
    method: normalizeMethod(raw?.method),
  };
}

function normalizeModule(raw: any, index: number): ScoreModule {
  const isExtra = Boolean(raw?.isExtra);
  const key = String(raw?.moduleKey || `module_${index + 1}`).trim() || `module_${index + 1}`;
  const subModulesRaw = Array.isArray(raw?.subModules) ? raw.subModules : [];
  const subModules = subModulesRaw.map((item, subIndex) => normalizeSubModule(item, subIndex));
  return {
    moduleKey: isExtra ? "extra_adjust" : key,
    moduleName: String(raw?.moduleName || `模块${index + 1}`).trim(),
    weight: isExtra ? 0 : Math.max(0, asNumber(raw?.weight, 0)),
    method: normalizeMethod(raw?.method),
    isExtra,
    subModules,
    detail: {
      directInput: {
        min: asNumber(raw?.detail?.directInput?.min, 0),
        max: asNumber(raw?.detail?.directInput?.max, 100),
      },
      vote: {
        ballotTemplate: String(raw?.detail?.vote?.ballotTemplate || ""),
      },
      customScript: {
        script: String(raw?.detail?.customScript?.script || ""),
      },
    },
  };
}

function normalizeRuleContent(input: StructuredRuleContent | Record<string, any>): StructuredRuleContent {
  const raw = input as any;
  const sourceModules = Array.isArray(raw?.scoreModules)
    ? raw.scoreModules
    : Array.isArray(raw?.scoreCalculation?.modules)
      ? raw.scoreCalculation.modules
      : [];

  const modules = sourceModules.map((item, index) => normalizeModule(item, index));
  const regularModules = modules.filter((item) => !item.isExtra);
  const extraModules = modules.filter((item) => item.isExtra);
  const normalizedModules = [...regularModules, extraModules[0] || newExtraModule()];

  const gradeRaw = Array.isArray(raw?.gradeRules)
    ? raw.gradeRules
    : Array.isArray(raw?.grade?.rules)
      ? raw.grade.rules
      : [];
  const gradeRules = gradeRaw
    .map((item: any) => ({
      grade: String(item?.grade || "").trim(),
      min: asNumber(item?.min, 0),
      max: asNumber(item?.max, 0),
    }))
    .filter((item: GradeRule) => item.grade !== "");

  return {
    version: asNumber(raw?.version, 2),
    scoreModules: normalizedModules,
    gradeRules: gradeRules.length > 0 ? gradeRules : cloneContent(defaultRuleContent()).gradeRules,
  };
}

function parseRuleContent(raw: string): StructuredRuleContent {
  const text = String(raw || "").trim();
  if (!text) {
    return cloneContent(defaultRuleContent());
  }
  try {
    const parsed = JSON.parse(text);
    return normalizeRuleContent(parsed as Record<string, any>);
  } catch (_error) {
    return cloneContent(defaultRuleContent());
  }
}

function fillEditor(rule: RuleFileItem | null): void {
  if (!rule) {
    editForm.ruleName = "";
    editForm.description = "";
    Object.assign(ruleContent, defaultRuleContent());
    return;
  }
  editForm.ruleName = rule.ruleName;
  editForm.description = rule.description || "";
  Object.assign(ruleContent, parseRuleContent(rule.contentJson || ""));
}

function pickRule(row: RuleFileItem): void {
  selectedRule.value = row;
  fillEditor(row);
}

function validateContext(): string {
  if (!contextStore.sessionId) {
    return "请先选择考核场次";
  }
  if (!contextStore.periodCode) {
    return "请先选择周期";
  }
  if (!contextStore.objectGroupCode) {
    return "请先选择考核对象类型";
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
    const items = await listRuleFiles(contextStore.sessionId, includeHidden.value);
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

async function loadBindingsOnly(): Promise<void> {
  if (!contextStore.sessionId) {
    bindings.value = [];
    return;
  }
  bindings.value = await listRuleBindings(contextStore.sessionId);
}

async function loadData(): Promise<void> {
  loading.value = true;
  try {
    await contextStore.ensureInitialized();
    contextWarning.value = validateContext();
    await Promise.all([loadFilesOnly(), loadBindingsOnly()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载规则管理数据失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

async function bindRule(rule: RuleFileItem): Promise<void> {
  if (!canBind.value || !contextStore.sessionId) {
    ElMessage.warning("请先补全顶部上下文");
    return;
  }
  try {
    const binding = await selectRuleBinding({
      assessmentId: contextStore.sessionId,
      periodCode: contextStore.periodCode,
      objectGroupCode: contextStore.objectGroupCode,
      sourceRuleId: rule.id,
    });
    ElMessage.success("规则已绑定并自动创建组织拷贝");
    await Promise.all([loadFilesOnly(), loadBindingsOnly()]);
    const hit = ruleFiles.value.find((item) => item.id === binding.ruleFile.id);
    if (hit) {
      pickRule(hit);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "绑定规则失败";
    ElMessage.error(message);
  }
}

async function createGlobalRule(): Promise<void> {
  if (!contextStore.sessionId) {
    ElMessage.warning("请先选择考核场次");
    return;
  }
  try {
    const created = await createRuleFile({
      assessmentId: contextStore.sessionId,
      ruleName: `基础规则-${Date.now()}`,
      description: "Root 创建的基础规则",
      contentJson: JSON.stringify(defaultRuleContent(), null, 2),
    });
    ElMessage.success("基础规则已创建");
    await loadFilesOnly();
    const hit = ruleFiles.value.find((item) => item.id === created.id);
    if (hit) {
      pickRule(hit);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "创建规则失败";
    ElMessage.error(message);
  }
}

function addModule(): void {
  if (!canEditRule.value) {
    return;
  }
  const extraIndex = ruleContent.scoreModules.findIndex((item) => item.isExtra);
  const insertIndex = extraIndex >= 0 ? extraIndex : ruleContent.scoreModules.length;
  ruleContent.scoreModules.splice(insertIndex, 0, newRegularModule("", insertIndex));
}

function removeModule(module: ScoreModule): void {
  if (!canEditRule.value) {
    return;
  }
  if (module.isExtra) {
    ElMessage.warning("额外加减分模块不可删除");
    return;
  }
  const index = ruleContent.scoreModules.findIndex((item) => item.moduleKey === module.moduleKey);
  if (index >= 0) {
    ruleContent.scoreModules.splice(index, 1);
  }
}

function openModuleDetail(module: ScoreModule): void {
  moduleEditingKey.value = module.moduleKey;
  Object.assign(moduleForm, JSON.parse(JSON.stringify(module)) as ScoreModule);
  moduleDialogVisible.value = true;
}

function addSubModule(): void {
  if (!canEditRule.value) {
    return;
  }
  moduleForm.subModules.push(newSubModule("", moduleForm.subModules.length));
}

function removeSubModule(index: number): void {
  if (!canEditRule.value) {
    return;
  }
  moduleForm.subModules.splice(index, 1);
}

function saveModuleDetail(): void {
  if (!canEditRule.value) {
    return;
  }
  if (!moduleForm.moduleName.trim()) {
    ElMessage.warning("模块名称不能为空");
    return;
  }
  for (const sub of moduleForm.subModules) {
    if (!sub.name.trim()) {
      ElMessage.warning("子模块名称不能为空");
      return;
    }
  }
  const index = ruleContent.scoreModules.findIndex((item) => item.moduleKey === moduleEditingKey.value);
  if (index < 0) {
    moduleDialogVisible.value = false;
    return;
  }
  ruleContent.scoreModules[index] = normalizeModule(moduleForm, index);
  if (ruleContent.scoreModules[index].isExtra) {
    ruleContent.scoreModules[index].weight = 0;
    ruleContent.scoreModules[index].moduleName = "额外加减分模块";
  }
  moduleDialogVisible.value = false;
}

function addGradeRule(): void {
  if (!canEditRule.value) {
    return;
  }
  ruleContent.gradeRules.push({ grade: "", min: 0, max: 0 });
}

function removeGradeRule(index: number): void {
  if (!canEditRule.value) {
    return;
  }
  ruleContent.gradeRules.splice(index, 1);
}

function validateRuleContent(content: StructuredRuleContent): string {
  const regularModules = content.scoreModules.filter((item) => !item.isExtra);
  const extraModules = content.scoreModules.filter((item) => item.isExtra);
  if (regularModules.length === 0) {
    return "至少保留一个常规模块";
  }
  if (extraModules.length !== 1) {
    return "必须且只能保留一个额外加减分模块";
  }
  for (const module of regularModules) {
    if (!module.moduleName.trim()) {
      return "模块名称不能为空";
    }
    if (module.weight <= 0) {
      return `模块「${module.moduleName}」权重必须大于 0`;
    }
    for (const sub of module.subModules) {
      if (!sub.name.trim()) {
        return `模块「${module.moduleName}」包含空名称子模块`;
      }
      if (sub.weight < 0) {
        return `模块「${module.moduleName}」子模块权重不能小于 0`;
      }
    }
  }
  if (content.gradeRules.length === 0) {
    return "请至少配置一个等第划分";
  }
  for (const row of content.gradeRules) {
    if (!row.grade.trim()) {
      return "等第名称不能为空";
    }
    if (row.min > row.max) {
      return `等第「${row.grade}」最低分不能大于最高分`;
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

  const normalizedContent = normalizeRuleContent(cloneContent(ruleContent));
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

async function removeRule(rule: RuleFileItem): Promise<void> {
  try {
    await ElMessageBox.confirm(`确认删除规则「${rule.ruleName}」吗？`, "删除规则", { type: "warning" });
    await deleteRuleFile(rule.id);
    ElMessage.success("规则已删除");
    await Promise.all([loadFilesOnly(), loadBindingsOnly()]);
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除规则失败");
  }
}

async function hideRule(rule: RuleFileItem): Promise<void> {
  try {
    await hideRuleFile(rule.id);
    await loadFilesOnly();
    ElMessage.success("已隐藏该规则");
  } catch (_error) {
    ElMessage.error("隐藏规则失败");
  }
}

async function unhideRule(rule: RuleFileItem): Promise<void> {
  try {
    await unhideRuleFile(rule.id);
    await loadFilesOnly();
    ElMessage.success("已恢复显示该规则");
  } catch (_error) {
    ElMessage.error("恢复规则失败");
  }
}

watch(
  () => [contextStore.sessionId, contextStore.periodCode, contextStore.objectGroupCode],
  () => {
    contextWarning.value = validateContext();
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

.formula-text {
  margin-top: 8px;
  color: #606266;
  font-size: 13px;
}

.editor-actions {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.json-preview {
  margin-top: 10px;
}

.module-dialog-form {
  max-height: 62vh;
  overflow: auto;
}

.inline-form-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.inline-label {
  color: #606266;
  font-size: 12px;
}

.submodule-editor {
  width: 100%;
}

.submodule-toolbar {
  margin-bottom: 8px;
  display: flex;
  justify-content: flex-end;
}

.ml-6 {
  margin-left: 6px;
}

.mb-12 {
  margin-bottom: 12px;
}
</style>
