<template>
  <div ref="assessmentViewRef" class="assessment-view">
    <el-tabs v-model="activeTab">
      <el-tab-pane label="鑰冩牳鍦烘" name="sessions">
        <el-card>
          <div class="tool-row">
            <div class="header-actions">
              <el-button :loading="loadingSessions" @click="loadSessions">鍒锋柊</el-button>
              <el-button type="primary" :disabled="!canEdit" @click="openCreateDialog">
                鍒涘缓鑰冩牳鍦烘
              </el-button>
            </div>
          </div>

                    <el-table v-loading="loadingSessions" :data="sessions" border>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="displayName" label="场次名称" min-width="260" />
            <el-table-column prop="year" label="年度" width="100" />
            <el-table-column label="状态" width="220">
              <template #default="{ row }">
                <div class="session-status-cell">
                  <el-tag :type="sessionStatusTagType(row.status)" effect="light">
                    {{ sessionStatusText(row.status) }}
                  </el-tag>
                  <el-select
                    :model-value="row.status"
                    size="small"
                    class="session-status-select"
                    :disabled="!canEdit || changingSessionStatusId === row.id"
                    @change="(value) => onSessionStatusChange(row, value)"
                  >
                    <el-option
                      v-for="option in sessionStatusOptions"
                      :key="option.value"
                      :label="option.label"
                      :value="option.value"
                    />
                  </el-select>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="assessmentName" label="目录名" min-width="220" />
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="selectSession(row.id)">管理</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="鍛ㄦ湡閰嶇疆" name="period">
        <el-card>
          <div class="tool-row">
            <div class="header-actions">
              <el-button :disabled="!selectedDetail" :loading="loadingDetail" @click="reloadCurrent">鍒锋柊</el-button>
            </div>
          </div>

          <el-empty v-if="!selectedDetail" description="璇峰厛鍦ㄨ€冩牳鍦烘鏍囩閫夋嫨涓€涓€冩牳鍦烘" />

          <template v-else>
            <div class="section">
              <div class="tool-row">
                <div class="header-actions">
                  <el-button
                    type="primary"
                    :disabled="!canEditCurrentSession"
                    @click="addPeriod"
                  >
                    鏂板鍛ㄦ湡
                  </el-button>
                </div>
              </div>
              <el-table :data="periodDrafts" border>
                <el-table-column label="#" width="60">
                  <template #default="{ row, $index }">
                    <span
                      class="period-index-tag"
                      :class="{ 'is-shared': periodSharedGroupIndex(row) >= 0 }"
                      :style="periodSharedTagStyle(row)"
                      :title="periodSharedGroupTitle(row)"
                    >
                      {{ $index + 1 }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column label="缂栫爜" width="160">
                  <template #default="{ row }">
                    <el-input v-model="row.periodCode" @blur="onPeriodCodeBlur(row)" />
                  </template>
                </el-table-column>
                <el-table-column label="鍚嶇О" min-width="180">
                  <template #default="{ row }">
                    <el-input v-model="row.periodName" />
                  </template>
                </el-table-column>
                <el-table-column label="鎿嶄綔" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" :disabled="!canEditCurrentSession" @click="removePeriod($index)">鍒犻櫎</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="shared-rules-toggle">
                <el-button size="small" type="primary" plain @click="sharedRulesExpanded = !sharedRulesExpanded">
                  鍏辩敤瑙勫垯
                </el-button>
              </div>
              <el-collapse-transition>
                <div v-show="sharedRulesExpanded" class="binding-section">
                  <div class="section-head">
                    <strong>鍏辩敤瑙勫垯鍒嗙粍</strong>
                    <el-button
                      type="primary"
                      :disabled="!canEditCurrentSession"
                      @click="addRuleBindingGroup"
                    >
                      鏂板鍒嗙粍
                    </el-button>
                  </div>
                  <el-empty
                    v-if="ruleBindingGroups.length === 0"
                    description="未配置分组时，每个周期使用独立规则。"
                  />
                  <div
                    v-for="(group, groupIndex) in ruleBindingGroups"
                    v-else
                    :key="group.id"
                    class="binding-group-row"
                  >
                    <div class="binding-group-head">
                      <span>鍒嗙粍 {{ groupIndex + 1 }}</span>
                      <el-button link type="danger" :disabled="!canEditCurrentSession" @click="removeRuleBindingGroup(group.id)">
                        鍒犻櫎鍒嗙粍
                      </el-button>
                    </div>
                    <el-checkbox-group v-model="group.periodCodes" @change="onRuleBindingGroupChange">
                      <el-checkbox v-for="code in periodCodeOptions" :key="`${group.id}_${code}`" :label="code">
                        {{ periodCodeLabelMap[code] || code }}
                      </el-checkbox>
                    </el-checkbox-group>
                  </div>
                  <div class="period-hint">同组周期将共享规则配置；不在任何分组中的周期使用独立规则。仅绑定规则，不绑定评分数据。</div>
                </div>
              </el-collapse-transition>
              <div class="section-foot">
                <el-button
                  type="primary"
                  :disabled="!canEditCurrentSession || savingPeriods"
                  :loading="savingPeriods"
                  @click="savePeriods"
                >
                  淇濆瓨鍛ㄦ湡
                </el-button>
              </div>
            </div>
          </template>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="瀵硅薄鍒嗙粍閰嶇疆" name="groups">
        <el-card>
          <div class="tool-row">
            <div class="header-actions">
              <el-button :disabled="!selectedDetail" :loading="loadingDetail" @click="reloadCurrent">鍒锋柊</el-button>
            </div>
          </div>

          <el-empty v-if="!selectedDetail" description="璇峰厛鍦ㄨ€冩牳鍦烘鏍囩閫夋嫨涓€涓€冩牳鍦烘" />

          <template v-else>
            <div class="section">
              <div class="tool-row">
                <div class="header-actions">
                  <el-button
                    type="primary"
                    :disabled="!canEditCurrentSession"
                    @click="addGroup"
                  >
                    鏂板鍒嗙粍
                  </el-button>
                </div>
              </div>
              <el-table :data="groupDrafts" border>
                <el-table-column type="index" label="#" width="60" />
                <el-table-column label="绫诲瀷" width="120">
                  <template #default="{ row }">
                    <el-select v-model="row.objectType">
                      <el-option label="鍥綋" value="team" />
                      <el-option label="涓汉" value="individual" />
                    </el-select>
                  </template>
                </el-table-column>
                <el-table-column label="缂栫爜" width="180">
                  <template #default="{ row }">
                    <el-input v-model="row.groupCode" />
                  </template>
                </el-table-column>
                <el-table-column label="鍚嶇О" min-width="180">
                  <template #default="{ row }">
                    <el-input v-model="row.groupName" />
                  </template>
                </el-table-column>
                <el-table-column label="鎿嶄綔" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" :disabled="!canEditCurrentSession" @click="removeGroup($index)">鍒犻櫎</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="section-foot">
                <el-button
                  type="primary"
                  :disabled="!canEditCurrentSession || savingGroups"
                  :loading="savingGroups"
                  @click="saveGroups"
                >
                  淇濆瓨鍒嗙粍
                </el-button>
              </div>
            </div>
          </template>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="鑰冩牳瀵硅薄" name="objects">
        <el-card>
          <div class="tool-row">
            <div class="header-actions">
              <el-button :disabled="!selectedDetail" :loading="loadingDetail" @click="reloadCurrent">鍒锋柊</el-button>
            </div>
          </div>

          <el-empty v-if="!selectedDetail" description="璇峰厛鍦ㄨ€冩牳鍦烘鏍囩閫夋嫨涓€涓€冩牳鍦烘" />

          <template v-else>
            <div class="section">
              <div class="tool-row">
                <div class="header-actions">
                  <el-button
                    type="primary"
                    :disabled="!canEditCurrentSession"
                    @click="openObjectDialog"
                  >
                    鏂板瀵硅薄
                  </el-button>
                  <el-button
                    type="primary"
                    :disabled="!canEditCurrentSession || savingObjects"
                    :loading="savingObjects"
                    @click="saveObjects"
                  >
                    淇濆瓨瀵硅薄
                  </el-button>
                  <el-button
                    :disabled="!canEditCurrentSession || resettingObjects"
                    :loading="resettingObjects"
                    @click="resetObjects"
                  >
                    閲嶇疆涓洪粯璁?                  </el-button>
                </div>
              </div>
              <el-table v-loading="loadingObjects" :data="objectDrafts" border>
                <el-table-column prop="id" label="ID" width="80" />
                <el-table-column label="绫诲瀷" width="100">
                  <template #default="{ row }">
                    {{ row.objectType === "team" ? "鍥綋" : "涓汉" }}
                  </template>
                </el-table-column>
                <el-table-column prop="groupCode" label="鍒嗙粍缂栫爜" width="180" />
                <el-table-column label="鍒嗙粍鍚嶇О" width="180">
                  <template #default="{ row }">
                    {{ groupNameByCode[row.groupCode] || row.groupCode }}
                  </template>
                </el-table-column>
                <el-table-column label="鏉ユ簮绫诲瀷" width="140">
                  <template #default="{ row }">
                    {{ row.targetType === "department" ? "閮ㄩ棬" : row.targetType === "organization" ? "缁勭粐" : "浜哄憳" }}
                  </template>
                </el-table-column>
                <el-table-column prop="objectName" label="瀵硅薄鍚嶇О" min-width="220" />
                <el-table-column prop="targetId" label="鏉ユ簮ID" width="100" />
                <el-table-column label="鎿嶄綔" width="100">
                  <template #default="{ $index }">
                    <el-button link type="danger" :disabled="!canEditCurrentSession" @click="removeObject($index)">鍒犻櫎</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </template>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="createVisible" title="鍒涘缓鑰冩牳鍦烘" width="620px">
      <el-form label-width="110px">
        <el-form-item label="缁勭粐" required>
          <el-select v-model="createForm.organizationId" filterable style="width: 100%">
            <el-option
              v-for="item in organizations"
              :key="item.id"
              :label="item.orgName"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="骞村害" required>
          <el-input-number v-model="createForm.year" :min="2000" :max="9999" />
        </el-form-item>
        <el-form-item label="鍦烘鍚嶇О">
          <el-input
            v-model="createForm.displayName"
            placeholder="榛樿锛氬綋鍓嶅勾浠?缁勭粐鍚?鑰冩牳"
            @input="markCreateNameTouched"
          />
        </el-form-item>
        <el-form-item label="璇存槑">
          <el-input v-model="createForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">鍙栨秷</el-button>
        <el-button type="primary" :loading="creating" @click="createSession">鍒涘缓</el-button>
      </template>
    </el-dialog>

        <el-dialog v-model="objectDialogVisible" title="新增考核对象" width="780px">
      <el-form label-width="96px">
        <el-form-item label="筛选类型">
          <el-radio-group v-model="candidateFilter.targetType">
            <el-radio-button label="">全部</el-radio-button>
            <el-radio-button label="department">部门</el-radio-button>
            <el-radio-button label="organization">次级组织</el-radio-button>
            <el-radio-button label="employee">人员</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="关键字">
          <el-input v-model="candidateFilter.keyword" clearable placeholder="按对象/组织/部门名称筛选" />
        </el-form-item>
        <el-form-item label="候选对象" required>
          <el-table v-loading="loadingCandidates" :data="filteredCandidates" border height="280">
            <el-table-column label="选择" width="90">
              <template #default="{ row }">
                <el-button
                  link
                  type="primary"
                  @click="pickCandidate(row)"
                >
                  {{ selectedCandidateKey === candidateKey(row) ? "已选" : "选择" }}
                </el-button>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="110">
              <template #default="{ row }">
                {{ row.targetType === "department" ? "部门" : row.targetType === "organization" ? "次级组织" : "人员" }}
              </template>
            </el-table-column>
            <el-table-column prop="objectName" label="对象" min-width="160" />
            <el-table-column prop="organizationName" label="所属组织" min-width="160" />
            <el-table-column prop="departmentName" label="所属部门" min-width="160" />
          </el-table>
        </el-form-item>
        <el-form-item label="对象分组" required>
          <el-select v-model="objectDialog.groupCode" style="width: 100%">
            <el-option
              v-for="item in candidateGroupOptions"
              :key="item.groupCode"
              :label="`${item.groupName} (${item.groupCode})`"
              :value="item.groupCode"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="objectDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="appendObjectFromCandidate">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { useUnsavedStore } from "@/stores/unsaved";
import {
  createAssessmentSession,
  getAssessmentSession,
  listAssessmentObjectCandidates,
  listAssessmentSessionObjects,
  listAssessmentSessions,
  resetAssessmentSessionObjects,
  updateAssessmentObjectGroups,
  updateAssessmentObjects,
  updateAssessmentPeriods,
  updateAssessmentSessionStatus,
} from "@/api/assessment";
import { listOrganizations } from "@/api/org";
import type {
  AssessmentObjectCandidateItem,
  AssessmentObjectGroupItem,
  AssessmentSessionDetail,
  AssessmentSessionStatus,
  AssessmentSessionObjectItem,
  AssessmentSessionPeriodItem,
} from "@/types/assessment";
import type { OrganizationItem } from "@/types/org";

const appStore = useAppStore();
const contextStore = useContextStore();
const unsavedStore = useUnsavedStore();
const canEdit = computed(() => appStore.hasPermission("assessment:update"));
const selectedSessionStatus = computed<AssessmentSessionStatus>(() =>
  (selectedDetail.value?.session.status || "preparing") as AssessmentSessionStatus,
);
const canEditCurrentSession = computed(() => canEdit.value && selectedSessionStatus.value !== "completed");
const periodDirtySourceId = "assessment:periods";
const groupDirtySourceId = "assessment:groups";
const objectDirtySourceId = "assessment:objects";

const sessions = ref<AssessmentSessionDetail["session"][]>([]);
const selectedSessionId = ref<number | undefined>();
const selectedDetail = ref<AssessmentSessionDetail | null>(null);

const loadingSessions = ref(false);
const loadingDetail = ref(false);
const loadingObjects = ref(false);
const activeTab = ref<"sessions" | "period" | "groups" | "objects">("sessions");
const assessmentViewRef = ref<HTMLElement>();

const periodDrafts = ref<Array<{ periodCode: string; periodName: string; ruleBindingKey: string }>>([]);
const ruleBindingGroups = ref<Array<{ id: string; periodCodes: string[] }>>([]);
const groupDrafts = ref<Array<{ objectType: "team" | "individual"; groupCode: string; groupName: string }>>([]);
const objects = ref<AssessmentSessionObjectItem[]>([]);
const objectDrafts = ref<AssessmentSessionObjectItem[]>([]);

const savingPeriods = ref(false);
const savingGroups = ref(false);
const savingObjects = ref(false);
const resettingObjects = ref(false);
const changingSessionStatusId = ref<number | null>(null);

const organizations = ref<OrganizationItem[]>([]);
const createVisible = ref(false);
const creating = ref(false);
const createNameTouched = ref(false);
const createForm = reactive({
  year: new Date().getFullYear(),
  organizationId: undefined as number | undefined,
  displayName: "",
  description: "",
});

const objectDialogVisible = ref(false);
const loadingCandidates = ref(false);
const candidates = ref<AssessmentObjectCandidateItem[]>([]);
const selectedCandidateKey = ref("");
const candidateFilter = reactive({
  targetType: "",
  keyword: "",
});
const objectDialog = reactive({
  groupCode: "",
});
const periodBaseline = ref("");
const groupBaseline = ref("");
const objectBaseline = ref("");
const sharedRulesExpanded = ref(false);
const sharedRuleTagPalette = [
  { bg: "#ecf5ff", fg: "#409eff" },
  { bg: "#f0f9eb", fg: "#67c23a" },
  { bg: "#fdf6ec", fg: "#e6a23c" },
  { bg: "#fef0f0", fg: "#f56c6c" },
  { bg: "#f0fafa", fg: "#14b8a6" },
  { bg: "#f4f4f5", fg: "#606266" },
];
const sessionStatusOptions: Array<{ value: AssessmentSessionStatus; label: string }> = [
  { value: "preparing", label: "准备中" },
  { value: "active", label: "进行中" },
  { value: "completed", label: "已完成" },
];

function sessionStatusText(status: string): string {
  switch (status) {
    case "active":
      return "进行中";
    case "completed":
      return "已完成";
    case "preparing":
    default:
      return "准备中";
  }
}

function sessionStatusTagType(status: string): "info" | "success" | "warning" {
  switch (status) {
    case "active":
      return "success";
    case "completed":
      return "info";
    case "preparing":
    default:
      return "warning";
  }
}

const groupNameByCode = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {};
  for (const item of groupDrafts.value) {
    map[item.groupCode] = item.groupName;
  }
  return map;
});

