<template>
  <div class="module-rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div>
            <strong>模块规则</strong>
            <div class="subtitle">{{ contextText }}</div>
          </div>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadData">刷新</el-button>
            <el-button
              type="primary"
              :loading="saving"
              :disabled="!editingModule"
              @click="saveModule"
            >
              保存模块
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

      <el-form label-width="90px" class="selectors">
        <el-form-item label="规则">
          <el-select
            v-model="selectedRuleId"
            filterable
            placeholder="请选择规则"
            style="width: 360px"
            :disabled="loading || ruleBundles.length === 0"
            @change="handleRuleChange"
          >
            <el-option
              v-for="item in ruleBundles"
              :key="item.summary.id"
              :label="ruleLabel(item.summary)"
              :value="item.summary.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="模块">
          <el-select
            v-model="selectedModuleToken"
            placeholder="请选择模块"
            style="width: 360px"
            :disabled="!selectedBundle"
            @change="handleModuleChange"
          >
            <el-option
              v-for="item in moduleOptions"
              :key="item.token"
              :label="item.label"
              :value="item.token"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <el-empty
        v-if="!editingModule"
        description="当前激活周期没有可编辑模块"
      />

      <template v-else>
        <el-divider />
        <el-form label-width="140px" class="module-form">
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="moduleCode">
                <el-input :model-value="editingModule.moduleCode" disabled />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="moduleKey">
                <el-input v-model="editingModule.moduleKey" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="moduleName">
                <el-input v-model="editingModule.moduleName" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="weight">
                <el-input-number
                  v-model="editingModule.weight"
                  :disabled="editingModule.moduleCode === 'extra'"
                  :step="0.1"
                  :min="0"
                  :max="1"
                  :precision="4"
                  controls-position="right"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="maxScore">
                <el-input-number
                  v-model="editingModule.maxScore"
                  :disabled="editingModule.moduleCode === 'extra'"
                  :step="1"
                  :min="0"
                  :max="200"
                  :precision="2"
                  controls-position="right"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="sortOrder">
                <el-input-number
                  v-model="editingModule.sortOrder"
                  :min="1"
                  controls-position="right"
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="calculationMethod">
                <el-input v-model="editingModule.calculationMethod" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="启用">
                <el-switch v-model="editingModule.isActive" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="ContextScope(JSON)">
            <el-input
              v-model="editingModule.contextScopeText"
              type="textarea"
              :rows="3"
              placeholder='例如：{"source":"quarterly"}'
            />
          </el-form-item>

          <template v-if="editingModule.moduleCode === 'custom'">
            <el-form-item label="自定义规则">
              <div class="custom-rule-wrap">
                <el-input
                  :model-value="editingModule.expression"
                  type="textarea"
                  :rows="3"
                  disabled
                />
                <el-button type="primary" @click="openCustomEditor">打开 Overlay 编辑器</el-button>
              </div>
            </el-form-item>
          </template>

          <template v-if="editingModule.moduleCode === 'vote'">
            <el-form-item label="投票分组">
              <div class="vote-group-wrap">
                <el-button size="small" @click="addVoteGroup">新增分组</el-button>
                <el-table :data="editingModule.voteGroups" border size="small" class="vote-group-table">
                  <el-table-column prop="groupCode" label="groupCode">
                    <template #default="{ row }"><el-input v-model="row.groupCode" /></template>
                  </el-table-column>
                  <el-table-column prop="groupName" label="groupName">
                    <template #default="{ row }"><el-input v-model="row.groupName" /></template>
                  </el-table-column>
                  <el-table-column prop="weight" label="weight" width="120">
                    <template #default="{ row }">
                      <el-input-number
                        v-model="row.weight"
                        :min="0"
                        :max="1"
                        :step="0.1"
                        :precision="4"
                        controls-position="right"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column prop="voterType" label="voterType">
                    <template #default="{ row }"><el-input v-model="row.voterType" /></template>
                  </el-table-column>
                  <el-table-column prop="maxScore" label="maxScore" width="120">
                    <template #default="{ row }">
                      <el-input-number v-model="row.maxScore" :min="0" :precision="2" controls-position="right" />
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="90" fixed="right">
                    <template #default="{ $index }">
                      <el-button link type="danger" @click="removeVoteGroup($index)">删除</el-button>
                    </template>
                  </el-table-column>
                </el-table>
              </div>
            </el-form-item>
          </template>
        </el-form>
      </template>
    </el-card>

    <el-dialog
      v-model="customEditorVisible"
      title="自定义规则编辑器"
      width="780px"
      top="6vh"
    >
      <el-alert
        title="此 Overlay 用于替代原导航中的“计算引擎”入口，直接在模块规则内编辑 custom 表达式。"
        type="info"
        :closable="false"
        class="mb-12"
      />
      <el-input
        v-model="customExpressionDraft"
        type="textarea"
        :rows="14"
        placeholder="例如：team.score * 0.3 + if(team.rank <= 10, 5, 0)"
      />
      <template #footer>
        <el-button @click="customEditorVisible = false">取消</el-button>
        <el-button type="primary" @click="applyCustomExpression">应用表达式</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { useContextStore } from "@/stores/context";
