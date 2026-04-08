<template>
  <div ref="rulesViewRef" class="rules-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="card-title">{{ displayTitle }}</div>
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
        <el-tabs v-model="activeEditTab" class="editor-tabs" :class="{ 'is-locked-tab': lockEditTab }">
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
                  <el-table-column label="拖动排序" width="76" align="center">
                    <template #default="{ row, $index }">
                      <div
                        class="drag-handle"
                        :class="{
                          'is-disabled': !canEditRule || isExtraAdjustModule(row),
                          'is-dragging': draggingModuleIndex === $index,
                          'is-drop-target': moduleDropTargetIndex === $index && draggingModuleIndex !== $index,
                        }"
                        :draggable="canEditRule && !isExtraAdjustModule(row)"
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
                  <el-table-column label="模块名称" min-width="200">
                    <template #default="{ row }">
                      <el-input v-model="row.moduleName" :disabled="!canEditRule || isExtraAdjustModule(row)" />
                    </template>
                  </el-table-column>
                  <el-table-column label="权重" min-width="118">
                    <template #default="{ row }">
                      <el-input-number
                        v-model="row.weight"
                        class="module-weight-input"
                        :disabled="!canEditRule || isExtraAdjustModule(row)"
                        :min="0"
                        :step="1"
                      />
                    </template>
                  </el-table-column>
                  <el-table-column label="计分方式" min-width="150">
                    <template #default="{ row }">
                      <el-select
                        v-model="row.calculationMethod"
                        class="module-method-select"
                        :disabled="!canEditRule || isExtraAdjustModule(row)"
                        @change="handleMethodChange(row)"
                      >
                        <el-option label="直接录入" value="direct_input" />
                        <el-option label="投票模式" value="vote" />
                        <el-option label="自定义脚本" value="custom_script" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="160" fixed="right">
                    <template #default="{ row }">
                      <div class="table-row-actions">
                        <el-button size="small" type="primary" plain :disabled="isExtraAdjustModule(row)" @click="openModuleDetail(row)">详情</el-button>
                        <el-button size="small" type="danger" plain :disabled="!canEditRule || isExtraAdjustModule(row)" @click="removeScoreModule(row)">删除</el-button>
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
                <div v-if="!hideGradeInnerTitle" class="section-head">
                  <strong>等第划分规则</strong>
                </div>
                <el-table :data="activeScopedRule.grades" class="rules-table">
                  <el-table-column label="等第标题" min-width="120">
                    <template #default="{ row }">
                      <el-input v-model="row.title" :disabled="!canEditRule" />
                    </template>
                  </el-table-column>
                  <el-table-column label="上限" min-width="240">
                    <template #default="{ row }">
                      <div class="grade-node-cell">
                        <el-switch v-model="row.scoreNode.hasUpperLimit" :disabled="!canEditRule" />
                        <el-select
                          v-model="row.scoreNode.upperOperator"
                          class="grade-operator-select"
                          :disabled="!canEditRule || !row.scoreNode.hasUpperLimit"
                        >
                          <el-option label="<" value="<" />
                          <el-option label="<=" value="<=" />
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
                  <el-table-column label="下限" min-width="240">
                    <template #default="{ row }">
                      <div class="grade-node-cell">
                        <el-switch v-model="row.scoreNode.hasLowerLimit" :disabled="!canEditRule" />
                        <el-select
                          v-model="row.scoreNode.lowerOperator"
                          class="grade-operator-select"
                          :disabled="!canEditRule || !row.scoreNode.hasLowerLimit"
                        >
                          <el-option label=">" value=">" />
                          <el-option label=">=" value=">=" />
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
                  <el-table-column label="区间/条件" min-width="120">
                    <template #default="{ row }">
                      <el-select v-model="row.conditionLogic" class="grade-logic-select" :disabled="!canEditRule">
                        <el-option label="AND" value="and" />
                        <el-option label="OR" value="or" />
                      </el-select>
                    </template>
                  </el-table-column>
                  <el-table-column label="人数上限" min-width="166">
                    <template #default="{ row }">
                      <div class="grade-ratio-inline">
                        <el-input-number
                          v-model="row.maxRatioPercent"
                          class="grade-ratio-input"
                          :disabled="!canEditRule"
                          :min="0"
                          :max="100"
                          :step="0.1"
                          size="small"
                          placeholder="不限"
                        />
                        <span class="grade-ratio-unit">%</span>
                        <el-select
                          v-model="row.maxRatioRoundingMode"
                          class="grade-ratio-mode-select"
                          :disabled="!canEditRule"
                          size="small"
                        >
                          <el-option
                            v-for="item in maxRatioRoundingModeOptions"
                            :key="item.value"
                            :label="item.label"
                            :value="item.value"
                          />
                        </el-select>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" width="160" fixed="right">
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
                  <el-button type="warning" plain :disabled="!activeScopedRule" @click="openCopyDialog">
                    从其他范围复制规则
                  </el-button>
                </div>
              </div>
              <el-alert
                type="info"
                :closable="false"
                class="section-block"
                title="等第分配规则：按顺序匹配并受人数上限约束。"
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
          <div class="script-helper-panel">
            <div class="script-helper-header">
              <span>变量快速插入</span>
              <span v-if="expressionContextLoading" class="script-helper-loading">加载中...</span>
            </div>
            <div class="script-picker-grid">
              <div class="script-picker-field">
                <div class="script-picker-label">1. 考核周期</div>
                <el-select
                  v-model="moduleInsertPicker.periodCode"
                  class="script-picker-select"
                  placeholder="请选择考核周期"
                >
                  <el-option
                    v-for="period in expressionPeriods"
                    :key="`module-insert-period-${period}`"
                    :label="period"
                    :value="period"
                  />
                </el-select>
              </div>
              <div class="script-picker-field">
                <div class="script-picker-label">2. 考核对象</div>
                <el-select
                  v-model="moduleInsertPicker.objectRef"
                  class="script-picker-select"
                  placeholder="请选择考核对象"
                >
                  <el-option
                    v-for="item in expressionInsertObjectOptions"
                    :key="`module-insert-object-${item.value}`"
                    :label="item.label"
                    :value="item.value"
                  />
                </el-select>
              </div>
              <div class="script-picker-field">
                <div class="script-picker-label">3. 可调用数据</div>
                <el-select
                  v-model="moduleInsertPicker.dataKey"
                  class="script-picker-select"
                  placeholder="请选择可调用数据"
                >
                  <el-option
                    v-for="item in moduleExpressionDataOptions"
                    :key="`module-insert-data-${item.value}`"
                    :label="item.label"
                    :value="item.value"
                  />
                </el-select>
              </div>
            </div>
            <div class="script-picker-preview">
              <span class="script-picker-preview-label">将插入代码</span>
              <code class="script-picker-preview-code">{{ moduleInsertCodePreview || "请先完成三项选择" }}</code>
            </div>
            <div class="script-picker-actions">
              <el-button
                type="primary"
                plain
                :disabled="!canEditRule || !moduleInsertCodePreview"
                @click="insertModuleSelectedExpression"
              >
                插入变量
              </el-button>
            </div>
          </div>
        </template>

        <template v-else-if="moduleDetailTarget.calculationMethod === 'vote'">
          <div class="field-label">投票挡位与分值</div>
            <el-table :data="moduleVoteGradeRows" border class="rules-table vote-grade-table">
            <el-table-column label="挡位名称" min-width="160">
              <template #default="{ row }">
                <el-input v-model="row.label" :disabled="!canEditRule" placeholder="例如：优秀" />
              </template>
            </el-table-column>
            <el-table-column label="分值" min-width="160">
              <template #default="{ row }">
                <el-input-number
                  v-model="row.score"
                  :disabled="!canEditRule"
                  :min="0"
                  :max="100"
                  :step="1"
                  :precision="0"
                  style="width: 100%"
                />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="96" align="center">
              <template #default="{ $index }">
                <el-button
                  link
                  type="danger"
                  :disabled="!canEditRule || moduleVoteGradeRows.length <= 1"
                  @click="removeVoteGradeRow($index)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="canEditRule" class="table-footer-actions">
            <el-button type="primary" plain @click="addVoteGradeRow">新增挡位</el-button>
          </div>

          <div class="field-label">投票主体与权重</div>
          <el-table :data="moduleVoteSubjectRows" border class="rules-table vote-subject-table">
            <el-table-column label="主体名称" min-width="160">
              <template #default="{ row }">
                <el-input v-model="row.label" :disabled="!canEditRule" placeholder="例如：干部评议组" />
              </template>
            </el-table-column>
            <el-table-column label="权重" min-width="160">
              <template #default="{ row }">
                <el-input-number
                  v-model="row.weight"
                  :disabled="!canEditRule"
                  :min="0"
                  :step="0.01"
                  :precision="4"
                  style="width: 100%"
                />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="96" align="center">
              <template #default="{ $index }">
                <el-button
                  link
                  type="danger"
                  :disabled="!canEditRule || moduleVoteSubjectRows.length <= 1"
                  @click="removeVoteSubjectRow($index)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="canEditRule" class="table-footer-actions">
            <el-button type="primary" plain @click="addVoteSubjectRow">新增主体</el-button>
          </div>
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
        <div class="grade-extra-switch">
          <span>启用额外脚本条件</span>
          <el-switch v-model="gradeDetailDraft.extraConditionEnabled" :disabled="!canEditRule" />
        </div>
        <div class="field-label">额外条件脚本</div>
        <el-input
          v-model="gradeDetailDraft.extraConditionScript"
          type="textarea"
          :rows="12"
          :disabled="!canEditRule || !gradeDetailDraft.extraConditionEnabled"
          placeholder="默认不启用；开启后脚本才会参与等第判断"
        />
        <div class="script-helper-panel">
          <div class="script-helper-header">
            <span>变量快速插入</span>
            <span v-if="expressionContextLoading" class="script-helper-loading">加载中...</span>
          </div>
          <div class="script-picker-grid">
            <div class="script-picker-field">
              <div class="script-picker-label">1. 考核周期</div>
              <el-select
                v-model="gradeInsertPicker.periodCode"
                class="script-picker-select"
                placeholder="请选择考核周期"
              >
                <el-option
                  v-for="period in expressionPeriods"
                  :key="`grade-insert-period-${period}`"
                  :label="period"
                  :value="period"
                />
              </el-select>
            </div>
              <div class="script-picker-field">
                <div class="script-picker-label">2. 考核对象</div>
              <el-select
                v-model="gradeInsertPicker.objectRef"
                class="script-picker-select"
                placeholder="请选择考核对象"
              >
                <el-option
                  v-for="item in expressionInsertObjectOptions"
                  :key="`grade-insert-object-${item.value}`"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </div>
            <div class="script-picker-field">
              <div class="script-picker-label">3. 可调用数据</div>
              <el-select
                v-model="gradeInsertPicker.dataKey"
                class="script-picker-select"
                placeholder="请选择可调用数据"
              >
                <el-option
                  v-for="item in gradeExpressionDataOptions"
                  :key="`grade-insert-data-${item.value}`"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </div>
          </div>
          <div class="script-picker-preview">
            <span class="script-picker-preview-label">将插入代码</span>
            <code class="script-picker-preview-code">{{ gradeInsertCodePreview || "请先完成三项选择" }}</code>
          </div>
          <div class="script-picker-actions">
            <el-button
              type="primary"
              plain
              :disabled="!canEditRule || !gradeInsertCodePreview"
              @click="insertGradeSelectedExpression"
            >
              插入变量
            </el-button>
          </div>
        </div>
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
  getRuleExpressionContext,
  listRuleFiles,
  updateRuleFile,
} from "@/api/rules";
import type {
  AssessmentObjectGroupItem,
  AssessmentSessionItem,
  AssessmentSessionPeriodItem,
} from "@/types/assessment";
import type {
  RuleDependencyCheckResult,
  RuleExpressionContext,
  RuleExpressionObjectOption,
  RuleExpressionVariable,
  RuleFileItem,
} from "@/types/rules";