const existingObjectKeySet = computed(() => {
  const set = new Set<string>();
  for (const item of objectDrafts.value) {
    set.add(`${item.targetType}:${item.targetId}`);
  }
  return set;
});

const selectedCandidate = computed(() =>
  candidates.value.find((item) => candidateKey(item) === selectedCandidateKey.value),
);

const candidateGroupOptions = computed(() => {
  const selected = selectedCandidate.value;
  if (!selected) {
    return [] as Array<{ objectType: "team" | "individual"; groupCode: string; groupName: string }>;
  }
  const objectType = selected.recommendedObjectType;
  return groupDrafts.value.filter((item) => item.objectType === objectType);
});

const filteredCandidates = computed(() => {
  const keyword = candidateFilter.keyword.trim().toLowerCase();
  return candidates.value.filter((item) => {
    if (existingObjectKeySet.value.has(candidateKey(item))) {
      return false;
    }
    if (candidateFilter.targetType && item.targetType !== candidateFilter.targetType) {
      return false;
    }
    if (!keyword) {
      return true;
    }
    return [item.objectName, item.organizationName, item.departmentName || ""].some((text) =>
      text.toLowerCase().includes(keyword),
    );
  });
});

const periodCodeOptions = computed(() => {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    if (!code || seen.has(code)) {
      continue;
    }
    seen.add(code);
    result.push(code);
  }
  return result;
});

