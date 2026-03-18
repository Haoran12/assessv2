<template>
  <div class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div>
            <strong>总分规则</strong>
            <div class="subtitle">{{ contextText }}</div>
          </div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadRuleForContext">刷新</el-button>
            <el-button type="primary" :loading="saving" :disabled="!canSave" @click="saveModules">
              保存
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

      <template v-else>
        <el-alert
          title="总分 = Σ(模块分数 × 模块权重) + 额外加减分"
          type="info"
          :closable="false"
          class="mb-12"
        />

        <el-empty v-if="!loading && editableModules.length === 0" description="当前尚未配置模块，请新增后保存" />

        <el-table v-loading="loading" :data="editableModules" border>
          <el-table-column type="index" label="#" width="60" />
          <el-table-column label="模块名称" min-width="260">
            <template #default="{ row }">
              <el-input v-model="row.moduleName" maxlength="100" />
            </template>
          </el-table-column>
          <el-table-column label="模块权重(%)" width="190">
            <template #default="{ row }">
              <el-input-number
                v-model="row.weightPercent"
                :min="0"
                :max="100"
                :precision="2"
                :step="0.5"
                controls-position="right"
              />
            </template>
          </el-table-column>
          <el-table-column label="计算方式" width="180">
            <template #default="{ row }">
              <el-select v-model="row.calculationMethod" style="width: 140px">
                <el-option label="直接录入" value="manual" />
                <el-option label="投票计算" value="vote" />
                <el-option label="自定义规则" value="formula" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ $index }">
              <el-button link type="danger" @click="removeModule($index)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="table-actions">
          <el-button :disabled="!canEdit || loading" @click="addModule">新增模块</el-button>
          <div class="weight-sum" :class="{ invalid: !weightSumValid }">
            权重和（不含加减分模块）：{{ weightSumPercent.toFixed(2) }}%
          </div>
        </div>

        <el-alert
          v-if="!weightSumValid"
          title="参与折算模块的权重和必须等于 100%"
          type="warning"
          :closable="false"
          class="mb-12"
        />

        <el-alert
          :title="extraModuleTip"
          type="info"
          :closable="false"
        />
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { createRule, getRule, listRules, updateRule } from "@/api/rules";
import { objectTypeByCategory, assessmentCategoryLabel } from "@/constants/assessmentCategories";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { useUnsavedStore } from "@/stores/unsaved";
import { periodDisplayLabel } from "@/utils/assessment";
import type { AssessmentObjectCategory, AssessmentObjectType } from "@/types/assessment";
import type { CreateRulePayload, RuleDetail, RuleModule, RuleVoteGroup, UpdateRulePayload } from "@/types/rules";

type UiMethod = "manual" | "vote" | "formula";

interface EditableModuleRow {
  moduleName: string;
  weightPercent: number;
  calculationMethod: UiMethod;
  sourceModule?: RuleModule;
}

interface RuleMeta {
  id?: number;
  ruleName: string;
  description: string;
  isActive: boolean;
}

interface ActiveContext {
  yearId: number;
  periodCode: string;
  objectCategory: AssessmentObjectCategory;
  objectType: AssessmentObjectType;
}

const appStore = useAppStore();
const contextStore = useContextStore();
const unsavedStore = useUnsavedStore();

const loading = ref(false);
const saving = ref(false);
const contextWarning = ref("");
const editableModules = ref<EditableModuleRow[]>([]);
const extraModules = ref<RuleModule[]>([]);
const ruleMeta = ref<RuleMeta>({
  id: undefined,
  ruleName: "",
  description: "",
  isActive: true,
});
const baselineSignature = ref("");
const dirtySourceId = "rules-total";

const canEdit = computed(() => appStore.hasPermission("rule:update"));

const activeContext = computed<ActiveContext | null>(() => {
  if (!contextStore.yearId) {
    return null;
  }
  const periodCode = contextStore.periodCode?.trim();
  if (!periodCode) {
    return null;
  }
  if (contextStore.objectCategory === "all") {
    return null;
  }
  const category = contextStore.objectCategory as AssessmentObjectCategory;
  return {
    yearId: contextStore.yearId,
    periodCode,
    objectCategory: category,
    objectType: objectTypeByCategory(category),
  };
});

const contextText = computed(() => {
  const yearText = contextStore.currentYear
    ? `${contextStore.currentYear.year}年`
    : "未选择年度";
  const periodText = contextStore.periodCode
    ? periodDisplayLabel(contextStore.periodCode, contextStore.currentPeriod?.periodName)
    : "未选择周期";
  const categoryText =
    contextStore.objectCategory === "all"
      ? "全部分类"
      : assessmentCategoryLabel(contextStore.objectCategory);
  const objectTypeText =
    contextStore.objectCategory === "all"
      ? "未确定"
      : objectTypeByCategory(contextStore.objectCategory) === "team"
        ? "团体"
        : "个人";
  return `当前上下文：${yearText} / ${periodText} / ${objectTypeText} / ${categoryText}`;
});

