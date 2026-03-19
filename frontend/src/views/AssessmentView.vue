<template>
  <div class="assessment-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>考核场次</strong>
          <div class="header-actions">
            <el-button :loading="loadingSessions" @click="loadSessions">刷新</el-button>
            <el-button type="primary" :disabled="!canEdit" @click="openCreateDialog">创建考核场次</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingSessions" :data="sessions" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="displayName" label="场次名称" min-width="260" />
        <el-table-column prop="year" label="年度" width="100" />
        <el-table-column prop="organizationName" label="组织" min-width="180" />
        <el-table-column prop="assessmentName" label="目录名" min-width="220" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="selectSession(row.id)">管理</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <strong>考核管理</strong>
          <div class="header-actions">
            <el-button :disabled="!selectedDetail" :loading="loadingDetail" @click="reloadCurrent">刷新</el-button>
          </div>
        </div>
      </template>

      <el-empty v-if="!selectedDetail" description="请选择一个考核场次进行管理" />

      <template v-else>
        <el-descriptions :column="3" border class="mb-12">
          <el-descriptions-item label="场次">{{ selectedDetail.session.displayName }}</el-descriptions-item>
          <el-descriptions-item label="组织">{{ selectedDetail.session.organizationName }}</el-descriptions-item>
          <el-descriptions-item label="数据目录">{{ selectedDetail.session.dataDir }}</el-descriptions-item>
        </el-descriptions>

        <div class="section">
          <div class="section-head">
            <strong>周期配置</strong>
            <el-button size="small" :disabled="!canEdit" @click="addPeriod">新增周期</el-button>
          </div>
          <el-table :data="periodDrafts" border>
            <el-table-column type="index" label="#" width="60" />
            <el-table-column label="编码" width="160">
              <template #default="{ row }">
                <el-input v-model="row.periodCode" @blur="onPeriodCodeBlur(row)" />
              </template>
            </el-table-column>
            <el-table-column label="名称" min-width="180">
              <template #default="{ row }">
                <el-input v-model="row.periodName" />
              </template>
            </el-table-column>
            <el-table-column label="规则绑定" min-width="220">
              <template #default="{ row }">
                <el-select
                  v-model="row.ruleBindingKey"
                  style="width: 100%"
                  filterable
                  placeholder="默认绑定自己"
                  @change="onRuleBindingKeyChange(row)"
                >
                  <el-option
                    v-for="code in periodCodeOptions"
                    :key="`binding_${code}`"
                    :label="periodCodeLabelMap[code] || code"
                    :value="code"
                  />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="180">
              <template #default="{ $index }">
                <el-button link :disabled="!canEdit || $index === 0" @click="bindPeriodToPrevious($index)">绑定上一周期</el-button>
                <el-button link type="danger" :disabled="!canEdit" @click="removePeriod($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="period-hint">同一绑定键下的周期会共用同一套规则配置，未设置时默认绑定自身。</div>
          <div class="section-foot">
            <el-button type="primary" :disabled="!canEdit || savingPeriods" :loading="savingPeriods" @click="savePeriods">
              保存周期
            </el-button>
          </div>
        </div>

        <div class="section">
          <div class="section-head">
            <strong>考核对象分组</strong>
            <el-button size="small" :disabled="!canEdit" @click="addGroup">新增分组</el-button>
          </div>
          <el-table :data="groupDrafts" border>
            <el-table-column type="index" label="#" width="60" />
            <el-table-column label="类型" width="120">
              <template #default="{ row }">
                <el-select v-model="row.objectType">
                  <el-option label="团体" value="team" />
                  <el-option label="个人" value="individual" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="编码" width="180">
              <template #default="{ row }">
                <el-input v-model="row.groupCode" />
              </template>
            </el-table-column>
            <el-table-column label="名称" min-width="180">
              <template #default="{ row }">
                <el-input v-model="row.groupName" />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100">
              <template #default="{ $index }">
                <el-button link type="danger" :disabled="!canEdit" @click="removeGroup($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
          <div class="section-foot">
            <el-button type="primary" :disabled="!canEdit || savingGroups" :loading="savingGroups" @click="saveGroups">
              保存分组
            </el-button>
          </div>
        </div>

        <div class="section">
          <div class="section-head">
            <strong>考核对象（默认来自组织架构）</strong>
            <div class="header-actions">
              <el-button size="small" :disabled="!canEdit" @click="openObjectDialog">新增对象</el-button>
              <el-button
                size="small"
                type="primary"
                :disabled="!canEdit || savingObjects"
                :loading="savingObjects"
                @click="saveObjects"
              >
                保存对象
              </el-button>
              <el-button
                size="small"
                :disabled="!canEdit || resettingObjects"
                :loading="resettingObjects"
                @click="resetObjects"
              >
                重置为默认
              </el-button>
            </div>
          </div>
          <el-table v-loading="loadingObjects" :data="objectDrafts" border>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                {{ row.objectType === "team" ? "团体" : "个人" }}
              </template>
            </el-table-column>
            <el-table-column prop="groupCode" label="分组编码" width="180" />
            <el-table-column label="分组名称" width="180">
              <template #default="{ row }">
                {{ groupNameByCode[row.groupCode] || row.groupCode }}
              </template>
            </el-table-column>
            <el-table-column label="来源类型" width="140">
              <template #default="{ row }">
                {{ row.targetType === "department" ? "部门" : row.targetType === "organization" ? "组织" : "人员" }}
              </template>
            </el-table-column>
            <el-table-column prop="objectName" label="对象名称" min-width="220" />
            <el-table-column prop="targetId" label="来源ID" width="100" />
            <el-table-column label="操作" width="100">
              <template #default="{ $index }">
                <el-button link type="danger" :disabled="!canEdit" @click="removeObject($index)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </template>
    </el-card>

    <el-dialog v-model="createVisible" title="创建考核场次" width="620px">
      <el-form label-width="110px">
        <el-form-item label="组织" required>
          <el-select v-model="createForm.organizationId" filterable style="width: 100%">
            <el-option
              v-for="item in organizations"
              :key="item.id"
              :label="item.orgName"
              :value="item.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="年度" required>
          <el-input-number v-model="createForm.year" :min="2000" :max="9999" />
        </el-form-item>
        <el-form-item label="场次名称">
          <el-input
            v-model="createForm.displayName"
            placeholder="默认：当前年份+组织名+考核"
            @input="markCreateNameTouched"
          />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="createForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createSession">创建</el-button>
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
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
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
} from "@/api/assessment";
import { listOrganizations } from "@/api/org";
import type {
  AssessmentObjectCandidateItem,
  AssessmentObjectGroupItem,
  AssessmentSessionDetail,
  AssessmentSessionObjectItem,
  AssessmentSessionPeriodItem,
} from "@/types/assessment";
import type { OrganizationItem } from "@/types/org";