const periodCodeLabelMap = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {};
  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    if (!code) {
      continue;
    }
    const name = item.periodName.trim();
    map[code] = name ? `${code} - ${name}` : code;
  }
  return map;
});

function normalizePeriodCode(code: string): string {
  return String(code || "").trim().toUpperCase();
}

const sharedRuleGroupIndexByPeriodCode = computed(() => {
  const map = new Map<string, number>();
  for (let groupIndex = 0; groupIndex < ruleBindingGroups.value.length; groupIndex += 1) {
    const group = ruleBindingGroups.value[groupIndex];
    if (group.periodCodes.length <= 1) {
      continue;
    }
    for (const codeRaw of group.periodCodes) {
      const code = normalizePeriodCode(codeRaw);
      if (!code || map.has(code)) {
        continue;
      }
      map.set(code, groupIndex);
    }
  }
  return map;
});

function periodSharedGroupIndex(item: { periodCode: string }): number {
  const code = normalizePeriodCode(item.periodCode);
  if (!code) {
    return -1;
  }
  const hit = sharedRuleGroupIndexByPeriodCode.value.get(code);
  if (hit === undefined) {
    return -1;
  }
  return hit;
}

function periodSharedTagStyle(item: { periodCode: string }): Record<string, string> | undefined {
  const groupIndex = periodSharedGroupIndex(item);
  if (groupIndex < 0) {
    return undefined;
  }
  const tone = sharedRuleTagPalette[groupIndex % sharedRuleTagPalette.length];
  return {
    backgroundColor: tone.bg,
    color: tone.fg,
  };
}