import { getRule, listRules, updateRule } from "@/api/rules";
import type {
  RuleDetail,
  RuleModule,
  RuleModuleCode,
  RuleSummary,
  RuleVoteGroup,
  UpdateRulePayload,
} from "@/types/rules";

interface RuleBundle {
  summary: RuleSummary;
  detail: RuleDetail;
}

interface EditableVoteGroup {
  id?: number;
  moduleId?: number;
  groupCode: string;
  groupName: string;
  weight: number;
  voterType: string;
  voterScopeText: string;
  maxScore: number;
  sortOrder: number;
  isActive: boolean;
}

interface EditableModule {
  sourceIndex: number;
  id?: number;
  ruleId?: number;
  moduleCode: RuleModuleCode;
  moduleKey: string;
  moduleName: string;
  weight: number | null;
  maxScore: number | null;
  calculationMethod: string;
  expression: string;
  contextScopeText: string;
  sortOrder: number;
  isActive: boolean;
  voteGroups: EditableVoteGroup[];
}

const contextStore = useContextStore();

const loading = ref(false);
const saving = ref(false);
const contextWarning = ref("");
const contextText = ref("加载中...");

const ruleBundles = ref<RuleBundle[]>([]);
const selectedRuleId = ref<number | undefined>();
const selectedModuleToken = ref("");
const editingModule = ref<EditableModule | null>(null);

const customEditorVisible = ref(false);
const customExpressionDraft = ref("");

const selectedBundle = computed(() =>
  ruleBundles.value.find((item) => item.summary.id === selectedRuleId.value),
);

const moduleOptions = computed(() => {
  if (!selectedBundle.value) {
    return [] as Array<{ token: string; label: string }>;
  }
  return selectedBundle.value.detail.modules.map((item, index) => ({
    token: toModuleToken(item, index),
    label: `${item.moduleName} [${item.moduleCode}]`,
  }));
});

function stringifyJSONText(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    const text = value.trim();
    if (!text) {
      return "";
    }
    try {
      const parsed = JSON.parse(text) as unknown;
      return JSON.stringify(parsed, null, 2);
    } catch (_error) {
      return text;
    }
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch (_error) {
    return "";
  }
}

function parseJSONText(value: string): unknown | undefined {
  const text = value.trim();
  if (!text) {
    return undefined;
  }
  return JSON.parse(text) as unknown;
}

function toModuleToken(module: RuleModule, index: number): string {
  if (module.id) {
    return `id:${module.id}`;
  }
  return `idx:${index}`;
}

function resolveModuleIndex(detail: RuleDetail, token: string): number {
  if (token.startsWith("id:")) {
    const id = Number(token.slice(3));
    return detail.modules.findIndex((item) => item.id === id);
  }
  if (token.startsWith("idx:")) {
    return Number(token.slice(4));
  }
  return -1;
}