const appStore = useAppStore();
const contextStore = useContextStore();
const canEdit = computed(() => appStore.hasPermission("assessment:update"));

const sessions = ref<AssessmentSessionDetail["session"][]>([]);
const selectedSessionId = ref<number | undefined>();
const selectedDetail = ref<AssessmentSessionDetail | null>(null);

const loadingSessions = ref(false);
const loadingDetail = ref(false);
const loadingObjects = ref(false);

const periodDrafts = ref<Array<{ periodCode: string; periodName: string; ruleBindingKey: string }>>([]);
const groupDrafts = ref<Array<{ objectType: "team" | "individual"; groupCode: string; groupName: string }>>([]);
const objects = ref<AssessmentSessionObjectItem[]>([]);
const objectDrafts = ref<AssessmentSessionObjectItem[]>([]);

const savingPeriods = ref(false);
const savingGroups = ref(false);
const savingObjects = ref(false);
const resettingObjects = ref(false);

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

function addPeriod(): void {
  periodDrafts.value.push({ periodCode: "", periodName: "", ruleBindingKey: "" });
}

function removePeriod(index: number): void {
  periodDrafts.value.splice(index, 1);
  ensurePeriodBindingKeys();
}

function onPeriodCodeBlur(row: { periodCode: string; ruleBindingKey: string }): void {
  row.periodCode = row.periodCode.trim().toUpperCase();
  if (!row.ruleBindingKey.trim()) {
    row.ruleBindingKey = row.periodCode;
  }
  ensurePeriodBindingKeys();
}

function onRuleBindingKeyChange(row: { ruleBindingKey: string }): void {
  row.ruleBindingKey = row.ruleBindingKey.trim().toUpperCase();
  ensurePeriodBindingKeys();
}

function bindPeriodToPrevious(index: number): void {
  if (index <= 0 || index >= periodDrafts.value.length) {
    return;
  }
  const previousCode = periodDrafts.value[index - 1].periodCode.trim().toUpperCase();
  if (!previousCode) {
    ElMessage.warning("请先设置上一行周期编码");
    return;
  }
  periodDrafts.value[index].ruleBindingKey = previousCode;
  ensurePeriodBindingKeys();
}