function periodSharedGroupTitle(item: { periodCode: string }): string {
  const groupIndex = periodSharedGroupIndex(item);
  if (groupIndex < 0) {
    return "";
  }
  return `鍏辩敤瑙勫垯鍒嗙粍 ${groupIndex + 1}`;
}

function periodDraftSignature(): string {
  const periods = periodDrafts.value.map((item) => ({
    periodCode: item.periodCode.trim().toUpperCase(),
    periodName: item.periodName.trim(),
    ruleBindingKey: item.ruleBindingKey.trim().toUpperCase(),
  }));
  const bindings = ruleBindingGroups.value.map((group) => ({
    periodCodes: group.periodCodes.map((code) => String(code || "").trim().toUpperCase()),
  }));
  return JSON.stringify({ periods, bindings });
}

function groupDraftSignature(): string {
  return JSON.stringify(
    groupDrafts.value.map((item) => ({
      objectType: item.objectType,
      groupCode: item.groupCode.trim(),
      groupName: item.groupName.trim(),
    })),
  );
}

function objectDraftSignature(): string {
  return JSON.stringify(
    objectDrafts.value.map((item) => ({
      objectType: item.objectType,
      groupCode: item.groupCode,
      targetType: item.targetType,
      targetId: item.targetId,
      objectName: item.objectName,
      sortOrder: item.sortOrder,
      isActive: item.isActive,
    })),
  );
}

function resetPeriodBaseline(): void {
  periodBaseline.value = periodDraftSignature();
  unsavedStore.clearDirty(periodDirtySourceId);
}

function resetGroupBaseline(): void {
  groupBaseline.value = groupDraftSignature();
  unsavedStore.clearDirty(groupDirtySourceId);
}

function resetObjectBaseline(): void {
  objectBaseline.value = objectDraftSignature();
  unsavedStore.clearDirty(objectDirtySourceId);
}

function syncPeriodDirty(): void {
  if (!selectedDetail.value || !periodBaseline.value) {
    unsavedStore.clearDirty(periodDirtySourceId);
    return;
  }
  const current = periodDraftSignature();
  if (current === periodBaseline.value) {
    unsavedStore.clearDirty(periodDirtySourceId);
    return;
  }
  unsavedStore.markDirty(periodDirtySourceId);
}

function syncGroupDirty(): void {
  if (!selectedDetail.value || !groupBaseline.value) {
    unsavedStore.clearDirty(groupDirtySourceId);
    return;
  }
  const current = groupDraftSignature();
  if (current === groupBaseline.value) {
    unsavedStore.clearDirty(groupDirtySourceId);
    return;
  }
  unsavedStore.markDirty(groupDirtySourceId);
}

function syncObjectDirty(): void {
  if (!selectedDetail.value || !objectBaseline.value) {
    unsavedStore.clearDirty(objectDirtySourceId);
    return;
  }
  const current = objectDraftSignature();
  if (current === objectBaseline.value) {
    unsavedStore.clearDirty(objectDirtySourceId);
    return;
  }
  unsavedStore.markDirty(objectDirtySourceId);
}

function isDialogCancel(error: unknown): boolean {
  return (
    error === "cancel" ||
    error === "close" ||
    (error instanceof Error && (error.message === "cancel" || error.message === "close"))
  );
}

function hasBlockingDialogOpen(): boolean {
  return createVisible.value || objectDialogVisible.value;
}

function isSystemWindowActive(): boolean {
  return document.visibilityState === "visible" && document.hasFocus();
}

function isAssessmentViewShortcutScope(event: KeyboardEvent): boolean {
  const root = assessmentViewRef.value;
  const target = event.target;
  if (!root || !(target instanceof Node)) {
    return false;
  }
  if (target === document.body) {
    return true;
  }
  return root.contains(target);
}

async function saveActiveTab(): Promise<void> {
  if (activeTab.value === "period") {
    await savePeriods();
    return;
  }
  if (activeTab.value === "groups") {
    await saveGroups();
    return;
  }
  if (activeTab.value === "objects") {
    await saveObjects();
  }
}

function canTriggerSaveShortcut(): boolean {
  if (!canEditCurrentSession.value) {
    return false;
  }
  if (activeTab.value === "period") {
    return Boolean(selectedSessionId.value) && !savingPeriods.value && !loadingDetail.value;
  }
  if (activeTab.value === "groups") {
    return Boolean(selectedSessionId.value) && !savingGroups.value && !loadingDetail.value;
  }
  if (activeTab.value === "objects") {
    return Boolean(selectedSessionId.value) && !savingObjects.value && !loadingDetail.value;
  }
  return false;
}