function toEditableModule(module: RuleModule, sourceIndex: number): EditableModule {
  return {
    sourceIndex,
    id: module.id,
    ruleId: module.ruleId,
    moduleCode: module.moduleCode,
    moduleKey: module.moduleKey,
    moduleName: module.moduleName,
    weight: module.weight ?? null,
    maxScore: module.maxScore ?? null,
    calculationMethod: module.calculationMethod ?? "",
    expression: module.expression ?? "",
    contextScopeText: stringifyJSONText(module.contextScope),
    sortOrder: module.sortOrder,
    isActive: module.isActive,
    voteGroups: (module.voteGroups ?? []).map((group, index) => ({
      id: group.id,
      moduleId: group.moduleId,
      groupCode: group.groupCode,
      groupName: group.groupName,
      weight: group.weight,
      voterType: group.voterType,
      voterScopeText: stringifyJSONText(group.voterScope),
      maxScore: group.maxScore,
      sortOrder: group.sortOrder || index + 1,
      isActive: group.isActive,
    })),
  };
}

function toPayloadModule(module: EditableModule): RuleModule {
  return {
    id: module.id,
    ruleId: module.ruleId,
    moduleCode: module.moduleCode,
    moduleKey: module.moduleKey.trim(),
    moduleName: module.moduleName.trim(),
    weight: module.moduleCode === "extra" ? null : module.weight,
    maxScore: module.moduleCode === "extra" ? null : module.maxScore,
    calculationMethod: module.calculationMethod.trim(),
    expression: module.expression.trim(),
    contextScope: parseJSONText(module.contextScopeText),
    sortOrder: module.sortOrder,
    isActive: module.isActive,
    voteGroups:
      module.moduleCode === "vote"
        ? module.voteGroups.map(
            (group): RuleVoteGroup => ({
              id: group.id,
              moduleId: group.moduleId,
              groupCode: group.groupCode.trim(),
              groupName: group.groupName.trim(),
              weight: group.weight,
              voterType: group.voterType.trim(),
              voterScope: parseJSONText(group.voterScopeText),
              maxScore: group.maxScore,
              sortOrder: group.sortOrder,
              isActive: group.isActive,
            }),
          )
        : [],
  };
}

function ruleLabel(rule: RuleSummary): string {
  return `${rule.ruleName} (${rule.objectType}/${rule.objectCategory})`;
}

function syncVoteGroupSortOrder(): void {
  if (!editingModule.value) {
    return;
  }
  for (let index = 0; index < editingModule.value.voteGroups.length; index += 1) {
    editingModule.value.voteGroups[index].sortOrder = index + 1;
  }
}

function addVoteGroup(): void {
  if (!editingModule.value) {
    return;
  }
  const nextIndex = editingModule.value.voteGroups.length + 1;
  editingModule.value.voteGroups.push({
    groupCode: `group_${nextIndex}`,
    groupName: `group_${nextIndex}`,
    weight: 0,
    voterType: "peer",
    voterScopeText: "",
    maxScore: 100,
    sortOrder: nextIndex,
    isActive: true,
  });
  syncVoteGroupSortOrder();
}

function removeVoteGroup(index: number): void {
  if (!editingModule.value) {
    return;
  }
  editingModule.value.voteGroups.splice(index, 1);
  syncVoteGroupSortOrder();
}

function openCustomEditor(): void {
  if (!editingModule.value) {
    return;
  }
  customExpressionDraft.value = editingModule.value.expression || "";
  customEditorVisible.value = true;
}

function applyCustomExpression(): void {
  if (!editingModule.value) {
    return;
  }
  editingModule.value.expression = customExpressionDraft.value;
  editingModule.value.calculationMethod = "formula";
  customEditorVisible.value = false;
}

function handleRuleChange(): void {
  if (!selectedBundle.value || selectedBundle.value.detail.modules.length === 0) {
    selectedModuleToken.value = "";
    editingModule.value = null;
    return;
  }
  selectedModuleToken.value = toModuleToken(selectedBundle.value.detail.modules[0], 0);
  handleModuleChange();
}

function handleModuleChange(): void {
  if (!selectedBundle.value || !selectedModuleToken.value) {
    editingModule.value = null;
    return;
  }
  const index = resolveModuleIndex(selectedBundle.value.detail, selectedModuleToken.value);
  if (index < 0 || index >= selectedBundle.value.detail.modules.length) {
    editingModule.value = null;
    return;
  }
  editingModule.value = toEditableModule(selectedBundle.value.detail.modules[index], index);
}