type ScoreMethod = "direct_input" | "vote" | "custom_script";
type ConditionLogic = "and" | "or";
type UpperOperator = "<" | "<=";
type LowerOperator = ">" | ">=";
type MaxRatioRoundingMode = "real" | "floor" | "ceil";

const EXTRA_ADJUST_MODULE_KEY = "__extra_adjust__";
const EXTRA_ADJUST_MODULE_NAME = "额外加减分";

interface ScoreModule {
  id: string;
  moduleKey: string;
  moduleName: string;
  weight: number;
  calculationMethod: ScoreMethod;
  customScript: string;
  voteConfigJson: string;
}

interface VoteGradeRow {
  id: string;
  label: string;
  score: number | null;
}

interface VoteSubjectRow {
  id: string;
  label: string;
  weight: number | null;
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
  extraConditionEnabled: boolean;
  conditionLogic: ConditionLogic;
  maxRatioPercent: number | null;
  maxRatioRoundingMode: MaxRatioRoundingMode;
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

interface ExpressionInsertDataOption {
  value: string;
  label: string;
}

interface ExpressionInsertObjectOption {
  value: string;
  label: string;
  rawObject?: RuleExpressionObjectOption;
}

interface ExpressionInsertPicker {
  periodCode: string;
  objectRef: string;
  dataKey: string;
}

interface RulesViewProps {
  initialEditTab?: "modules" | "grades";
  lockEditTab?: boolean;
  headerTitle?: string;
  hideGradeInnerTitle?: boolean;
}

const props = withDefaults(defineProps<RulesViewProps>(), {
  initialEditTab: "modules",
  lockEditTab: false,
  headerTitle: "",
  hideGradeInnerTitle: false,
});

const lockEditTab = computed(() => props.lockEditTab);
const hideGradeInnerTitle = computed(() => props.hideGradeInnerTitle);
const displayTitle = computed(() => {
  const custom = String(props.headerTitle || "").trim();
  return custom || "规则管理";
});
const maxRatioRoundingModeOptions: Array<{ value: MaxRatioRoundingMode; label: string }> = [
  { value: "real", label: "实数" },
  { value: "floor", label: "去尾" },
  { value: "ceil", label: "进一" },
];

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
const activeEditTab = ref<"modules" | "grades">(props.initialEditTab);

const moduleDetailVisible = ref(false);
const moduleDetailTargetId = ref("");
const moduleDetailDraft = reactive({
  customScript: "",
  voteConfigJson: "",
});
const moduleVoteGradeRows = ref<VoteGradeRow[]>([]);
const moduleVoteSubjectRows = ref<VoteSubjectRow[]>([]);
const moduleVoteConfigExtras = ref<Record<string, unknown>>({});
const gradeDetailVisible = ref(false);
const gradeDetailTargetId = ref("");
const gradeDetailDraft = reactive({
  extraConditionScript: "",
  extraConditionEnabled: false,
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
const expressionContextLoading = ref(false);
const expressionContext = ref<RuleExpressionContext | null>(null);
const moduleInsertPicker = reactive<ExpressionInsertPicker>({
  periodCode: "",
  objectRef: "",
  dataKey: "",
});
const gradeInsertPicker = reactive<ExpressionInsertPicker>({
  periodCode: "",
  objectRef: "",
  dataKey: "",
});

const ruleContent = reactive<StructuredRuleContent>(defaultRuleContent(true));

const contextWarning = ref("");
const canEditRule = computed(() => !!currentRule.value?.canEdit);

const activeScopedRule = computed(() =>
  ruleContent.scopedRules.find((item) => item.id === activeScopedRuleId.value) || null,
);

const totalWeight = computed(() =>
  (activeScopedRule.value?.scoreModules || [])
    .filter((item) => !isExtraAdjustModule(item))
    .reduce((sum, item) => sum + asNumber(item.weight, 0), 0),
);

const structuredJsonPreview = computed(() => JSON.stringify(normalizeRuleContent(cloneDeep(ruleContent)), null, 2));
const expressionPeriods = computed<string[]>(() => expressionContext.value?.periods || []);
const expressionObjects = computed<RuleExpressionObjectOption[]>(() => expressionContext.value?.objects || []);
const EXPRESSION_OBJECT_SELF = "__self__";
const EXPRESSION_OBJECT_PARENT = "__parent__";
const EXPRESSION_OBJECT_STATIC_PREFIX = "object:";
const expressionInsertObjectOptions = computed<ExpressionInsertObjectOption[]>(() => {
  const options: ExpressionInsertObjectOption[] = [
    { value: EXPRESSION_OBJECT_SELF, label: "对象自己（当前 objectId）" },
    { value: EXPRESSION_OBJECT_PARENT, label: "对象所属部门/组织（当前 parentObjectId）" },
  ];
  for (const item of expressionObjects.value) {
    options.push({
      value: `${EXPRESSION_OBJECT_STATIC_PREFIX}${item.objectId}`,
      label: formatExpressionObjectOption(item),
      rawObject: item,
    });
  }
  return options;
});
const expressionModuleKeys = computed<string[]>(() => {
  const keys: string[] = [];
  const seen = new Set<string>();
  const allVariables: RuleExpressionVariable[] = [
    ...(expressionContext.value?.moduleVariables || []),
    ...(expressionContext.value?.gradeVariables || []),
  ];
  for (const item of allVariables) {
    const source = String(item.insertText || item.name || "").trim();
    const match = /^moduleScores\["(.+)"\]$/.exec(source);
    if (!match || !match[1]) {
      continue;
    }
    const moduleKey = String(match[1]).trim();
    if (!moduleKey || seen.has(moduleKey)) {
      continue;
    }
    seen.add(moduleKey);
    keys.push(moduleKey);
  }
  return keys;
});
const expressionModuleNameByKey = computed<Record<string, string>>(() => {
  const mapping: Record<string, string> = {};
  const applyName = (moduleKey: string, moduleName: string, overwrite: boolean): void => {
    const key = String(moduleKey || "").trim();
    const name = String(moduleName || "").trim();
    if (!key || !name) {
      return;
    }
    if (overwrite || !mapping[key]) {
      mapping[key] = name;
    }
  };
  for (const module of activeScopedRule.value?.scoreModules || []) {
    applyName(module.moduleKey, module.moduleName, true);
  }
  for (const scoped of ruleContent.scopedRules) {
    for (const module of scoped.scoreModules) {
      applyName(module.moduleKey, module.moduleName, false);
    }
  }
  return mapping;
});
const expressionDataOptions = computed<ExpressionInsertDataOption[]>(() => {
  const options: ExpressionInsertDataOption[] = [
    { value: "score", label: "总分" },
    { value: "rank", label: "排名" },
    { value: "grade", label: "等第" },
    { value: "has_score", label: "是否已评分" },
    { value: "target_score", label: "业务目标总分" },
  ];
  for (const key of expressionModuleKeys.value) {
    const moduleName = String(expressionModuleNameByKey.value[key] || "").trim() || key;
    options.push({
      value: `module_score:${key}`,
      label: `模块分：${moduleName}`,
    });
  }
  return options;
});
const moduleExpressionDataOptions = computed<ExpressionInsertDataOption[]>(() =>
  filterExpressionDataOptionsByObjectRef(moduleInsertPicker.objectRef),
);
const gradeExpressionDataOptions = computed<ExpressionInsertDataOption[]>(() =>
  filterExpressionDataOptionsByObjectRef(gradeInsertPicker.objectRef),
);
const moduleInsertCodePreview = computed(() =>
  buildExpressionInsertCode(
    moduleInsertPicker.periodCode,
    moduleInsertPicker.objectRef,
    moduleInsertPicker.dataKey,
  ),
);
const gradeInsertCodePreview = computed(() =>
  buildExpressionInsertCode(
    gradeInsertPicker.periodCode,
    gradeInsertPicker.objectRef,
    gradeInsertPicker.dataKey,
  ),
);

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

function formatExpressionObjectOption(item: RuleExpressionObjectOption): string {
  const objectName = String(item.objectName || "").trim() || `对象${item.objectId}`;
  const organizationName = String(contextStore.currentSession?.organizationName || "").trim();
  const targetType = String(item.targetType || "")
    .trim()
    .toLowerCase();
  if (targetType === "organization" || targetType === "org" || targetType === "group" || targetType === "company") {
    return objectName;
  }
  if (targetType === "department" || targetType === "employee" || targetType === "person" || targetType === "individual") {
    return organizationName ? `${organizationName}-${objectName}` : objectName;
  }
  if (organizationName) {
    return `${organizationName}-${objectName}`;
  }
  return objectName;
}

function resolveExpressionObject(objectRef: string): RuleExpressionObjectOption | null {
  const normalizedRef = String(objectRef || "").trim();
  if (!normalizedRef.startsWith(EXPRESSION_OBJECT_STATIC_PREFIX)) {
    return null;
  }
  const objectID = Number(normalizedRef.slice(EXPRESSION_OBJECT_STATIC_PREFIX.length));
  if (!Number.isFinite(objectID) || objectID <= 0) {
    return null;
  }
  for (const item of expressionObjects.value) {
    if (item.objectId === objectID) {
      return item;
    }
  }
  return null;
}

function buildExpressionInsertCode(
  periodCode: string,
  objectRef: string,
  dataKey: string,
): string {
  const normalizedPeriod = String(periodCode || "").trim();
  const normalizedObjectRef = String(objectRef || "").trim();
  const object = resolveExpressionObject(normalizedObjectRef);
  const normalizedDataKey = String(dataKey || "").trim();
  if (!normalizedPeriod || !normalizedObjectRef || !normalizedDataKey) {
    return "";
  }
  const isSelf = normalizedObjectRef === EXPRESSION_OBJECT_SELF;
  const isParent = normalizedObjectRef === EXPRESSION_OBJECT_PARENT;
  if (!isSelf && !isParent && !object) {
    return "";
  }

  const runtimeObjectLiteral = isSelf ? "objectId" : isParent ? "parentObjectId" : "";
  const periodLiteral = JSON.stringify(normalizedPeriod);
  const objectLiteral = runtimeObjectLiteral || String(object?.objectId || "");
  if (!objectLiteral) {
    return "";
  }
  if (normalizedDataKey === "score") {
    return `score(${periodLiteral}, ${objectLiteral})`;
  }
  if (normalizedDataKey === "rank") {
    return `rank(${periodLiteral}, ${objectLiteral})`;
  }
  if (normalizedDataKey === "grade") {
    return `grade(${periodLiteral}, ${objectLiteral})`;
  }
  if (normalizedDataKey === "has_score") {
    return `hasScore(${periodLiteral}, ${objectLiteral})`;
  }
  if (normalizedDataKey === "target_score") {
    if (isSelf) {
      return `targetScore(${periodLiteral}, targetType, targetId)`;
    }
    if (isParent) {
      return "";
    }
    if (!object) {
      return "";
    }
    const targetType = JSON.stringify(String(object.targetType || "").trim());
    return `targetScore(${periodLiteral}, ${targetType}, ${object.targetId})`;
  }
  if (normalizedDataKey.startsWith("module_score:")) {
    const moduleKey = String(normalizedDataKey.slice("module_score:".length)).trim();
    if (!moduleKey) {
      return "";
    }
    return `moduleScore(${periodLiteral}, ${objectLiteral}, ${JSON.stringify(moduleKey)})`;
  }
  return "";
}

function defaultExpressionPeriodCode(): string {
  const preferred = String(contextStore.periodCode || "").trim();
  if (preferred && expressionPeriods.value.includes(preferred)) {
    return preferred;
  }
  return expressionPeriods.value[0] || "";
}

function defaultExpressionObjectRef(): string {
  return expressionInsertObjectOptions.value[0]?.value || "";
}

function filterExpressionDataOptionsByObjectRef(objectRef: string): ExpressionInsertDataOption[] {
  if (String(objectRef || "").trim() === EXPRESSION_OBJECT_PARENT) {
    return expressionDataOptions.value.filter((item) => item.value !== "target_score");
  }
  return expressionDataOptions.value;
}

function ensureExpressionPickerState(picker: ExpressionInsertPicker): void {
  if (!expressionPeriods.value.includes(picker.periodCode)) {
    picker.periodCode = defaultExpressionPeriodCode();
  }
  const objectExists = !!picker.objectRef && expressionInsertObjectOptions.value.some((item) => item.value === picker.objectRef);
  if (!objectExists) {
    picker.objectRef = defaultExpressionObjectRef();
  }
  const availableDataOptions = filterExpressionDataOptionsByObjectRef(picker.objectRef);
  const dataExists = availableDataOptions.some((item) => item.value === picker.dataKey);
  if (!dataExists) {
    picker.dataKey = availableDataOptions[0]?.value || "";
  }
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

function normalizeMaxRatioRoundingMode(value: unknown): MaxRatioRoundingMode {
  const text = String(value || "")
    .trim()
    .toLowerCase();
  if (text === "floor") {
    return "floor";
  }
  if (text === "ceil") {
    return "ceil";
  }
  return "real";
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

function normalizePeriodCode(value: unknown): string {
  return String(value || "").trim().toUpperCase();
}

function resolveSharedPeriodCodes(periodCode: string): string[] {
  const normalizedPeriod = normalizePeriodCode(periodCode);
  if (!normalizedPeriod) {
    return [];
  }

  const currentPeriod = contextStore.periods.find(
    (item) => normalizePeriodCode(item.periodCode) === normalizedPeriod,
  );
  if (!currentPeriod) {
    return [normalizedPeriod];
  }

  const bindingKey = String(currentPeriod.ruleBindingKey || "").trim();
  if (!bindingKey) {
    return [normalizedPeriod];
  }

  const result = contextStore.periods
    .filter((item) => String(item.ruleBindingKey || "").trim() === bindingKey)
    .map((item) => normalizePeriodCode(item.periodCode))
    .filter((item) => !!item);
  if (result.length === 0) {
    return [normalizedPeriod];
  }
  if (!result.includes(normalizedPeriod)) {
    result.push(normalizedPeriod);
  }
  return Array.from(new Set(result));
}

function scopedIncludesAnyPeriod(scoped: ScopedRule, periodCodes: string[]): boolean {
  if (periodCodes.length === 0) {
    return false;
  }
  const lookup = new Set(periodCodes.map((item) => normalizePeriodCode(item)));
  return scoped.applicablePeriods.some((item) => lookup.has(normalizePeriodCode(item)));
}

function applySharedPeriodBindingToActiveScopedRule(): string[] {
  const scoped = activeScopedRule.value;
  const periodCode = normalizePeriodCode(contextStore.periodCode);
  const groupCode = String(contextStore.objectGroupCode || "").trim();
  if (!scoped || !periodCode || !groupCode) {
    return [];
  }

  const sharedPeriods = resolveSharedPeriodCodes(periodCode);
  const targetPeriods = sharedPeriods.length > 0 ? sharedPeriods : [periodCode];
  const sharedPeriodLookup = new Set(targetPeriods.map((item) => normalizePeriodCode(item)));

  scoped.applicablePeriods = normalizedCodeList([...scoped.applicablePeriods, ...targetPeriods], true);
  scoped.applicableObjectGroups = normalizedCodeList([...scoped.applicableObjectGroups, groupCode], false);

  // Keep one scoped rule owner per period-group pair in a shared binding.
  for (const row of ruleContent.scopedRules) {
    if (row.id === scoped.id) {
      continue;
    }
    if (!row.applicableObjectGroups.includes(groupCode)) {
      continue;
    }
    row.applicablePeriods = normalizedCodeList(
      row.applicablePeriods.filter((item) => !sharedPeriodLookup.has(normalizePeriodCode(item))),
      true,
    );
  }
  return targetPeriods;
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

function defaultVoteGradeRows(): VoteGradeRow[] {
  const seeds = [
    { label: "优秀", score: 100 },
    { label: "良好", score: 85 },
    { label: "一般", score: 70 },
    { label: "较差", score: 60 },
  ];
  return seeds.map((item) => ({
    id: uuid("vote_grade"),
    label: item.label,
    score: item.score,
  }));
}

function defaultVoteSubjectRows(): VoteSubjectRow[] {
  return [
    {
      id: uuid("vote_subject"),
      label: "主体1",
      weight: 1,
    },
  ];
}

function normalizeVoteGradeRow(raw: unknown, index: number): VoteGradeRow | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const row = raw as Record<string, unknown>;
  const label = String(row.label ?? row.name ?? row.title ?? row.grade ?? row.option ?? "").trim();
  const score = toNullableNumber(row.score ?? row.value ?? row.points);
  if (!label && score === null) {
    return null;
  }
  return {
    id: uuid("vote_grade"),
    label: label || `挡位${index + 1}`,
    score,
  };
}

function normalizeVoteSubjectRow(raw: unknown, index: number): VoteSubjectRow | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const row = raw as Record<string, unknown>;
  const label = String(row.label ?? row.name ?? row.title ?? row.subject ?? row.group ?? "").trim();
  const weight = toNullableNumber(row.weight ?? row.ratio ?? row.value ?? row.points);
  if (!label && weight === null) {
    return null;
  }
  return {
    id: uuid("vote_subject"),
    label: label || `主体${index + 1}`,
    weight,
  };
}

function parseVoteGradeRowsFromConfig(configText: string): {
  rows: VoteGradeRow[];
  extras: Record<string, unknown>;
} {
  const parsed = parseJsonOrText(configText);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    return {
      rows: defaultVoteGradeRows(),
      extras: {},
    };
  }

  const extras: Record<string, unknown> = { ...(parsed as Record<string, unknown>) };
  let candidate: unknown;
  const candidateKeys = ["gradeScores", "grades", "levels", "options", "items"];
  for (const key of candidateKeys) {
    if (Object.prototype.hasOwnProperty.call(extras, key)) {
      candidate = extras[key];
      delete extras[key];
      break;
    }
  }

  const rows: VoteGradeRow[] = [];
  if (Array.isArray(candidate)) {
    candidate.forEach((item, index) => {
      const normalized = normalizeVoteGradeRow(item, index);
      if (normalized) {
        rows.push(normalized);
      }
    });
  } else if (candidate && typeof candidate === "object") {
    Object.entries(candidate as Record<string, unknown>).forEach(([key, value], index) => {
      const score = toNullableNumber(value);
      if (score === null) {
        return;
      }
      rows.push({
        id: uuid("vote_grade"),
        label: String(key || "").trim() || `挡位${index + 1}`,
        score,
      });
    });
  }

  if (rows.length === 0) {
    const numericEntries = Object.entries(extras).filter(([, value]) => toNullableNumber(value) !== null);
    if (numericEntries.length >= 2) {
      numericEntries.forEach(([key, value], index) => {
        const score = toNullableNumber(value);
        if (score === null) {
          return;
        }
        rows.push({
          id: uuid("vote_grade"),
          label: String(key || "").trim() || `挡位${index + 1}`,
          score,
        });
        delete extras[key];
      });
    }
  }

  return {
    rows: rows.length > 0 ? rows : defaultVoteGradeRows(),
    extras,
  };
}

function parseVoteSubjectRowsFromConfig(extras: Record<string, unknown>): VoteSubjectRow[] {
  let candidate: unknown;
  const candidateKeys = ["voterSubjects", "subjectWeights", "subjects", "voteSubjects", "voterGroups", "groups"];
  for (const key of candidateKeys) {
    if (Object.prototype.hasOwnProperty.call(extras, key)) {
      candidate = extras[key];
      delete extras[key];
      break;
    }
  }

  const rows: VoteSubjectRow[] = [];
  if (Array.isArray(candidate)) {
    candidate.forEach((item, index) => {
      const normalized = normalizeVoteSubjectRow(item, index);
      if (normalized) {
        rows.push(normalized);
      }
    });
  } else if (candidate && typeof candidate === "object") {
    Object.entries(candidate as Record<string, unknown>).forEach(([key, value], index) => {
      const weight = toNullableNumber(value);
      if (weight === null) {
        return;
      }
      rows.push({
        id: uuid("vote_subject"),
        label: String(key || "").trim() || `主体${index + 1}`,
        weight,
      });
    });
  }

  if (rows.length === 0) {
    return defaultVoteSubjectRows();
  }
  return rows;
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

function isExtraAdjustModule(module: Pick<ScoreModule, "moduleKey" | "id"> | null | undefined): boolean {
  if (!module) {
    return false;
  }
  const moduleKey = String(module.moduleKey || "").trim();
  const moduleID = String(module.id || "").trim();
  return moduleKey === EXTRA_ADJUST_MODULE_KEY || moduleID === EXTRA_ADJUST_MODULE_KEY;
}

function normalizeExtraAdjustModule(module?: Partial<ScoreModule>): ScoreModule {
  return {
    id: String(module?.id || EXTRA_ADJUST_MODULE_KEY).trim() || EXTRA_ADJUST_MODULE_KEY,
    moduleKey: EXTRA_ADJUST_MODULE_KEY,
    moduleName: EXTRA_ADJUST_MODULE_NAME,
    weight: 0,
    calculationMethod: "direct_input",
    customScript: "",
    voteConfigJson: "",
  };
}

function ensureScoreModulesWithExtraAdjust(modules: ScoreModule[]): ScoreModule[] {
  const normalized: ScoreModule[] = [];
  let extraModule: ScoreModule | null = null;
  for (const module of modules) {
    if (isExtraAdjustModule(module)) {
      if (!extraModule) {
        extraModule = normalizeExtraAdjustModule(module);
      }
      continue;
    }
    normalized.push(module);
  }
  normalized.push(extraModule || normalizeExtraAdjustModule());
  return normalized;
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
    extraConditionEnabled: false,
    conditionLogic: "and",
    maxRatioPercent: null,
    maxRatioRoundingMode: "real",
  };
}

function defaultScopedRule(withContext: boolean): ScopedRule {
  return {
    id: uuid("scoped"),
    applicablePeriods: withContext && contextStore.periodCode ? [contextStore.periodCode] : [],
    applicableObjectGroups: withContext && contextStore.objectGroupCode ? [contextStore.objectGroupCode] : [],
    scoreModules: ensureScoreModulesWithExtraAdjust([newScoreModule("基础绩效", 100)]),
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
    extraConditionEnabled:
      typeof raw?.extraConditionEnabled === "boolean"
        ? raw.extraConditionEnabled
        : String(raw?.extraConditionScript || "").trim().length > 0,
    conditionLogic: normalizeLogic(raw?.conditionLogic || "and"),
    maxRatioPercent: maxRatio,
    maxRatioRoundingMode: normalizeMaxRatioRoundingMode(raw?.maxRatioRoundingMode),
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
    scoreModules: ensureScoreModulesWithExtraAdjust(modules.length > 0 ? modules : [newScoreModule(`模块${index + 1}`, 100)]),
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
  const periodCode = normalizePeriodCode(contextStore.periodCode);
  const groupCode = contextStore.objectGroupCode;
  if (!periodCode || !groupCode) {
    activeScopedRuleId.value = "";
    return;
  }
  const sharedPeriodCodes = resolveSharedPeriodCodes(periodCode);
  const targetPeriods = sharedPeriodCodes.length > 0 ? sharedPeriodCodes : [periodCode];

  const target = ruleContent.scopedRules.find(
    (item) =>
      item.applicableObjectGroups.includes(groupCode) &&
      scopedIncludesAnyPeriod(item, targetPeriods),
  );
  if (target) {
    activeScopedRuleId.value = target.id;
    return;
  }

  const row = defaultScopedRule(false);
  row.applicablePeriods = targetPeriods;
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
    await loadExpressionContext();
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载规则管理数据失败";
    ElMessage.error(message);
  } finally {
    loading.value = false;
  }
}

async function loadExpressionContext(): Promise<void> {
  if (!contextStore.sessionId) {
    expressionContext.value = null;
    moduleInsertPicker.periodCode = "";
    moduleInsertPicker.objectRef = "";
    moduleInsertPicker.dataKey = "";
    gradeInsertPicker.periodCode = "";
    gradeInsertPicker.objectRef = "";
    gradeInsertPicker.dataKey = "";
    return;
  }
  expressionContextLoading.value = true;
  try {
    expressionContext.value = await getRuleExpressionContext(
      contextStore.sessionId,
      contextStore.periodCode || "",
      contextStore.objectGroupCode || "",
    );
    ensureExpressionPickerState(moduleInsertPicker);
    ensureExpressionPickerState(gradeInsertPicker);
  } catch {
    expressionContext.value = null;
    moduleInsertPicker.periodCode = "";
    moduleInsertPicker.objectRef = "";
    moduleInsertPicker.dataKey = "";
    gradeInsertPicker.periodCode = "";
    gradeInsertPicker.objectRef = "";
    gradeInsertPicker.dataKey = "";
  } finally {
    expressionContextLoading.value = false;
  }
}

function appendScriptSnippet(currentText: string, snippet: string): string {
  const normalizedSnippet = String(snippet || "").trim();
  if (!normalizedSnippet) {
    return currentText;
  }
  const existing = String(currentText || "");
  if (!existing.trim()) {
    return normalizedSnippet;
  }
  const divider = existing.endsWith("\n") ? "" : "\n";
  return existing + divider + normalizedSnippet;
}

function insertModuleScriptSnippet(snippet: string): void {
  if (!canEditRule.value) {
    return;
  }
  moduleDetailDraft.customScript = appendScriptSnippet(moduleDetailDraft.customScript, snippet);
}

function insertGradeScriptSnippet(snippet: string): void {
  if (!canEditRule.value) {
    return;
  }
  gradeDetailDraft.extraConditionEnabled = true;
  gradeDetailDraft.extraConditionScript = appendScriptSnippet(gradeDetailDraft.extraConditionScript, snippet);
}

function insertModuleSelectedExpression(): void {
  if (!canEditRule.value || !moduleInsertCodePreview.value) {
    return;
  }
  insertModuleScriptSnippet(moduleInsertCodePreview.value);
}

function insertGradeSelectedExpression(): void {
  if (!canEditRule.value || !gradeInsertCodePreview.value) {
    return;
  }
  insertGradeScriptSnippet(gradeInsertCodePreview.value);
}

function handleMethodChange(module: ScoreModule): void {
  if (isExtraAdjustModule(module)) {
    module.calculationMethod = "direct_input";
    module.customScript = "";
    module.voteConfigJson = "";
    return;
  }
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
  const targetModule = activeScopedRule.value.scoreModules[index];
  if (isExtraAdjustModule(targetModule)) {
    event.preventDefault();
    return;
  }
  draggingModuleIndex.value = index;
  draggingModuleId.value = targetModule?.id || "";
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
  if (module.calculationMethod === "vote") {
    const parsed = parseVoteGradeRowsFromConfig(module.voteConfigJson || "");
    moduleVoteGradeRows.value = parsed.rows;
    moduleVoteSubjectRows.value = parseVoteSubjectRowsFromConfig(parsed.extras);
    moduleVoteConfigExtras.value = parsed.extras;
  } else {
    moduleVoteGradeRows.value = [];
    moduleVoteSubjectRows.value = [];
    moduleVoteConfigExtras.value = {};
  }
  ensureExpressionPickerState(moduleInsertPicker);
  moduleDetailVisible.value = true;
}

function closeModuleDetail(): void {
  moduleDetailVisible.value = false;
  moduleDetailTargetId.value = "";
  moduleDetailDraft.customScript = "";
  moduleDetailDraft.voteConfigJson = "";
  moduleVoteGradeRows.value = [];
  moduleVoteSubjectRows.value = [];
  moduleVoteConfigExtras.value = {};
}

function addVoteGradeRow(): void {
  moduleVoteGradeRows.value.push({
    id: uuid("vote_grade"),
    label: `挡位${moduleVoteGradeRows.value.length + 1}`,
    score: null,
  });
}

function removeVoteGradeRow(index: number): void {
  if (moduleVoteGradeRows.value.length <= 1) {
    return;
  }
  moduleVoteGradeRows.value.splice(index, 1);
}

function addVoteSubjectRow(): void {
  moduleVoteSubjectRows.value.push({
    id: uuid("vote_subject"),
    label: `主体${moduleVoteSubjectRows.value.length + 1}`,
    weight: null,
  });
}

function removeVoteSubjectRow(index: number): void {
  if (moduleVoteSubjectRows.value.length <= 1) {
    return;
  }
  moduleVoteSubjectRows.value.splice(index, 1);
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
    const cleanedRows = moduleVoteGradeRows.value.map((item, index) => ({
      label: String(item.label || "").trim() || `挡位${index + 1}`,
      score: toNullableNumber(item.score),
    }));
    const cleanedSubjects = moduleVoteSubjectRows.value.map((item, index) => ({
      label: String(item.label || "").trim() || `主体${index + 1}`,
      weight: toNullableNumber(item.weight),
    }));
    if (cleanedRows.length === 0) {
      ElMessage.warning("请至少配置一个投票挡位");
      return;
    }
    if (cleanedSubjects.length === 0) {
      ElMessage.warning("请至少配置一个投票主体");
      return;
    }
    const seenLabels = new Set<string>();
    for (const row of cleanedRows) {
      if (row.score === null || !Number.isFinite(row.score)) {
        ElMessage.warning(`请填写挡位「${row.label}」的分值`);
        return;
      }
      if (row.score < 0 || row.score > 100) {
        ElMessage.warning(`挡位「${row.label}」分值必须在 0 到 100 之间`);
        return;
      }
      if (seenLabels.has(row.label)) {
        ElMessage.warning(`挡位名称「${row.label}」重复，请调整`);
        return;
      }
      seenLabels.add(row.label);
    }
    const seenSubjectLabels = new Set<string>();
    for (const subject of cleanedSubjects) {
      if (subject.weight === null || !Number.isFinite(subject.weight)) {
        ElMessage.warning(`请填写主体「${subject.label}」的权重`);
        return;
      }
      if (subject.weight <= 0) {
        ElMessage.warning(`主体「${subject.label}」权重必须大于 0`);
        return;
      }
      if (seenSubjectLabels.has(subject.label)) {
        ElMessage.warning(`主体名称「${subject.label}」重复，请调整`);
        return;
      }
      seenSubjectLabels.add(subject.label);
    }
    const voteConfig = {
      ...moduleVoteConfigExtras.value,
      gradeScores: cleanedRows.map((item) => ({
        label: item.label,
        score: item.score as number,
      })),
      voterSubjects: cleanedSubjects.map((item) => ({
        label: item.label,
        weight: item.weight as number,
      })),
    };
    target.voteConfigJson = JSON.stringify(voteConfig, null, 2);
    moduleDetailDraft.voteConfigJson = target.voteConfigJson;
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
  gradeDetailDraft.extraConditionEnabled = Boolean(grade.extraConditionEnabled);
  ensureExpressionPickerState(gradeInsertPicker);
  gradeDetailVisible.value = true;
}

function closeGradeDetail(): void {
  gradeDetailVisible.value = false;
  gradeDetailTargetId.value = "";
  gradeDetailDraft.extraConditionScript = "";
  gradeDetailDraft.extraConditionEnabled = false;
}

function applyGradeDetail(): void {
  const target = gradeDetailTarget.value;
  if (!target) {
    closeGradeDetail();
    return;
  }
  target.extraConditionScript = String(gradeDetailDraft.extraConditionScript || "");
  target.extraConditionEnabled = Boolean(gradeDetailDraft.extraConditionEnabled);
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

    activeScopedRule.value.scoreModules = ensureScoreModulesWithExtraAdjust(
      sourceScopedRule.scoreModules.map((item, index) =>
        normalizeScoreModule(
          {
            ...cloneDeep(item),
            id: uuid("module"),
            moduleKey: String(item.moduleKey || `module_${index + 1}`).trim() || `module_${index + 1}`,
          },
          index,
        ),
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
  const editableModules = activeScopedRule.value.scoreModules.filter((item) => !isExtraAdjustModule(item));
  editableModules.push(newScoreModule(`模块${editableModules.length + 1}`, 0));
  activeScopedRule.value.scoreModules = ensureScoreModulesWithExtraAdjust(editableModules);
}

async function removeScoreModule(module: ScoreModule): Promise<void> {
  if (!canEditRule.value || !activeScopedRule.value) {
    return;
  }
  if (isExtraAdjustModule(module)) {
    ElMessage.warning("额外加减分模块为系统固定项，不可删除");
    return;
  }
  const moduleName = module.moduleName.trim() || "未命名模块";
  try {
    await ElMessageBox.confirm(`确认删除模块「${moduleName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
    activeScopedRule.value.scoreModules = ensureScoreModulesWithExtraAdjust(
      activeScopedRule.value.scoreModules.filter((item) => item.id !== module.id),
    );
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
  const editableModules = row.scoreModules.filter((item) => !isExtraAdjustModule(item));
  const normalizedModules = editableModules.map((module, index) => {
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
  normalizedModules.push({
    id: EXTRA_ADJUST_MODULE_KEY,
    moduleKey: EXTRA_ADJUST_MODULE_KEY,
    moduleName: EXTRA_ADJUST_MODULE_NAME,
    weight: 0,
    calculationMethod: "direct_input",
    customScript: "",
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
    extraConditionEnabled: Boolean(grade.extraConditionEnabled),
    extraConditionScript: String(grade.extraConditionScript || "").trim(),
    conditionLogic: normalizeLogic(grade.conditionLogic),
    maxRatioPercent: toNullableNumber(grade.maxRatioPercent),
    maxRatioRoundingMode: normalizeMaxRatioRoundingMode(grade.maxRatioRoundingMode),
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
    const editableModules = scoped.scoreModules.filter((item) => !isExtraAdjustModule(item));

    if (editableModules.length === 0) {
      return `${title}至少需要一个分数模块`;
    }
    const total = editableModules.reduce((sum, item) => sum + item.weight, 0);
    if (total <= 0) {
      return `${title}的模块总权重必须大于 0`;
    }

    for (const module of editableModules) {
      if (!module.moduleName.trim()) {
        return `${title}存在空模块名称`;
      }
      if (module.weight <= 0) {
        return `${title}中模块「${module.moduleName}」权重必须大于 0`;
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
      const enabledExtraCondition = Boolean(grade.extraConditionEnabled);
      const extraScriptText = String(grade.extraConditionScript || "").trim();
      if (enabledExtraCondition && !extraScriptText) {
        return `${title}中等第「${grade.title}」启用额外脚本后，脚本不能为空`;
      }
      if (!node.hasLowerLimit && !node.hasUpperLimit && !(enabledExtraCondition && extraScriptText)) {
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
      if (!["real", "floor", "ceil"].includes(grade.maxRatioRoundingMode)) {
        return `${title}中等第「${grade.title}」人数比例取整模式不合法`;
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

  const synchronizedPeriods = applySharedPeriodBindingToActiveScopedRule();
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
    if (synchronizedPeriods.length > 1) {
      ElMessage.success(`规则已保存，并已同步到周期：${synchronizedPeriods.join(", ")}`);
    } else {
      ElMessage.success("规则已保存");
    }
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
    void loadExpressionContext();
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
  () => [
    expressionPeriods.value.join("|"),
    expressionInsertObjectOptions.value.map((item) => item.value).join("|"),
    expressionDataOptions.value.map((item) => item.value).join("|"),
  ],
  () => {
    ensureExpressionPickerState(moduleInsertPicker);
    ensureExpressionPickerState(gradeInsertPicker);
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

watch(
  () => props.initialEditTab,
  (value) => {
    if (value) {
      activeEditTab.value = value;
    }
  },
  { immediate: true },
);

watch(
  () => activeEditTab.value,
  (value) => {
    if (lockEditTab.value && value !== props.initialEditTab) {
      activeEditTab.value = props.initialEditTab;
    }
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

.editor-tabs.is-locked-tab :deep(.el-tabs__header) {
  display: none;
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

.grade-extra-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  color: #606266;
  font-size: 13px;
}

.script-helper-panel {
  margin-top: 12px;
  padding: 10px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  background: #fafafa;
}

.script-helper-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #303133;
}

.script-helper-loading {
  font-size: 12px;
  color: #909399;
  font-weight: 400;
}

.script-picker-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.script-picker-field {
  min-width: 0;
}

.script-picker-label {
  margin-bottom: 6px;
  font-size: 12px;
  color: #606266;
}

.script-picker-select {
  width: 100%;
}

.script-picker-preview {
  margin-top: 10px;
  display: grid;
  gap: 6px;
}

.script-picker-preview-label {
  font-size: 12px;
  color: #606266;
}

.script-picker-preview-code {
  display: block;
  min-height: 34px;
  padding: 6px 8px;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  background: #fff;
  color: #303133;
  font-size: 12px;
  line-height: 1.6;
  word-break: break-all;
}

.script-picker-actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.script-helper-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.script-helper-group-filter {
  width: 240px;
  max-width: 100%;
}

.script-helper-search {
  width: 260px;
  max-width: 100%;
}

.script-helper-section + .script-helper-section {
  margin-top: 8px;
}

.script-helper-title {
  font-size: 12px;
  color: #606266;
  margin-bottom: 4px;
}

.script-helper-items {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.script-helper-rich-list {
  display: grid;
  gap: 8px;
}

.script-helper-rich-item {
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  padding: 8px;
  background: #fff;
}

.script-helper-rich-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.script-helper-rich-name {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  line-height: 1.4;
}

.script-helper-rich-type {
  flex: 0 0 auto;
  font-size: 12px;
  color: #909399;
  line-height: 1.2;
}

.script-helper-rich-desc {
  margin-top: 4px;
  font-size: 12px;
  color: #606266;
  line-height: 1.5;
}

.script-helper-rich-footer {
  margin-top: 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.script-helper-code {
  flex: 1;
  min-width: 0;
  display: inline-block;
  padding: 2px 6px;
  border-radius: 4px;
  background: #f5f7fa;
  color: #303133;
  font-size: 12px;
  line-height: 1.6;
  word-break: break-all;
}

.script-helper-empty {
  font-size: 12px;
  color: #909399;
}

@media (max-width: 900px) {
  .script-picker-grid {
    grid-template-columns: 1fr;
  }
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
.rules-table :deep(.grade-logic-select) {
  width: 100%;
}

.grade-ratio-inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.grade-ratio-unit {
  color: #909399;
  font-size: 12px;
  line-height: 1;
}

.rules-table :deep(.grade-ratio-input) {
  width: 92px;
}

.rules-table :deep(.grade-ratio-mode-select) {
  width: 88px;
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