function canTriggerCreateShortcut(): boolean {
  if (!canEdit.value) {
    return false;
  }
  if (activeTab.value === "sessions") {
    return !creating.value;
  }
  if (activeTab.value === "period") {
    return Boolean(selectedDetail.value) && canEditCurrentSession.value && !loadingDetail.value;
  }
  if (activeTab.value === "groups") {
    return Boolean(selectedDetail.value) && canEditCurrentSession.value && !loadingDetail.value;
  }
  if (activeTab.value === "objects") {
    return Boolean(selectedDetail.value) && canEditCurrentSession.value && !loadingDetail.value && !loadingCandidates.value;
  }
  return false;
}

function createInActiveTab(): void {
  if (activeTab.value === "sessions") {
    openCreateDialog();
    return;
  }
  if (activeTab.value === "period") {
    addPeriod();
    return;
  }
  if (activeTab.value === "groups") {
    addGroup();
    return;
  }
  if (activeTab.value === "objects") {
    void openObjectDialog();
  }
}

function handleAssessmentViewKeydown(event: KeyboardEvent): void {
  const ctrlOrMeta = event.ctrlKey || event.metaKey;
  if (!ctrlOrMeta || event.altKey) {
    return;
  }
  if (!isSystemWindowActive()) {
    return;
  }
  if (!isAssessmentViewShortcutScope(event)) {
    return;
  }
  if (hasBlockingDialogOpen()) {
    return;
  }
  const key = String(event.key || "").toLowerCase();
  if (key === "s") {
    if (!canTriggerSaveShortcut()) {
      return;
    }
    event.preventDefault();
    void saveActiveTab();
    return;
  }
  if (key === "n") {
    if (!canTriggerCreateShortcut()) {
      return;
    }
    event.preventDefault();
    createInActiveTab();
    return;
  }
}

function bindingGroupId(): string {
  return `binding_group_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function addPeriod(): void {
  periodDrafts.value.push({ periodCode: "", periodName: "", ruleBindingKey: "" });
}

async function removePeriod(index: number): Promise<void> {
  const period = periodDrafts.value[index];
  if (!period) {
    return;
  }
  const periodLabel = period.periodName.trim() || period.periodCode.trim().toUpperCase() || `第 ${index + 1} 个周期`;
  try {
    await ElMessageBox.confirm(`确认删除周期「${periodLabel}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "鍒犻櫎",
      cancelButtonText: "鍙栨秷",
    });
    periodDrafts.value.splice(index, 1);
    ensureRuleBindingGroupsIntegrity();
    ensurePeriodBindingKeys();
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("鍒犻櫎鍛ㄦ湡澶辫触");
  }
}

function onPeriodCodeBlur(row: { periodCode: string; ruleBindingKey: string }): void {
  row.periodCode = row.periodCode.trim().toUpperCase();
  if (!row.ruleBindingKey.trim()) {
    row.ruleBindingKey = row.periodCode;
  }
  ensureRuleBindingGroupsIntegrity();
  ensurePeriodBindingKeys();
}

function addRuleBindingGroup(): void {
  ruleBindingGroups.value.push({ id: bindingGroupId(), periodCodes: [] });
}

async function removeRuleBindingGroup(groupID: string): Promise<void> {
  const groupIndex = ruleBindingGroups.value.findIndex((item) => item.id === groupID);
  if (groupIndex < 0) {
    return;
  }
  try {
    await ElMessageBox.confirm(`纭鍒犻櫎鍒嗙粍 ${groupIndex + 1} 鍚楋紵`, "鍒犻櫎纭", {
      type: "warning",
      confirmButtonText: "鍒犻櫎",
      cancelButtonText: "鍙栨秷",
    });
    ruleBindingGroups.value = ruleBindingGroups.value.filter((item) => item.id !== groupID);
    ensureRuleBindingGroupsIntegrity();
    ensurePeriodBindingKeys();
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("鍒犻櫎鍒嗙粍澶辫触");
  }
}

function onRuleBindingGroupChange(): void {
  ensureRuleBindingGroupsIntegrity();
  ensurePeriodBindingKeys();
}

function ensureRuleBindingGroupsIntegrity(): void {
  const available = new Set(periodCodeOptions.value);
  for (const group of ruleBindingGroups.value) {
    const seen = new Set<string>();
    const normalized: string[] = [];
    for (const codeRaw of group.periodCodes) {
      const code = String(codeRaw || "").trim().toUpperCase();
      if (!code || seen.has(code) || !available.has(code)) {
        continue;
      }
      seen.add(code);
      normalized.push(code);
    }
    group.periodCodes = normalized;
  }
}

function validateRuleBindingGroups(): string {
  const ownerByCode = new Map<string, number>();
  for (let index = 0; index < ruleBindingGroups.value.length; index += 1) {
    const group = ruleBindingGroups.value[index];
    for (const code of group.periodCodes) {
      const owner = ownerByCode.get(code);
      if (owner !== undefined) {
        return `周期「${code}」被分组 ${owner + 1} 和分组 ${index + 1} 重复选择`;
      }
      ownerByCode.set(code, index);
    }
  }
  return "";
}

function applyRuleBindingGroupsToPeriods(): void {
  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    item.ruleBindingKey = code;
  }

  const orderMap = new Map<string, number>();
  periodDrafts.value.forEach((item, index) => {
    const code = item.periodCode.trim().toUpperCase();
    orderMap.set(code, index);
  });

  const periodMap = new Map<string, { periodCode: string; periodName: string; ruleBindingKey: string }>();
  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    if (!code) {
      continue;
    }
    periodMap.set(code, item);
  }

  for (const group of ruleBindingGroups.value) {
    if (group.periodCodes.length <= 1) {
      continue;
    }
    const sortedCodes = [...group.periodCodes].sort((a, b) => (orderMap.get(a) || 0) - (orderMap.get(b) || 0));
    const anchor = sortedCodes[0];
    if (!anchor) {
      continue;
    }
    for (const code of sortedCodes) {
      const item = periodMap.get(code);
      if (item) {
        item.ruleBindingKey = anchor;
      }
    }
  }
}

