<template>
  <div class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>规则配置（M3）</strong>
          <div class="toolbar">
            <el-button @click="loadRules">刷新</el-button>
            <el-button type="primary" @click="openCreateRule">新建规则</el-button>
          </div>
        </div>
      </template>

      <el-form :inline="true" class="filters">
        <el-form-item label="年度编号">
          <el-input-number v-model="filters.yearId" :min="1" controls-position="right" />
        </el-form-item>
        <el-form-item label="周期">
          <el-select v-model="filters.periodCode" clearable style="width: 140px">
            <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select v-model="filters.objectType" style="width: 140px" @change="onFilterObjectTypeChange">
            <el-option label="团体" value="team" />
            <el-option label="个人" value="individual" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象分类">
          <el-select v-model="filters.objectCategory" style="width: 180px">
            <el-option
              v-for="item in objectCategoryOptions(filters.objectType)"
              :key="item"
              :label="assessmentCategoryLabel(item)"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadRules">查询</el-button>
          <el-button @click="loadTemplates">模板刷新</el-button>
        </el-form-item>
      </el-form>

      <el-table v-loading="rulesLoading" :data="rules" border>
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column prop="yearId" label="年度编号" width="100" />
        <el-table-column prop="periodCode" label="周期" width="110" />
        <el-table-column prop="objectType" label="对象类型" width="120" />
        <el-table-column label="对象分类" min-width="150">
          <template #default="{ row }">
            {{ assessmentCategoryLabel(row.objectCategory) }}
          </template>
        </el-table-column>
        <el-table-column prop="ruleName" label="规则名称" min-width="180" />
        <el-table-column prop="moduleCount" label="模块数" width="90" />
        <el-table-column prop="isActive" label="启用" width="80">
          <template #default="{ row }">
            <el-tag :type="row.isActive ? 'success' : 'info'">{{ row.isActive ? "是" : "否" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEditRule(row.id)">编辑</el-button>
            <el-button link type="success" @click="saveTemplateFromRule(row.id, row.ruleName)">
              存为模板
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <strong>规则模板</strong>
        </div>
      </template>
      <el-table v-loading="templatesLoading" :data="templates" border>
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column prop="templateName" label="模板名称" min-width="180" />
        <el-table-column prop="objectType" label="对象类型" width="120" />
        <el-table-column label="对象分类" min-width="150">
          <template #default="{ row }">
            {{ assessmentCategoryLabel(row.objectCategory) }}
          </template>
        </el-table-column>
        <el-table-column label="模块数" width="90">
          <template #default="{ row }">
            {{ row.config?.modules?.length ?? 0 }}
          </template>
        </el-table-column>
        <el-table-column prop="isSystem" label="系统模板" width="100">
          <template #default="{ row }">
            <el-tag :type="row.isSystem ? 'warning' : 'info'">{{ row.isSystem ? "是" : "否" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openApplyTemplate(row)">应用</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="ruleDialogVisible"
      :title="ruleForm.id ? '编辑规则' : '新建规则'"
      width="1100px"
      top="4vh"
    >
      <div class="dialog-content">
        <el-form label-width="120px">
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="年度编号">
                <el-input-number
                  v-model="ruleForm.yearId"
                  :disabled="Boolean(ruleForm.id)"
                  :min="1"
                  controls-position="right"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="周期">
                <el-select v-model="ruleForm.periodCode" :disabled="Boolean(ruleForm.id)">
                  <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="启用">
                <el-switch v-model="ruleForm.isActive" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="对象类型">
                <el-select
                  v-model="ruleForm.objectType"
                  :disabled="Boolean(ruleForm.id)"
                  @change="onRuleObjectTypeChange"
                >
                  <el-option label="团体" value="team" />
                  <el-option label="个人" value="individual" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="对象分类">
                <el-select v-model="ruleForm.objectCategory" :disabled="Boolean(ruleForm.id)">
                  <el-option
                    v-for="item in objectCategoryOptions(ruleForm.objectType)"
                    :key="item"
                    :label="assessmentCategoryLabel(item)"
                    :value="item"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="季度复用">
                <el-switch v-model="ruleForm.syncQuarterly" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="规则名称">
            <el-input v-model="ruleForm.ruleName" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="ruleForm.description" type="textarea" :rows="2" />
          </el-form-item>
        </el-form>

        <div class="module-header">
          <div>
            <strong>模块配置</strong>
            <el-tag class="weight-tag" :type="weightSumTagType">
              权重和（不含加减分模块）：{{ weightedSum.toFixed(4) }}
            </el-tag>
          </div>
          <div>
            <el-button @click="addModule('direct')">新增直接录入</el-button>
            <el-button @click="addModule('vote')">新增投票</el-button>
            <el-button @click="addModule('custom')">新增自定义</el-button>
            <el-button @click="addModule('extra')">新增加减分</el-button>
          </div>
        </div>

        <div class="modules">
          <el-card
            v-for="(module, index) in ruleForm.modules"
            :key="index"
            class="module-card"
            draggable="true"
            @dragstart="onModuleDragStart(index)"
            @dragover.prevent
            @drop="onModuleDrop(index)"
          >
            <template #header>
              <div class="module-card-header">
                <strong>模块 {{ index + 1 }} - {{ module.moduleCode }}</strong>
                <div class="module-actions">
                  <el-button link :disabled="index === 0" @click="moveModule(index, -1)">上移</el-button>
                  <el-button
                    link
                    :disabled="index === ruleForm.modules.length - 1"
                    @click="moveModule(index, 1)"
                  >
                    下移
                  </el-button>
                  <el-button link type="danger" @click="removeModule(index)">删除</el-button>
                </div>
              </div>
            </template>

            <el-row :gutter="12">
              <el-col :span="6">
                <el-form-item label="类型">
                  <el-select v-model="module.moduleCode">
                    <el-option label="直接录入" value="direct" />
                    <el-option label="投票" value="vote" />
                    <el-option label="自定义" value="custom" />
                    <el-option label="加减分" value="extra" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="模块标识">
                  <el-input v-model="module.moduleKey" />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="模块名称">
                  <el-input v-model="module.moduleName" />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="排序">
                  <el-input-number v-model="module.sortOrder" :min="1" controls-position="right" />
                </el-form-item>
              </el-col>
            </el-row>

            <el-row :gutter="12">
              <el-col :span="6">
                <el-form-item label="权重">
                  <el-input-number
                    v-model="module.weight"
                    :disabled="module.moduleCode === 'extra'"
                    :min="0"
                    :max="1"
                    :step="0.1"
                    :precision="4"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="最高分">
                  <el-input-number
                    v-model="module.maxScore"
                    :disabled="module.moduleCode === 'extra'"
                    :min="0"
                    :max="200"
                    :step="1"
                    :precision="2"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="计算方式">
                  <el-input v-model="module.calculationMethod" />
                </el-form-item>
              </el-col>
              <el-col :span="6">
                <el-form-item label="启用">
                  <el-switch v-model="module.isActive" />
                </el-form-item>
              </el-col>
            </el-row>

            <el-form-item label="表达式（自定义）">
              <el-input
                v-model="module.expression"
                type="textarea"
                :rows="2"
                placeholder="例如：team.score * 0.3 + if(team.rank <= 10, 5, 0)"
              />
            </el-form-item>
            <el-alert
              v-if="module.moduleCode === 'custom'"
              type="info"
              :closable="false"
              title="表达式白名单变量：team.score、team.rank、q1.score~q4.score、extra_points、org.*、module_*；函数支持 abs/round/ceil/floor/max/min/if/avg/sum"
              style="margin-bottom: 8px"
            />
            <el-form-item label="上下文范围（JSON）">
              <el-input
                v-model="module.contextScopeText"
                type="textarea"
                :rows="2"
                placeholder='例如：{"source":"quarterly"}'
              />
            </el-form-item>

            <template v-if="module.moduleCode === 'vote'">
              <div class="vote-group-header">
                <div>
                  <strong>投票分组</strong>
                  <el-tag class="weight-tag" :type="voteGroupWeightTagType(module)">
                    分组权重和：{{ voteGroupWeightSum(module).toFixed(4) }}
                  </el-tag>
                </div>
                <el-button size="small" @click="addVoteGroup(module)">新增分组</el-button>
              </div>
              <el-table :data="module.voteGroups" border size="small">
                <el-table-column prop="groupCode" label="分组编码">
                  <template #default="{ row }"><el-input v-model="row.groupCode" /></template>
                </el-table-column>
                <el-table-column prop="groupName" label="分组名称">
                  <template #default="{ row }"><el-input v-model="row.groupName" /></template>
                </el-table-column>
                <el-table-column prop="weight" label="权重" width="120">
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
                <el-table-column prop="voterType" label="投票人类型">
                  <template #default="{ row }"><el-input v-model="row.voterType" /></template>
                </el-table-column>
                <el-table-column prop="maxScore" label="最高分" width="120">
                  <template #default="{ row }">
                    <el-input-number v-model="row.maxScore" :min="0" :precision="2" controls-position="right" />
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="100" fixed="right">
                  <template #default="{ $index }">
                    <el-button link type="danger" @click="removeVoteGroup(module, $index)">删除</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </template>
          </el-card>
        </div>
      </div>
      <template #footer>
        <el-button @click="ruleDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="ruleSaving" @click="submitRule">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="applyDialogVisible" title="应用模板" width="580px">
      <el-form label-width="120px">
        <el-form-item label="模板">
          <el-input v-model="applyForm.templateName" disabled />
        </el-form-item>
        <el-form-item label="年度编号">
          <el-input-number v-model="applyForm.yearId" :min="1" controls-position="right" />
        </el-form-item>
        <el-form-item label="周期">
          <el-select v-model="applyForm.periodCode">
            <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select v-model="applyForm.objectType" @change="onApplyObjectTypeChange">
            <el-option label="团体" value="team" />
            <el-option label="个人" value="individual" />
          </el-select>
        </el-form-item>
        <el-form-item label="对象分类">
          <el-select v-model="applyForm.objectCategory">
            <el-option
              v-for="item in objectCategoryOptions(applyForm.objectType)"
              :key="item"
              :label="assessmentCategoryLabel(item)"
              :value="item"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="规则名称">
          <el-input v-model="applyForm.ruleName" />
        </el-form-item>
        <el-form-item label="覆盖已有规则">
          <el-switch v-model="applyForm.overwrite" />
        </el-form-item>
        <el-form-item label="季度复用">
          <el-switch v-model="applyForm.syncQuarterly" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="applyDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="applySaving" @click="submitApplyTemplate">应用</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useContextStore } from "@/stores/context";
import { assessmentCategoriesByObjectType, assessmentCategoryLabel } from "@/constants/assessmentCategories";
import {
  applyRuleTemplate,
  createRule,
  createTemplateFromRule,
  getRule,
  listRules,
  listRuleTemplates,
  updateRule,
} from "@/api/rules";
import type {
  ApplyTemplatePayload,
  AssessmentPeriodCode,
  CreateRulePayload,
  RuleDetail,
  RuleModule,
  RuleModuleCode,
  RuleObjectType,
  RuleSummary,
  RuleTemplateSummary,
  UpdateRulePayload,
} from "@/types/rules";

interface EditableVoteGroup {
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

interface RuleFormState {
  id: number | null;
  yearId: number | null;
  periodCode: AssessmentPeriodCode;
  objectType: RuleObjectType;
  objectCategory: string;
  ruleName: string;
  description: string;
  isActive: boolean;
  syncQuarterly: boolean;
  modules: EditableModule[];
}

interface ApplyFormState {
  templateId: number | null;
  templateName: string;
  yearId: number | null;
  periodCode: AssessmentPeriodCode;
  objectType: RuleObjectType;
  objectCategory: string;
  ruleName: string;
  description: string;
  syncQuarterly: boolean;
  isActive: boolean;
  overwrite: boolean;
}

const periodOptions: AssessmentPeriodCode[] = ["Q1", "Q2", "Q3", "Q4", "YEAR_END"];
const contextStore = useContextStore();

const filters = reactive({
  yearId: null as number | null,
  periodCode: "" as AssessmentPeriodCode | "",
  objectType: "team" as RuleObjectType,
  objectCategory: "subsidiary_company",
});

const rulesLoading = ref(false);
const templatesLoading = ref(false);
const ruleSaving = ref(false);
const applySaving = ref(false);

const rules = ref<RuleSummary[]>([]);
const templates = ref<RuleTemplateSummary[]>([]);

const ruleDialogVisible = ref(false);
const ruleForm = reactive<RuleFormState>(createDefaultRuleForm());

const applyDialogVisible = ref(false);
const applyForm = reactive<ApplyFormState>(createDefaultApplyForm());
const draggingModuleIndex = ref<number | null>(null);

const allowedExpressionFunctions = new Set([
  "abs",
  "round",
  "ceil",
  "floor",
  "max",
  "min",
  "if",
  "avg",
  "sum",
]);

const allowedExpressionVars = new Set([
  "team.score",
  "team.rank",
  "q1.score",
  "q2.score",
  "q3.score",
  "q4.score",
  "extra_points",
]);

const weightedSum = computed(() =>
  ruleForm.modules
    .filter((item) => item.moduleCode !== "extra")
    .reduce((sum, item) => sum + (item.weight ?? 0), 0),
);

const weightSumTagType = computed(() => (Math.abs(weightedSum.value - 1) < 0.0001 ? "success" : "warning"));

function createDefaultRuleForm(): RuleFormState {
  return {
    id: null,
    yearId: filters.yearId,
    periodCode: (filters.periodCode || "Q1") as AssessmentPeriodCode,
    objectType: filters.objectType,
    objectCategory: filters.objectCategory,
    ruleName: "",
    description: "",
    isActive: true,
    syncQuarterly: false,
    modules: [createModule("direct", 1)],
  };
}

function createDefaultApplyForm(): ApplyFormState {
  return {
    templateId: null,
    templateName: "",
    yearId: filters.yearId,
    periodCode: (filters.periodCode || "Q1") as AssessmentPeriodCode,
    objectType: filters.objectType,
    objectCategory: filters.objectCategory,
    ruleName: "",
    description: "",
    syncQuarterly: false,
    isActive: true,
    overwrite: false,
  };
}

function createModule(moduleCode: RuleModuleCode, sortOrder: number): EditableModule {
  const base: EditableModule = {
    moduleCode,
    moduleKey: `${moduleCode}_${sortOrder}`,
    moduleName: `${moduleCode}_${sortOrder}`,
    weight: moduleCode === "extra" ? null : 0,
    maxScore: moduleCode === "extra" ? null : 100,
    calculationMethod: moduleCode === "vote" ? "grade_mapping" : moduleCode === "custom" ? "formula" : "",
    expression: "",
    contextScopeText: "",
    sortOrder,
    isActive: true,
    voteGroups: [],
  };
  if (moduleCode === "vote") {
    base.voteGroups.push(createVoteGroup(1));
  }
  return base;
}

function createVoteGroup(sortOrder: number): EditableVoteGroup {
  return {
    groupCode: `group_${sortOrder}`,
    groupName: `group_${sortOrder}`,
    weight: 0,
    voterType: "peer",
    voterScopeText: "",
    maxScore: 100,
    sortOrder,
    isActive: true,
  };
}

function objectCategoryOptions(objectType: RuleObjectType): string[] {
  return assessmentCategoriesByObjectType(objectType);
}

function onFilterObjectTypeChange(): void {
  const options = objectCategoryOptions(filters.objectType);
  if (!options.includes(filters.objectCategory)) {
    filters.objectCategory = options[0];
  }
}

function onRuleObjectTypeChange(): void {
  const options = objectCategoryOptions(ruleForm.objectType);
  if (!options.includes(ruleForm.objectCategory)) {
    ruleForm.objectCategory = options[0];
  }
}

function onApplyObjectTypeChange(): void {
  const options = objectCategoryOptions(applyForm.objectType);
  if (!options.includes(applyForm.objectCategory)) {
    applyForm.objectCategory = options[0];
  }
}

function syncModuleSortOrder(): void {
  for (let index = 0; index < ruleForm.modules.length; index += 1) {
    ruleForm.modules[index].sortOrder = index + 1;
  }
}

function syncVoteGroupSortOrder(module: EditableModule): void {
  for (let index = 0; index < module.voteGroups.length; index += 1) {
    module.voteGroups[index].sortOrder = index + 1;
  }
}

function moveModule(index: number, offset: number): void {
  const next = index + offset;
  if (next < 0 || next >= ruleForm.modules.length) {
    return;
  }
  const current = ruleForm.modules[index];
  ruleForm.modules.splice(index, 1);
  ruleForm.modules.splice(next, 0, current);
  syncModuleSortOrder();
}

function onModuleDragStart(index: number): void {
  draggingModuleIndex.value = index;
}

function onModuleDrop(index: number): void {
  if (draggingModuleIndex.value === null || draggingModuleIndex.value === index) {
    draggingModuleIndex.value = null;
    return;
  }
  const dragIndex = draggingModuleIndex.value;
  const current = ruleForm.modules[dragIndex];
  ruleForm.modules.splice(dragIndex, 1);
  ruleForm.modules.splice(index, 0, current);
  syncModuleSortOrder();
  draggingModuleIndex.value = null;
}

function voteGroupWeightSum(module: EditableModule): number {
  return module.voteGroups.reduce((sum, item) => sum + (item.weight || 0), 0);
}

function voteGroupWeightTagType(module: EditableModule): "success" | "warning" {
  return Math.abs(voteGroupWeightSum(module) - 1) < 0.0001 ? "success" : "warning";
}

async function loadRules(): Promise<void> {
  rulesLoading.value = true;
  try {
    rules.value = await listRules({
      yearId: filters.yearId ?? undefined,
      periodCode: filters.periodCode || undefined,
      objectType: filters.objectType || undefined,
      objectCategory: filters.objectCategory || undefined,
    });
  } catch (error) {
    void error;
    ElMessage.error("规则列表加载失败");
  } finally {
    rulesLoading.value = false;
  }
}

async function loadTemplates(): Promise<void> {
  templatesLoading.value = true;
  try {
    templates.value = await listRuleTemplates({
      objectType: filters.objectType || undefined,
      objectCategory: filters.objectCategory || undefined,
    });
  } catch (error) {
    void error;
    ElMessage.error("模板列表加载失败");
  } finally {
    templatesLoading.value = false;
  }
}

function openCreateRule(): void {
  Object.assign(ruleForm, createDefaultRuleForm());
  syncModuleSortOrder();
  ruleDialogVisible.value = true;
}

async function openEditRule(ruleId: number): Promise<void> {
  try {
    const detail = await getRule(ruleId);
    fillRuleForm(detail);
    ruleDialogVisible.value = true;
  } catch (error) {
    void error;
    ElMessage.error("规则详情加载失败");
  }
}

function fillRuleForm(detail: RuleDetail): void {
  const modules = detail.modules.map((item, index) => {
    const moduleCode = item.moduleCode as RuleModuleCode;
    const editable: EditableModule = {
      moduleCode,
      moduleKey: item.moduleKey,
      moduleName: item.moduleName,
      weight: item.weight ?? null,
      maxScore: item.maxScore ?? null,
      calculationMethod: item.calculationMethod ?? "",
      expression: item.expression ?? "",
      contextScopeText: stringifyJSONText(item.contextScope),
      sortOrder: item.sortOrder || index + 1,
      isActive: item.isActive,
      voteGroups: [],
    };
    if (Array.isArray(item.voteGroups) && item.voteGroups.length > 0) {
      editable.voteGroups = item.voteGroups.map((group, groupIndex) => ({
        groupCode: group.groupCode,
        groupName: group.groupName,
        weight: group.weight,
        voterType: group.voterType,
        voterScopeText: stringifyJSONText(group.voterScope),
        maxScore: group.maxScore,
        sortOrder: group.sortOrder || groupIndex + 1,
        isActive: group.isActive,
      }));
    }
    return editable;
  });

  Object.assign(ruleForm, {
    id: detail.rule.id,
    yearId: detail.rule.yearId,
    periodCode: detail.rule.periodCode,
    objectType: detail.rule.objectType,
    objectCategory: detail.rule.objectCategory,
    ruleName: detail.rule.ruleName,
    description: detail.rule.description,
    isActive: detail.rule.isActive,
    syncQuarterly: false,
    modules,
  });
  syncModuleSortOrder();
}

function addModule(moduleCode: RuleModuleCode): void {
  ruleForm.modules.push(createModule(moduleCode, ruleForm.modules.length + 1));
  syncModuleSortOrder();
}

function removeModule(index: number): void {
  if (ruleForm.modules.length <= 1) {
    ElMessage.warning("至少保留一个模块");
    return;
  }
  ruleForm.modules.splice(index, 1);
  syncModuleSortOrder();
}

function addVoteGroup(module: EditableModule): void {
  module.voteGroups.push(createVoteGroup(module.voteGroups.length + 1));
  syncVoteGroupSortOrder(module);
}

function removeVoteGroup(module: EditableModule, index: number): void {
  module.voteGroups.splice(index, 1);
  syncVoteGroupSortOrder(module);
}

function validateExpressionFront(expression: string): boolean {
  const text = expression.trim();
  if (!text) {
    return false;
  }
  if (/[^A-Za-z0-9_.,()+\-*/%<>=!&|\s]/.test(text)) {
    return false;
  }
  if (/[;'"`]/.test(text)) {
    return false;
  }

  let balance = 0;
  for (const char of text) {
    if (char === "(") {
      balance += 1;
    }
    if (char === ")") {
      balance -= 1;
      if (balance < 0) {
        return false;
      }
    }
  }
  if (balance !== 0) {
    return false;
  }

  const tokenRegex = /[A-Za-z_][A-Za-z0-9_.]*/g;
  for (const match of text.matchAll(tokenRegex)) {
    const token = match[0];
    const start = match.index ?? 0;
    const nextChar = text.slice(start + token.length).trimStart()[0];
    if (nextChar === "(") {
      if (!allowedExpressionFunctions.has(token.toLowerCase())) {
        return false;
      }
      continue;
    }
    if (
      allowedExpressionVars.has(token) ||
      (token.startsWith("org.") && token.length > "org.".length) ||
      (token.startsWith("module_") && token.length > "module_".length)
    ) {
      continue;
    }
    return false;
  }
  return true;
}

function validateModulesBeforeSubmit(): boolean {
  if (Math.abs(weightedSum.value - 1) > 0.0001) {
    ElMessage.warning("参与折算的模块权重和必须等于 1.0000");
    return false;
  }

  for (let moduleIndex = 0; moduleIndex < ruleForm.modules.length; moduleIndex += 1) {
    const module = ruleForm.modules[moduleIndex];
    if (!module.moduleKey.trim() || !module.moduleName.trim()) {
      ElMessage.warning(`模块 ${moduleIndex + 1} 的“模块标识”和“模块名称”不能为空`);
      return false;
    }
    if (module.moduleCode === "custom") {
      if (!validateExpressionFront(module.expression)) {
        ElMessage.warning(`模块 ${moduleIndex + 1} 的表达式不合法或不在白名单范围`);
        return false;
      }
    }
    if (module.moduleCode === "vote") {
      if (module.voteGroups.length === 0) {
        ElMessage.warning(`模块 ${moduleIndex + 1} 至少需要一个投票分组`);
        return false;
      }
      if (Math.abs(voteGroupWeightSum(module) - 1) > 0.0001) {
        ElMessage.warning(`模块 ${moduleIndex + 1} 的投票分组权重和必须等于 1.0000`);
        return false;
      }
    }
  }

  return true;
}

async function submitRule(): Promise<void> {
  try {
    if (!ruleForm.yearId || ruleForm.yearId <= 0) {
      ElMessage.warning("请填写年度编号");
      return;
    }
    if (!ruleForm.ruleName.trim()) {
      ElMessage.warning("请填写规则名称");
      return;
    }
    if (!validateModulesBeforeSubmit()) {
      return;
    }
    const payloadModules = buildPayloadModules(ruleForm.modules);
    ruleSaving.value = true;

    if (ruleForm.id) {
      const payload: UpdateRulePayload = {
        ruleName: ruleForm.ruleName.trim(),
        description: ruleForm.description.trim(),
        isActive: ruleForm.isActive,
        syncQuarterly: ruleForm.syncQuarterly,
        modules: payloadModules,
      };
      await updateRule(ruleForm.id, payload);
      ElMessage.success("规则已更新");
    } else {
      const payload: CreateRulePayload = {
        yearId: ruleForm.yearId,
        periodCode: ruleForm.periodCode,
        objectType: ruleForm.objectType,
        objectCategory: ruleForm.objectCategory,
        ruleName: ruleForm.ruleName.trim(),
        description: ruleForm.description.trim(),
        isActive: ruleForm.isActive,
        syncQuarterly: ruleForm.syncQuarterly,
        modules: payloadModules,
      };
      await createRule(payload);
      ElMessage.success("规则已创建");
    }

    ruleDialogVisible.value = false;
    await Promise.all([loadRules(), loadTemplates()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "规则保存失败";
    ElMessage.error(message);
  } finally {
    ruleSaving.value = false;
  }
}

async function saveTemplateFromRule(ruleId: number, ruleName: string): Promise<void> {
  try {
    const { value } = await ElMessageBox.prompt("输入模板名称", "存为模板", {
      confirmButtonText: "保存",
      cancelButtonText: "取消",
      inputValue: `${ruleName} 模板`,
    });
    await createTemplateFromRule(ruleId, {
      templateName: value.trim(),
      description: `来源规则 #${ruleId}`,
    });
    ElMessage.success("模板创建成功");
    await loadTemplates();
  } catch (error) {
    void error;
  }
}

function openApplyTemplate(template: RuleTemplateSummary): void {
  Object.assign(applyForm, {
    ...createDefaultApplyForm(),
    templateId: template.id,
    templateName: template.templateName,
    objectType: template.objectType,
    objectCategory: template.objectCategory,
    ruleName: template.config?.ruleName || template.templateName,
    description: template.config?.description || template.description || "",
  });
  applyDialogVisible.value = true;
}

async function submitApplyTemplate(): Promise<void> {
  if (!applyForm.templateId) {
    ElMessage.warning("模板编号无效");
    return;
  }
  if (!applyForm.yearId || applyForm.yearId <= 0) {
    ElMessage.warning("请填写年度编号");
    return;
  }

  try {
    applySaving.value = true;
    const payload: ApplyTemplatePayload = {
      yearId: applyForm.yearId,
      periodCode: applyForm.periodCode,
      objectType: applyForm.objectType,
      objectCategory: applyForm.objectCategory,
      ruleName: applyForm.ruleName.trim(),
      description: applyForm.description.trim(),
      syncQuarterly: applyForm.syncQuarterly,
      isActive: applyForm.isActive,
      overwrite: applyForm.overwrite,
    };
    await applyRuleTemplate(applyForm.templateId, payload);
    ElMessage.success("模板应用成功");
    applyDialogVisible.value = false;
    await loadRules();
  } catch (error) {
    const message = error instanceof Error ? error.message : "模板应用失败";
    ElMessage.error(message);
  } finally {
    applySaving.value = false;
  }
}

function buildPayloadModules(modules: EditableModule[]): RuleModule[] {
  return modules.map((module) => ({
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
        ? module.voteGroups.map((group) => ({
            groupCode: group.groupCode.trim(),
            groupName: group.groupName.trim(),
            weight: group.weight,
            voterType: group.voterType.trim(),
            voterScope: parseJSONText(group.voterScopeText),
            maxScore: group.maxScore,
            sortOrder: group.sortOrder,
            isActive: group.isActive,
          }))
        : [],
  }));
}

function parseJSONText(value: string): unknown | undefined {
  const text = value.trim();
  if (!text) {
    return undefined;
  }
  return JSON.parse(text) as unknown;
}

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

async function syncFiltersFromActiveContext(): Promise<void> {
  await contextStore.ensureInitialized();
  if (contextStore.yearId) {
    filters.yearId = contextStore.yearId;
  }
  const activePeriod = contextStore.periods.find((item) => item.status === "active");
  if (activePeriod) {
    filters.periodCode = activePeriod.periodCode;
    return;
  }
  if (contextStore.currentPeriod) {
    filters.periodCode = contextStore.currentPeriod.periodCode;
    return;
  }
  filters.periodCode = contextStore.periodCode;
}

onMounted(async () => {
  await syncFiltersFromActiveContext();
  await Promise.all([loadRules(), loadTemplates()]);
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

.toolbar {
  display: flex;
  gap: 8px;
}

.filters {
  margin-bottom: 12px;
}

.dialog-content {
  max-height: 72vh;
  overflow: auto;
  padding-right: 8px;
}

.module-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 12px 0;
  gap: 12px;
}

.weight-tag {
  margin-left: 8px;
}

.modules {
  display: grid;
  gap: 12px;
}

.module-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.module-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.vote-group-header {
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

@media (max-width: 900px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .module-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