const weightSumPercent = computed(() =>
  editableModules.value.reduce((sum, item) => sum + normalizeWeightPercent(item.weightPercent), 0),
);

const weightSumValid = computed(() => Math.abs(roundTo2(weightSumPercent.value) - 100) < 0.001);

const canSave = computed(
  () =>
    canEdit.value &&
    !loading.value &&
    !saving.value &&
    !!activeContext.value &&
    editableModules.value.length > 0 &&
    weightSumValid.value,
);

const extraModuleTip = computed(() => {
  const fixedName = extraModules.value[0]?.moduleName || "权重外加减分";
  return `固定模块：「${fixedName}」不参与权重和校验。`;
});

function buildContextWarning(): string {
  if (!contextStore.yearId) {
    return "请先在顶部选择年度";
  }
  if (!contextStore.periodCode?.trim()) {
    return "请先在顶部选择周期";
  }
  if (contextStore.objectCategory === "all") {
    return "请先在顶部选择具体考核对象分类";
  }
  return "";
}

function roundTo2(value: number): number {
  return Math.round(value * 100) / 100;
}

function roundTo4(value: number): number {
  return Math.round(value * 10000) / 10000;
}

function normalizeWeightPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  if (value < 0) {
    return 0;
  }
  if (value > 100) {
    return 100;
  }
  return roundTo2(value);
}

function toUiMethod(module: RuleModule): UiMethod {
  if (module.moduleCode === "vote") {
    return "vote";
  }
  if (module.moduleCode === "custom") {
    return "formula";
  }
  return "manual";
}

function defaultRuleName(ctx: ActiveContext): string {
  const periodText = periodDisplayLabel(ctx.periodCode, contextStore.currentPeriod?.periodName);
  return `${ctx.yearId}-${periodText}-${assessmentCategoryLabel(ctx.objectCategory)}-总分规则`;
}

function defaultExtraModule(sortOrder: number, usedKeys: Set<string>): RuleModule {
  let moduleKey = "extra_points";
  let suffix = 1;
  while (usedKeys.has(moduleKey)) {
    suffix += 1;
    moduleKey = `extra_points_${suffix}`;
  }
  usedKeys.add(moduleKey);
  return {
    moduleCode: "extra",
    moduleKey,
    moduleName: "权重外加减分",
    sortOrder,
    isActive: true,
  };
}

function normalizeVoteGroups(groups: RuleVoteGroup[] | undefined): RuleVoteGroup[] {
  if (!groups || groups.length === 0) {
    return [
      {
        groupCode: "custom_group",
        groupName: "默认分组",
        weight: 1,
        voterType: "custom",
        maxScore: 100,
        sortOrder: 1,
        isActive: true,
      },
    ];
  }

  const normalized = groups.map((group, index) => ({
    ...group,
    groupCode: String(group.groupCode || `group_${index + 1}`).trim(),
    groupName: String(group.groupName || `分组${index + 1}`).trim(),
    weight: Number.isFinite(group.weight) && group.weight > 0 ? Number(group.weight) : 0,
    voterType: String(group.voterType || "custom").trim() || "custom",
    maxScore: Number.isFinite(group.maxScore) && group.maxScore > 0 ? Number(group.maxScore) : 100,
    sortOrder: index + 1,
    isActive: group.isActive !== false,
  }));

  const sum = normalized.reduce((acc, item) => acc + item.weight, 0);
  if (sum <= 0) {
    normalized[0].weight = 1;
    return normalized;
  }

  if (Math.abs(sum - 1) > 0.00001) {
    const scaled = normalized.map((item) => ({ ...item, weight: roundTo4(item.weight / sum) }));
    const scaledSum = scaled.reduce((acc, item) => acc + item.weight, 0);
    const delta = roundTo4(1 - scaledSum);
    scaled[scaled.length - 1].weight = roundTo4(scaled[scaled.length - 1].weight + delta);
    return scaled;
  }

  return normalized;
}

function fallbackModuleKey(moduleName: string, index: number, usedKeys: Set<string>): string {
  const base = moduleName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  let candidate = base ? `${base}_${index + 1}` : `module_${index + 1}`;
  let suffix = 1;
  while (usedKeys.has(candidate)) {
    suffix += 1;
    candidate = `${base || `module_${index + 1}`}_${suffix}`;
  }
  usedKeys.add(candidate);
  return candidate;
}