function buildRuleBindingGroupsFromPeriods(): void {
  const available = new Set(periodCodeOptions.value);
  const groupsByAnchor = new Map<string, string[]>();

  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    if (!code) {
      continue;
    }
    let anchor = item.ruleBindingKey.trim().toUpperCase();
    if (!anchor || !available.has(anchor)) {
      anchor = code;
    }
    const bucket = groupsByAnchor.get(anchor) || [];
    bucket.push(code);
    groupsByAnchor.set(anchor, bucket);
  }

  const result: Array<{ id: string; periodCodes: string[] }> = [];
  for (const codes of groupsByAnchor.values()) {
    const deduped = Array.from(new Set(codes));
    if (deduped.length <= 1) {
      continue;
    }
    result.push({
      id: bindingGroupId(),
      periodCodes: deduped,
    });
  }

  ruleBindingGroups.value = result;
  ensurePeriodBindingKeys();
}

function ensurePeriodBindingKeys(): void {
  const available = new Set(periodCodeOptions.value);
  for (const item of periodDrafts.value) {
    item.periodCode = item.periodCode.trim().toUpperCase();
  }

  applyRuleBindingGroupsToPeriods();

  for (const item of periodDrafts.value) {
    const code = item.periodCode.trim().toUpperCase();
    let bindingKey = item.ruleBindingKey.trim().toUpperCase();
    if (!bindingKey) {
      bindingKey = code;
    }
    if (bindingKey && !available.has(bindingKey)) {
      bindingKey = code;
    }
    item.ruleBindingKey = bindingKey;
  }
}

function addGroup(): void {
  groupDrafts.value.push({ objectType: "team", groupCode: "", groupName: "" });
}