function ensurePeriodBindingKeys(): void {
  const available = new Set(periodCodeOptions.value);
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

function removeGroup(index: number): void {
  groupDrafts.value.splice(index, 1);
}

function candidateKey(item: Pick<AssessmentObjectCandidateItem, "targetType" | "targetId">): string {
  return `${item.targetType}:${item.targetId}`;
}

function buildDefaultDisplayName(year: number, organizationId?: number): string {
  if (!organizationId) {
    return `${year}年考核`;
  }
  const organization = organizations.value.find((item) => item.id === organizationId);
  if (!organization) {
    return `${year}年考核`;
  }
  return `${year}年${organization.orgName}考核`;
}

function markCreateNameTouched(): void {
  createNameTouched.value = true;
}

async function loadOrganizations(): Promise<void> {
  organizations.value = await listOrganizations({ status: "active" });
}

async function loadSessions(): Promise<void> {
  loadingSessions.value = true;
  try {
    sessions.value = await listAssessmentSessions();
    if (!selectedSessionId.value && sessions.value.length > 0) {
      await selectSession(sessions.value[0].id);
    }
  } catch (_error) {
    ElMessage.error("加载考核场次失败");
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
    ensurePeriodBindingKeys();
    groupDrafts.value = detail.objectGroups.map((item) => ({
      objectType: item.objectType,
      groupCode: item.groupCode,
      groupName: item.groupName,
    }));
    await loadObjects(sessionId);
    if (contextStore.sessionId !== sessionId) {
      await contextStore.setSession(sessionId);
    }
  } catch (_error) {
    ElMessage.error("加载场次详情失败");
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
    ElMessage.warning("请选择组织");
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
    ElMessage.success("考核场次创建成功");
    createVisible.value = false;
    await contextStore.refreshSessions();
    await loadSessions();
    await selectSession(detail.session.id);
  } catch (error) {
    const message = error instanceof Error ? error.message : "创建考核场次失败";
    ElMessage.error(message);
  } finally {
    creating.value = false;
  }
}

async function savePeriods(): Promise<void> {
  if (!selectedSessionId.value) {
    return;
  }
  const items = periodDrafts.value.map((item, index) => ({
    periodCode: item.periodCode.trim().toUpperCase(),
    periodName: item.periodName.trim(),
    ruleBindingKey: item.ruleBindingKey.trim().toUpperCase(),
    sortOrder: index + 1,
  }));
  if (items.some((item) => !item.periodCode || !item.periodName)) {
    ElMessage.warning("周期编码和名称不能为空");
    return;
  }
  const codeSet = new Set(items.map((item) => item.periodCode));
  for (const item of items) {
    if (!item.ruleBindingKey) {
      item.ruleBindingKey = item.periodCode;
    }
    if (!codeSet.has(item.ruleBindingKey)) {
      ElMessage.warning(`规则绑定周期「${item.ruleBindingKey}」不存在，请检查周期配置`);
      return;
    }
  }
  savingPeriods.value = true;
  try {
    await updateAssessmentPeriods(selectedSessionId.value, { items });
    ElMessage.success("周期已保存");
    await contextStore.refreshCurrentDetail();
    await reloadCurrent();
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存周期失败";
    ElMessage.error(message);
  } finally {
    savingPeriods.value = false;
  }
}

async function saveGroups(): Promise<void> {
  if (!selectedSessionId.value) {
    return;
  }
  const items = groupDrafts.value.map((item, index) => ({
    objectType: item.objectType,
    groupCode: item.groupCode.trim(),
    groupName: item.groupName.trim(),
    sortOrder: index + 1,
  }));
  if (items.some((item) => !item.groupCode || !item.groupName)) {
    ElMessage.warning("对象分组编码和名称不能为空");
    return;
  }
  savingGroups.value = true;
  try {
    await updateAssessmentObjectGroups(selectedSessionId.value, { items });
    ElMessage.success("对象分组已保存");
    await contextStore.refreshCurrentDetail();
    await reloadCurrent();
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存对象分组失败";
    ElMessage.error(message);
  } finally {
    savingGroups.value = false;
  }
}

function removeObject(index: number): void {
  objectDrafts.value.splice(index, 1);
}

async function saveObjects(): Promise<void> {
  if (!selectedSessionId.value) {
    return;
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
    ElMessage.success("考核对象已保存");
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存考核对象失败";
    ElMessage.error(message);
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
  if (!selectedSessionId.value) {
    ElMessage.warning("请先选择考核场次");
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
    ElMessage.warning("请选择对象分组");
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
  if (!selectedSessionId.value) {
    return;
  }
  resettingObjects.value = true;
  try {
    objects.value = await resetAssessmentSessionObjects(selectedSessionId.value);
    objectDrafts.value = objects.value.map((item) => ({ ...item }));
    ElMessage.success("已重置为默认对象");
  } catch (error) {
    const message = error instanceof Error ? error.message : "重置对象失败";
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

onMounted(async () => {
  await Promise.all([loadOrganizations(), loadSessions()]);
  if (contextStore.sessionId) {
    await selectSession(contextStore.sessionId);
  }
});
</script>

<style scoped>
.assessment-view {
  display: grid;
  gap: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.header-actions {
  display: flex;
  gap: 8px;
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

.period-hint {
  margin-top: 8px;
  color: #909399;
  font-size: 13px;
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