function currentSignature(): string {
  return JSON.stringify({
    context: {
      yearId: contextStore.yearId || null,
      periodCode: contextStore.periodCode || "",
      objectCategory: contextStore.objectCategory,
    },
    ruleId: ruleMeta.value.id || null,
    modules: editableModules.value.map((item) => ({
      moduleName: item.moduleName,
      weightPercent: roundTo2(item.weightPercent),
      calculationMethod: item.calculationMethod,
      sourceModuleCode: item.sourceModule?.moduleCode || "",
      sourceModuleKey: item.sourceModule?.moduleKey || "",
    })),
  });
}

function resetBaseline(): void {
  baselineSignature.value = currentSignature();
  unsavedStore.clearDirty(dirtySourceId);
}

function applyRuleDetail(detail: RuleDetail, ctx: ActiveContext): void {
  ruleMeta.value = {
    id: detail.rule.id,
    ruleName: detail.rule.ruleName,
    description: detail.rule.description || "",
    isActive: detail.rule.isActive,
  };

  const modules = [...detail.modules].sort((a, b) => {
    if (a.sortOrder !== b.sortOrder) {
      return a.sortOrder - b.sortOrder;
    }
    return (a.id || 0) - (b.id || 0);
  });

  const normals: EditableModuleRow[] = [];
  const extras: RuleModule[] = [];

  for (const module of modules) {
    if (module.moduleCode === "extra") {
      extras.push(module);
      continue;
    }
    normals.push({
      moduleName: module.moduleName,
      weightPercent: roundTo2((module.weight || 0) * 100),
      calculationMethod: toUiMethod(module),
      sourceModule: module,
    });
  }

  editableModules.value = normals;
  extraModules.value = extras;

  if (!ruleMeta.value.ruleName.trim()) {
    ruleMeta.value.ruleName = defaultRuleName(ctx);
  }
}

function applyEmptyRule(ctx: ActiveContext): void {
  ruleMeta.value = {
    id: undefined,
    ruleName: defaultRuleName(ctx),
    description: "",
    isActive: true,
  };
  editableModules.value = [];
  extraModules.value = [];
}