async function removeGroup(index: number): Promise<void> {
  const group = groupDrafts.value[index];
  if (!group) {
    return;
  }
  const groupLabel = group.groupName.trim() || group.groupCode.trim() || `第 ${index + 1} 个分组`;
  try {
    await ElMessageBox.confirm(`确认删除对象分组「${groupLabel}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "鍒犻櫎",
      cancelButtonText: "鍙栨秷",
    });
    groupDrafts.value.splice(index, 1);
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("鍒犻櫎瀵硅薄鍒嗙粍澶辫触");
  }
}

function candidateKey(item: Pick<AssessmentObjectCandidateItem, "targetType" | "targetId">): string {
  return `${item.targetType}:${item.targetId}`;
}

function buildDefaultDisplayName(year: number, organizationId?: number): string {
  if (!organizationId) {
    return `${year}骞磋€冩牳`;
  }
  const organization = organizations.value.find((item) => item.id === organizationId);
  if (!organization) {
    return `${year}骞磋€冩牳`;
  }
  return `${year}年${organization.orgName}考核`;
}

function markCreateNameTouched(): void {
  createNameTouched.value = true;
}

async function loadOrganizations(): Promise<void> {
  organizations.value = await listOrganizations({ status: "active" });
}

function hasSessionInCurrentList(sessionId: number | undefined): boolean {
  if (!sessionId) {
    return false;
  }
  return sessions.value.some((item) => item.id === sessionId);
}

async function loadSessions(): Promise<void> {
  loadingSessions.value = true;
  try {
    sessions.value = await listAssessmentSessions();
    if (sessions.value.length === 0) {
      selectedSessionId.value = undefined;
      selectedDetail.value = null;
      periodDrafts.value = [];
      ruleBindingGroups.value = [];
      groupDrafts.value = [];
      objects.value = [];
      objectDrafts.value = [];
      resetPeriodBaseline();
      resetGroupBaseline();
      resetObjectBaseline();
      return;
    }
    if (!hasSessionInCurrentList(selectedSessionId.value)) {
      selectedSessionId.value = undefined;
    }
    if (!selectedSessionId.value) {
      await selectSession(sessions.value[0].id);
    }
  } catch (_error) {
    ElMessage.error("鍔犺浇鑰冩牳鍦烘澶辫触");
  } finally {
    loadingSessions.value = false;
  }
}

async function loadObjects(sessionId: number): Promise<void> {
  loadingObjects.value = true;
  try {
    objects.value = await listAssessmentSessionObjects(sessionId);
    objectDrafts.value = objects.value.map((item) => ({ ...item }));
  } finally {
    loadingObjects.value = false;
  }
}

async function selectSession(sessionId: number): Promise<void> {
  if (!hasSessionInCurrentList(sessionId)) {
    return;
  }
  selectedSessionId.value = sessionId;
  loadingDetail.value = true;
  try {
    const detail = await getAssessmentSession(sessionId);
    selectedDetail.value = detail;
    periodDrafts.value = detail.periods.map((item) => ({
      periodCode: item.periodCode,
      periodName: item.periodName,
      ruleBindingKey: item.ruleBindingKey || item.periodCode,
    }));
    buildRuleBindingGroupsFromPeriods();
    groupDrafts.value = detail.objectGroups.map((item) => ({
      objectType: item.objectType,
      groupCode: item.groupCode,
      groupName: item.groupName,
    }));
    await loadObjects(sessionId);
    if (contextStore.sessionId !== sessionId) {
      await contextStore.setSession(sessionId);
    }
    resetPeriodBaseline();
    resetGroupBaseline();
    resetObjectBaseline();
  } catch (_error) {
    ElMessage.error("鍔犺浇鍦烘璇︽儏澶辫触");
  } finally {
    loadingDetail.value = false;
  }
}

async function reloadCurrent(): Promise<void> {
  if (!selectedSessionId.value) {
    return;
  }
  await selectSession(selectedSessionId.value);
}

function openCreateDialog(): void {
  createForm.year = new Date().getFullYear();
  createForm.organizationId = organizations.value[0]?.id;
  createNameTouched.value = false;
  createForm.displayName = buildDefaultDisplayName(createForm.year, createForm.organizationId);
  createForm.description = "";
  createVisible.value = true;
}

async function createSession(): Promise<void> {
  if (!createForm.organizationId) {
    ElMessage.warning("璇烽€夋嫨缁勭粐");
    return;
  }
  creating.value = true;
  try {
    const detail = await createAssessmentSession({
      year: createForm.year,
      organizationId: createForm.organizationId,
      displayName: createForm.displayName.trim() || undefined,
      description: createForm.description.trim() || undefined,
    });
    ElMessage.success("鑰冩牳鍦烘鍒涘缓鎴愬姛");
    createVisible.value = false;
    await contextStore.refreshSessions();
    await loadSessions();
    await selectSession(detail.session.id);
  } catch (error) {
    const message = error instanceof Error ? error.message : "鍒涘缓鑰冩牳鍦烘澶辫触";
    ElMessage.error(message);
  } finally {
    creating.value = false;
  }
}

async function savePeriods(): Promise<boolean> {
  if (!canEditCurrentSession.value) {
    return false;
  }
  if (!selectedSessionId.value) {
    return false;
  }
  ensureRuleBindingGroupsIntegrity();
  const groupValidation = validateRuleBindingGroups();
  if (groupValidation) {
    ElMessage.warning(groupValidation);
    return false;
  }
  ensurePeriodBindingKeys();

  const items = periodDrafts.value.map((item, index) => ({
    periodCode: item.periodCode.trim().toUpperCase(),
    periodName: item.periodName.trim(),
    ruleBindingKey: item.ruleBindingKey.trim().toUpperCase(),
    sortOrder: index + 1,
  }));
  if (items.some((item) => !item.periodCode || !item.periodName)) {
    ElMessage.warning("周期编码和名称不能为空");
    return false;
  }
  const duplicateCheck = new Set<string>();
  for (const item of items) {
    if (duplicateCheck.has(item.periodCode)) {
      ElMessage.warning(`周期编码「${item.periodCode}」重复，请检查`);
      return false;
    }
    duplicateCheck.add(item.periodCode);
  }
  const codeSet = new Set(items.map((item) => item.periodCode));
  for (const item of items) {
    if (!item.ruleBindingKey) {
      item.ruleBindingKey = item.periodCode;
    }
    if (!codeSet.has(item.ruleBindingKey)) {
      ElMessage.warning(`规则绑定周期「${item.ruleBindingKey}」不存在，请检查周期配置`);
      return false;
    }
  }
  savingPeriods.value = true;
  try {
    await updateAssessmentPeriods(selectedSessionId.value, { items });
    ElMessage.success("周期已保存");
    await contextStore.refreshCurrentDetail();
    await reloadCurrent();
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : "淇濆瓨鍛ㄦ湡澶辫触";
    ElMessage.error(message);
    return false;
  } finally {
    savingPeriods.value = false;
  }
}

async function onSessionStatusChange(
  row: AssessmentSessionDetail["session"],
  nextStatusRaw: string,
): Promise<void> {
  if (!canEdit.value) {
    return;
  }
  const nextStatus = String(nextStatusRaw || "").trim() as AssessmentSessionStatus;
  if (!row?.id || !nextStatus || row.status === nextStatus) {
    return;
  }
  if (!sessionStatusOptions.some((item) => item.value === nextStatus)) {
    ElMessage.warning("无效的场次状态");
    return;
  }

  if (nextStatus === "completed") {
    try {
      await ElMessageBox.confirm(
        "切换到“已完成”后，将创建该场次唯一数据快照，并锁定场次数据与规则（包括 Root 在内均不可修改）。确认继续？",
        "完成确认",
        {
          type: "warning",
          confirmButtonText: "确认完成",
          cancelButtonText: "取消",
        },
      );
    } catch (error) {
      if (isDialogCancel(error)) {
        return;
      }
      ElMessage.error("状态切换已取消");
      return;
    }
  }

  changingSessionStatusId.value = row.id;
  try {
    await updateAssessmentSessionStatus(row.id, { status: nextStatus });
    ElMessage.success(`场次状态已更新为「${sessionStatusText(nextStatus)}」`);

    const shouldRefreshDetail = selectedSessionId.value === row.id;
    await contextStore.refreshSessions();
    await loadSessions();
    if (shouldRefreshDetail) {
      await selectSession(row.id);
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "更新场次状态失败";
    ElMessage.error(message);
  } finally {
    changingSessionStatusId.value = null;
  }
}

async function saveGroups(): Promise<boolean> {
  if (!canEditCurrentSession.value) {
    return false;
  }
  if (!selectedSessionId.value) {
    return false;
  }
  const items = groupDrafts.value.map((item, index) => ({
    objectType: item.objectType,
    groupCode: item.groupCode.trim(),
    groupName: item.groupName.trim(),
    sortOrder: index + 1,
  }));
  if (items.some((item) => !item.groupCode || !item.groupName)) {
    ElMessage.warning("对象分组编码和名称不能为空");
    return false;
  }
  savingGroups.value = true;
  try {
    await updateAssessmentObjectGroups(selectedSessionId.value, { items });
    ElMessage.success("对象分组已保存");
    await contextStore.refreshCurrentDetail();
    await reloadCurrent();
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : "淇濆瓨瀵硅薄鍒嗙粍澶辫触";
    ElMessage.error(message);
    return false;
  } finally {
    savingGroups.value = false;
  }
}

async function removeObject(index: number): Promise<void> {
  const object = objectDrafts.value[index];
  if (!object) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除对象「${object.objectName}」吗？`, "删除确认", {
      type: "warning",
      confirmButtonText: "鍒犻櫎",
      cancelButtonText: "鍙栨秷",
    });
    objectDrafts.value.splice(index, 1);
  } catch (error) {
    if (isDialogCancel(error)) {
      return;
    }
    ElMessage.error("鍒犻櫎瀵硅薄澶辫触");
  }
}

async function saveObjects(): Promise<boolean> {
  if (!canEditCurrentSession.value) {
    return false;
  }
  if (!selectedSessionId.value) {
    return false;
  }
  const items = objectDrafts.value.map((item, index) => ({
    objectType: item.objectType,
    groupCode: item.groupCode,
    targetType: item.targetType,
    targetId: item.targetId,
    sortOrder: index + 1,
    isActive: true,
  }));
  savingObjects.value = true;
  try {
    objects.value = await updateAssessmentObjects(selectedSessionId.value, { items });
    objectDrafts.value = objects.value.map((item) => ({ ...item }));
    resetObjectBaseline();
    ElMessage.success("考核对象已保存");
    return true;
  } catch (error) {
    const message = error instanceof Error ? error.message : "淇濆瓨鑰冩牳瀵硅薄澶辫触";
    ElMessage.error(message);
    return false;
  } finally {
    savingObjects.value = false;
  }
}