async function loadData(): Promise<void> {
  loading.value = true;
  contextWarning.value = "";
  try {
    await contextStore.ensureInitialized();
    await contextStore.refreshPeriods();

    if (!contextStore.yearId) {
      ruleBundles.value = [];
      selectedRuleId.value = undefined;
      selectedModuleToken.value = "";
      editingModule.value = null;
      contextText.value = "未选择考核年度";
      return;
    }

    const activePeriod = contextStore.periods.find((item) => item.status === "active");
    const targetPeriod = activePeriod || contextStore.currentPeriod || contextStore.periods[0];
    if (!targetPeriod) {
      contextText.value = "当前年度没有可用周期";
      return;
    }
    if (!activePeriod) {
      contextWarning.value = "当前年度不存在“进行中(active)”周期，已回退到当前选中周期。";
    }

    const yearId = contextStore.yearId;
    contextText.value = `当前定位：yearId=${yearId} / period=${targetPeriod.periodCode} / status=${targetPeriod.status}`;

    const rows = await listRules({
      yearId,
      periodCode: targetPeriod.periodCode,
    });
    const activeRows = rows.filter((item) => item.isActive);
    const targetRows = activeRows.length > 0 ? activeRows : rows;
    if (activeRows.length === 0 && rows.length > 0) {
      contextWarning.value = `${contextWarning.value} 未找到激活规则，已展示该周期全部规则。`.trim();
    }

    const details = await Promise.all(targetRows.map((item) => getRule(item.id)));
    ruleBundles.value = targetRows.map((summary, index) => ({
      summary,
      detail: details[index],
    }));

    if (ruleBundles.value.length === 0) {
      selectedRuleId.value = undefined;
      selectedModuleToken.value = "";
      editingModule.value = null;
      return;
    }

    const availableRuleIds = new Set(ruleBundles.value.map((item) => item.summary.id));
    if (!selectedRuleId.value || !availableRuleIds.has(selectedRuleId.value)) {
      selectedRuleId.value = ruleBundles.value[0].summary.id;
    }
    handleRuleChange();
  } catch (error) {
    const message = error instanceof Error ? error.message : "模块规则加载失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

async function refreshCurrentRuleDetail(ruleId: number): Promise<void> {
  const index = ruleBundles.value.findIndex((item) => item.summary.id === ruleId);
  if (index < 0) {
    return;
  }
  const detail = await getRule(ruleId);
  const next = [...ruleBundles.value];
  next[index] = {
    ...next[index],
    detail,
  };
  ruleBundles.value = next;
}

async function saveModule(): Promise<void> {
  if (!selectedBundle.value || !editingModule.value) {
    return;
  }
  if (!editingModule.value.moduleKey.trim() || !editingModule.value.moduleName.trim()) {
    ElMessage.warning("moduleKey 和 moduleName 不能为空");
    return;
  }

  saving.value = true;
  try {
    const sourceModules = selectedBundle.value.detail.modules;
    const targetIndex = editingModule.value.sourceIndex;
    if (targetIndex < 0 || targetIndex >= sourceModules.length) {
      ElMessage.error("模块索引无效，请刷新后重试");
      return;
    }

    const payloadModule = toPayloadModule(editingModule.value);
    const modules = sourceModules.map((item, index) => (index === targetIndex ? payloadModule : item));
    const payload: UpdateRulePayload = {
      ruleName: selectedBundle.value.summary.ruleName,
      description: selectedBundle.value.summary.description,
      isActive: selectedBundle.value.summary.isActive,
      syncQuarterly: false,
      modules,
    };

    await updateRule(selectedBundle.value.summary.id, payload);
    await refreshCurrentRuleDetail(selectedBundle.value.summary.id);
    handleModuleChange();
    ElMessage.success("模块规则已保存");
  } catch (error) {
    const message = error instanceof Error ? error.message : "模块保存失败";
    ElMessage.error(message);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await loadData();
});
</script>

<style scoped>
.module-rules-view {
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

.selectors {
  margin-top: 6px;
}

.module-form {
  max-width: 1100px;
}

.custom-rule-wrap {
  display: grid;
  gap: 10px;
  width: 100%;
}

.vote-group-wrap {
  width: 100%;
}

.vote-group-table {
  margin-top: 8px;
}

.mb-12 {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