async function loadRuleForContext(): Promise<void> {
  const warning = buildContextWarning();
  contextWarning.value = warning;

  if (warning || !activeContext.value) {
    editableModules.value = [];
    extraModules.value = [];
    ruleMeta.value = {
      id: undefined,
      ruleName: "",
      description: "",
      isActive: true,
    };
    resetBaseline();
    return;
  }

  const ctx = activeContext.value;
  loading.value = true;
  try {
    const rules = await listRules({
      yearId: ctx.yearId,
      periodCode: ctx.periodCode,
      objectType: ctx.objectType,
      objectCategory: ctx.objectCategory,
    });

    if (rules.length === 0) {
      applyEmptyRule(ctx);
      resetBaseline();
      return;
    }

    const targetRule = rules[0];
    const detail = await getRule(targetRule.id);
    applyRuleDetail(detail, ctx);
    resetBaseline();
  } catch (error) {
    const message = error instanceof Error ? error.message : "总分规则加载失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

function addModule(): void {
  if (!canEdit.value) {
    ElMessage.warning("当前账号没有总分规则编辑权限");
    return;
  }

  const maxEditable = extraModules.value.length > 0 ? 9 : 10;
  if (editableModules.value.length >= maxEditable) {
    ElMessage.warning(`最多只能配置 ${maxEditable} 个可折算模块`);
    return;
  }

  editableModules.value.push({
    moduleName: `新模块${editableModules.value.length + 1}`,
    weightPercent: 0,
    calculationMethod: "manual",
  });
}

async function removeModule(index: number): Promise<void> {
  if (!canEdit.value) {
    ElMessage.warning("当前账号没有总分规则编辑权限");
    return;
  }

  try {
    await ElMessageBox.confirm("确认删除该模块吗？", "删除确认", {
      type: "warning",
    });
  } catch (_error) {
    return;
  }

  editableModules.value.splice(index, 1);
}

function buildModulesPayload(): RuleModule[] {
  const usedKeys = new Set<string>();

  for (const extra of extraModules.value) {
    if (extra.moduleKey) {
      usedKeys.add(extra.moduleKey);
    }
  }

  const normalizedEditable = editableModules.value.map((item, index) => {
    const moduleName = item.moduleName.trim();
    const weightValue = roundTo4(normalizeWeightPercent(item.weightPercent) / 100);

    const base = item.sourceModule;
    const moduleCode =
      item.calculationMethod === "vote"
        ? "vote"
        : item.calculationMethod === "formula"
          ? "custom"
          : "direct";

    const moduleKey =
      base?.moduleKey && base.moduleKey.trim()
        ? base.moduleKey.trim()
        : fallbackModuleKey(moduleName, index, usedKeys);

    usedKeys.add(moduleKey);

    const module: RuleModule = {
      moduleCode,
      moduleKey,
      moduleName,
      weight: weightValue,
      maxScore:
        base?.maxScore && Number.isFinite(base.maxScore) && base.maxScore > 0
          ? Number(base.maxScore)
          : 100,
      calculationMethod: "",
      expression: "",
      contextScope: base?.contextScope,
      sortOrder: index + 1,
      isActive: base?.isActive !== false,
    };

    if (moduleCode === "vote") {
      module.calculationMethod = "grade_mapping";
      module.voteGroups = normalizeVoteGroups(base?.voteGroups);
    }

    if (moduleCode === "custom") {
      module.calculationMethod = "formula";
      const expression = base?.expression?.trim();
      module.expression = expression || "team.score";
      module.voteGroups = [];
    }

    if (moduleCode === "direct") {
      module.calculationMethod = "";
      module.expression = "";
      module.voteGroups = [];
    }

    return module;
  });

  const normalizedExtras: RuleModule[] = extraModules.value.map((item, index) => ({
    ...item,
    moduleCode: "extra" as const,
    moduleKey: item.moduleKey?.trim() || fallbackModuleKey(item.moduleName || "extra", index, usedKeys),
    moduleName: item.moduleName?.trim() || "权重外加减分",
    weight: undefined,
    maxScore: undefined,
    calculationMethod: "",
    expression: "",
    voteGroups: [],
    sortOrder: normalizedEditable.length + index + 1,
    isActive: item.isActive !== false,
  }));

  if (normalizedExtras.length === 0) {
    normalizedExtras.push(defaultExtraModule(normalizedEditable.length + 1, usedKeys));
  }

  return [...normalizedEditable, ...normalizedExtras];
}

function validateBeforeSave(): string | null {
  if (!activeContext.value) {
    return "请先补全顶部上下文后再保存";
  }
  if (editableModules.value.length === 0) {
    return "请至少配置一个总分计算模块";
  }

  for (let index = 0; index < editableModules.value.length; index++) {
    const module = editableModules.value[index];
    if (!module.moduleName.trim()) {
      return `第 ${index + 1} 个模块名称不能为空`;
    }
    if (normalizeWeightPercent(module.weightPercent) <= 0) {
      return `第 ${index + 1} 个模块权重必须大于 0`;
    }
  }

  if (!weightSumValid.value) {
    return "参与折算模块的权重和必须等于 100%";
  }

  return null;
}

async function saveModules(): Promise<boolean> {
  if (!canEdit.value) {
    ElMessage.warning("当前账号没有总分规则编辑权限");
    return false;
  }

  const validationError = validateBeforeSave();
  if (validationError) {
    ElMessage.warning(validationError);
    return false;
  }

  const ctx = activeContext.value;
  if (!ctx) {
    ElMessage.warning("请先补全顶部上下文后再保存");
    return false;
  }

  saving.value = true;
  try {
    const modules = buildModulesPayload();

    let detail: RuleDetail;
    if (ruleMeta.value.id) {
      const payload: UpdateRulePayload = {
        ruleName: ruleMeta.value.ruleName,
        description: ruleMeta.value.description,
        isActive: ruleMeta.value.isActive,
        syncQuarterly: false,
        modules,
      };
      detail = await updateRule(ruleMeta.value.id, payload);
    } else {
      const payload: CreateRulePayload = {
        yearId: ctx.yearId,
        periodCode: ctx.periodCode,
        objectType: ctx.objectType,
        objectCategory: ctx.objectCategory,
        ruleName: ruleMeta.value.ruleName || defaultRuleName(ctx),
        description: ruleMeta.value.description,
        isActive: true,
        syncQuarterly: false,
        modules,
      };
      detail = await createRule(payload);
    }

    applyRuleDetail(detail, ctx);
    resetBaseline();
    ElMessage.success("总分规则已保存");
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : "总分规则保存失败";
    ElMessage.error(message);
    return false;
  } finally {
    saving.value = false;
  }
}

watch(
  () => [contextStore.yearId, contextStore.periodCode, contextStore.objectCategory],
  () => {
    void loadRuleForContext();
  },
  { immediate: true },
);

watch(
  editableModules,
  () => {
    if (loading.value || !baselineSignature.value) {
      unsavedStore.clearDirty(dirtySourceId);
      return;
    }

    if (currentSignature() === baselineSignature.value) {
      unsavedStore.clearDirty(dirtySourceId);
      return;
    }

    unsavedStore.markDirty(dirtySourceId);
  },
  { deep: true },
);

onMounted(() => {
  unsavedStore.setSourceMeta(dirtySourceId, {
    label: "总分规则",
    save: saveModules,
  });
});

onBeforeUnmount(() => {
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

.subtitle {
  margin-top: 4px;
  color: #606266;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.table-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
  margin-bottom: 12px;
}

.weight-sum {
  color: #606266;
  font-size: 13px;
  font-weight: 500;
}

.weight-sum.invalid {
  color: #e6a23c;
}

.mb-12 {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .table-actions {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