function pickCandidate(item: AssessmentObjectCandidateItem): void {
  selectedCandidateKey.value = candidateKey(item);
}

function syncDialogGroupCode(): void {
  if (!selectedCandidate.value) {
    objectDialog.groupCode = "";
    return;
  }
  const options = candidateGroupOptions.value;
  if (options.length === 0) {
    objectDialog.groupCode = "";
    return;
  }
  if (options.some((item) => item.groupCode === objectDialog.groupCode)) {
    return;
  }
  const recommended = selectedCandidate.value.recommendedGroupCode;
  objectDialog.groupCode = options.some((item) => item.groupCode === recommended)
    ? recommended
    : options[0].groupCode;
}

async function openObjectDialog(): Promise<void> {
  if (!canEditCurrentSession.value) {
    return;
  }
  if (!selectedSessionId.value) {
    ElMessage.warning("璇峰厛閫夋嫨鑰冩牳鍦烘");
    return;
  }
  loadingCandidates.value = true;
  try {
    candidates.value = await listAssessmentObjectCandidates(selectedSessionId.value);
    candidateFilter.targetType = "";
    candidateFilter.keyword = "";
    selectedCandidateKey.value = filteredCandidates.value[0] ? candidateKey(filteredCandidates.value[0]) : "";
    syncDialogGroupCode();
    objectDialogVisible.value = true;
  } catch (error) {
    const message = error instanceof Error ? error.message : "加载考核对象候选失败";
    ElMessage.error(message);
  } finally {
    loadingCandidates.value = false;
  }
}

function appendObjectFromCandidate(): void {
  if (!selectedCandidate.value) {
    ElMessage.warning("请选择候选对象");
    return;
  }
  if (!objectDialog.groupCode) {
    ElMessage.warning("璇烽€夋嫨瀵硅薄鍒嗙粍");
    return;
  }
  const key = candidateKey(selectedCandidate.value);
  if (existingObjectKeySet.value.has(key)) {
    ElMessage.warning("该对象已在当前考核对象列表中");
    return;
  }
  objectDrafts.value.push({
    id: -Date.now(),
    assessmentId: selectedSessionId.value || 0,
    objectType: selectedCandidate.value.recommendedObjectType,
    groupCode: objectDialog.groupCode,
    targetType: selectedCandidate.value.targetType,
    targetId: selectedCandidate.value.targetId,
    objectName: selectedCandidate.value.objectName,
    sortOrder: objectDrafts.value.length + 1,
    isActive: true,
    createdAt: Date.now(),
    updatedAt: Date.now(),
  });
  objectDialogVisible.value = false;
}

async function resetObjects(): Promise<void> {
  if (!canEditCurrentSession.value) {
    return;
  }
  if (!selectedSessionId.value) {
    return;
  }
  resettingObjects.value = true;
  try {
    objects.value = await resetAssessmentSessionObjects(selectedSessionId.value);
    objectDrafts.value = objects.value.map((item) => ({ ...item }));
    resetObjectBaseline();
    ElMessage.success("宸查噸缃负榛樿瀵硅薄");
  } catch (error) {
    const message = error instanceof Error ? error.message : "閲嶇疆瀵硅薄澶辫触";
    ElMessage.error(message);
  } finally {
    resettingObjects.value = false;
  }
}

watch(
  () => [createVisible.value, createForm.year, createForm.organizationId],
  ([visible]) => {
    if (!visible || createNameTouched.value) {
      return;
    }
    createForm.displayName = buildDefaultDisplayName(createForm.year, createForm.organizationId);
  },
);

watch(
  () => [selectedCandidateKey.value, groupDrafts.value.length],
  () => {
    syncDialogGroupCode();
  },
);

watch(
  () => [selectedDetail.value, periodDrafts.value, ruleBindingGroups.value],
  () => {
    syncPeriodDirty();
  },
  { deep: true },
);

watch(
  () => [selectedDetail.value, groupDrafts.value],
  () => {
    syncGroupDirty();
  },
  { deep: true },
);

watch(
  () => [selectedDetail.value, objectDrafts.value],
  () => {
    syncObjectDirty();
  },
  { deep: true },
);

onMounted(async () => {
  window.addEventListener("keydown", handleAssessmentViewKeydown);
  unsavedStore.setSourceMeta(periodDirtySourceId, {
    label: "鑰冩牳绠＄悊-鍛ㄦ湡閰嶇疆",
    save: savePeriods,
  });
  unsavedStore.setSourceMeta(groupDirtySourceId, {
    label: "鑰冩牳绠＄悊-瀵硅薄鍒嗙粍",
    save: saveGroups,
  });
  unsavedStore.setSourceMeta(objectDirtySourceId, {
    label: "鑰冩牳绠＄悊-鑰冩牳瀵硅薄",
    save: saveObjects,
  });

  await Promise.all([loadOrganizations(), loadSessions()]);
  const preferredSessionId = contextStore.sessionId;
  if (
    preferredSessionId
    && preferredSessionId !== selectedSessionId.value
    && hasSessionInCurrentList(preferredSessionId)
  ) {
    await selectSession(preferredSessionId);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleAssessmentViewKeydown);
  unsavedStore.unregisterSource(periodDirtySourceId);
  unsavedStore.unregisterSource(groupDirtySourceId);
  unsavedStore.unregisterSource(objectDirtySourceId);
});
</script>

<style scoped>
.assessment-view {
  display: flex;
  flex-direction: column;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.tool-row {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}

.section {
  margin-top: 16px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.section-foot {
  margin-top: 10px;
}

.binding-section {
  margin-top: 12px;
}

.shared-rules-toggle {
  margin-top: 12px;
}

.binding-group-row {
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.binding-group-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 13px;
  color: #606266;
}

.period-hint {
  margin-top: 8px;
  color: #909399;
  font-size: 13px;
}

.period-index-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 24px;
  border-radius: 4px;
}

.period-index-tag.is-shared {
  font-weight: 600;
}

.session-status-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.session-status-select {
  width: 120px;
}
</style>


