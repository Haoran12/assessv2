<template>
  <div class="users-page">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>用户管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingUsers" @click="handleRefresh">刷新</el-button>
            <el-button v-if="isRoot" type="primary" @click="openCreateUserDialog">新增用户</el-button>
          </div>
        </div>
      </template>

      <div class="toolbar">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="按用户名或姓名搜索"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />
        <el-select
          v-model="query.status"
          clearable
          placeholder="账号状态"
          @change="handleSearch"
        >
          <el-option label="正常" value="active" />
          <el-option label="停用" value="inactive" />
          <el-option label="锁定" value="locked" />
        </el-select>
        <el-button type="primary" @click="handleSearch">查询</el-button>
      </div>

      <el-table v-loading="loadingUsers" :data="rows" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="realName" label="姓名" min-width="140" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="用户组" min-width="260">
          <template #default="{ row }">
            <div v-if="displayRoleNames(row).length > 0" class="group-tags">
              <el-tag
                v-for="name in displayRoleNames(row)"
                :key="`${row.id}-${name}`"
                size="small"
                effect="plain"
              >
                {{ name }}
              </el-tag>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="主角色" width="150">
          <template #default="{ row }">
            <el-tag>{{ roleDisplayName(row.primaryRole, row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="强制改密" width="110">
          <template #default="{ row }">
            <el-tag :type="row.mustChangePassword ? 'warning' : 'success'">
              {{ row.mustChangePassword ? "是" : "否" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近登录" min-width="170">
          <template #default="{ row }">
            {{ formatTimestamp(row.lastLoginAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="320" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button
                v-if="isRoot"
                size="small"
                :disabled="loadingObjectLinks || savingObjectLinks"
                @click="openObjectLinkDialog(row)"
              >
                对象绑定
              </el-button>
              <el-button
                v-if="isRoot"
                size="small"
                type="primary"
                plain
                :disabled="loadingGroups"
                @click="openEditUserDialog(row)"
              >
                修改
              </el-button>
              <el-button
                v-if="isRoot"
                size="small"
                type="danger"
                plain
                :disabled="!canDeleteUser(row)"
                @click="handleDeleteUser(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="total, prev, pager, next, sizes"
          :total="total"
          :page-size="query.pageSize"
          :current-page="query.page"
          :page-sizes="[10, 20, 50, 100]"
          @current-change="handlePageChange"
          @size-change="handlePageSizeChange"
        />
      </div>
    </el-card>

    <el-card v-if="isRoot">
      <template #header>
        <div class="header-row">
          <strong>用户组管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingGroups" @click="loadUserGroups">刷新</el-button>
            <el-button type="primary" @click="openGroupForm()">新增用户组</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingGroups" :data="userGroups" border>
        <el-table-column prop="id" label="编号" width="70" />
        <el-table-column prop="roleName" label="组名" min-width="160" />
        <el-table-column prop="roleCode" label="编码" min-width="160" />
        <el-table-column prop="description" label="描述" min-width="220">
          <template #default="{ row }">
            {{ row.description || "-" }}
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag :type="row.isSystem ? 'info' : 'success'">
              {{ row.isSystem ? "系统" : "自定义" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="200" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" :disabled="row.isSystem" @click="openGroupForm(row)">编辑</el-button>
              <el-button
                size="small"
                type="danger"
                plain
                :disabled="row.isSystem"
                @click="handleDeleteGroup(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      v-model="userFormVisible"
      width="560px"
      :title="userFormMode === 'create' ? '新增用户' : `修改用户 - ${editingUserRow?.username ?? ''}`"
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item label="用户名" required>
          <el-input
            v-model="userForm.username"
            maxlength="50"
            :disabled="isEditingRootUser"
            placeholder="3-50位，字母开头，可含 . _ -"
          />
        </el-form-item>
        <el-form-item label="姓名" required>
          <el-input v-model="userForm.realName" maxlength="100" />
        </el-form-item>
        <el-form-item :label="userFormMode === 'create' ? '初始密码' : '新密码'">
          <el-input
            v-model="userForm.password"
            type="password"
            show-password
            :placeholder="userFormMode === 'create' ? '留空使用系统默认密码' : '留空不修改密码'"
          />
        </el-form-item>
        <el-form-item label="账号状态" required>
          <el-select v-model="userForm.status" style="width: 100%">
            <el-option label="正常" value="active" />
            <el-option label="停用" value="inactive" />
            <el-option label="锁定" value="locked" />
          </el-select>
        </el-form-item>
        <el-form-item label="强制改密">
          <el-switch v-model="userForm.mustChangePassword" />
        </el-form-item>
        <el-form-item label="用户组" required>
          <el-checkbox-group v-model="userForm.roleIds" class="role-checkbox-group">
            <el-checkbox v-for="group in userGroups" :key="group.id" :label="group.id" border>
              {{ group.roleName }} ({{ group.roleCode }})
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="主角色" required>
          <el-radio-group v-model="userForm.primaryRoleId">
            <el-radio v-for="roleId in userForm.roleIds" :key="`edit-primary-${roleId}`" :label="roleId">
              {{ userGroupName(roleId) }}
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="userFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingUser" @click="handleSubmitUserForm">保存</el-button>
      </template>
    </el-dialog>
    <el-dialog
      v-model="groupFormVisible"
      width="540px"
      :title="editingGroupID ? '编辑用户组' : '新增用户组'"
      destroy-on-close
    >
      <el-form label-width="88px">
        <el-form-item label="组名" required>
          <el-input v-model="groupForm.roleName" maxlength="100" />
        </el-form-item>
        <el-form-item label="编码">
          <el-input
            v-model="groupForm.roleCode"
            maxlength="50"
            placeholder="留空自动生成，例如 ops-team"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="groupForm.description"
            type="textarea"
            :rows="3"
            maxlength="300"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingGroup" @click="handleSaveGroup">保存</el-button>
      </template>
    </el-dialog>
    <el-dialog
      v-model="objectLinkVisible"
      width="1080px"
      :title="objectLinkTargetUser ? `对象绑定 - ${objectLinkTargetUser.username}` : '对象绑定'"
      destroy-on-close
    >
      <div class="object-link-editor">
        <div class="object-link-create-row">
          <el-select
            v-model="objectLinkDraft.yearId"
            clearable
            filterable
            placeholder="选择年度"
            :loading="loadingAssessmentYears"
            style="width: 180px"
            @change="handleObjectLinkYearChange"
          >
            <el-option
              v-for="year in sortedAssessmentYears"
              :key="year.id"
              :label="`${year.year}年度`"
              :value="year.id"
            />
          </el-select>
          <el-select
            v-model="objectLinkDraft.objectId"
            clearable
            filterable
            placeholder="选择考核对象"
            :loading="loadingAssessmentObjects"
            style="min-width: 320px"
          >
            <el-option
              v-for="item in draftAssessmentObjectOptions"
              :key="item.id"
              :label="`${item.objectName} (#${item.id})`"
              :value="item.id"
            />
          </el-select>
          <el-select
            v-model="objectLinkDraft.linkType"
            filterable
            placeholder="关系类型"
            style="width: 230px"
            :loading="loadingObjectLinkTypeOptions"
          >
            <el-option
              v-for="item in objectLinkTypeSelectOptions"
              :key="`draft-link-type-${item}`"
              :label="item"
              :value="item"
            />
          </el-select>
          <el-select v-model="objectLinkDraft.accessLevel" style="width: 140px">
            <el-option label="只读" value="read" />
            <el-option label="详情" value="detail" />
          </el-select>
          <el-button :loading="loadingObjectLinkTypeOptions" @click="openObjectLinkTypeConfigDialog">
            配置关系类型
          </el-button>
          <el-button type="primary" @click="handleAddObjectLinkRow">添加绑定</el-button>
        </div>

        <el-table
          v-loading="loadingObjectLinks"
          :data="objectLinkRows"
          border
          class="object-link-table"
          max-height="460"
        >
          <el-table-column label="年度" width="90">
            <template #default="{ row }">
              {{ assessmentYearTextByID(row.assessmentObjectYear) }}
            </template>
          </el-table-column>
          <el-table-column label="考核对象" min-width="260">
            <template #default="{ row }">
              <div class="object-name">{{ row.assessmentObjectName }}</div>
              <div class="object-meta-row">
                <el-tag size="small" effect="plain">{{ row.assessmentObjectType }}</el-tag>
                <el-tag size="small" effect="plain">{{ row.assessmentObjectCategory }}</el-tag>
                <el-tag size="small" :type="row.assessmentObjectActive ? 'success' : 'info'">
                  {{ row.assessmentObjectActive ? "启用" : "停用" }}
                </el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="关系类型" width="190">
            <template #default="{ row }">
              <el-select
                v-model="row.linkType"
                filterable
                :loading="loadingObjectLinkTypeOptions"
              >
                <el-option
                  v-for="item in objectLinkTypeSelectOptions"
                  :key="`table-link-type-${row.assessmentObjectId}-${item}`"
                  :label="item"
                  :value="item"
                />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="访问级别" width="120">
            <template #default="{ row }">
              <el-select v-model="row.accessLevel">
                <el-option label="只读" value="read" />
                <el-option label="详情" value="detail" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="主绑定" width="90">
            <template #default="{ row }">
              <el-switch v-model="row.isPrimary" />
            </template>
          </el-table-column>
          <el-table-column label="启用" width="90">
            <template #default="{ row }">
              <el-switch v-model="row.isActive" />
            </template>
          </el-table-column>
          <el-table-column label="生效开始" min-width="180">
            <template #default="{ row }">
              <el-date-picker
                v-model="row.effectiveFrom"
                type="datetime"
                clearable
                placeholder="开始时间"
              />
            </template>
          </el-table-column>
          <el-table-column label="生效结束" min-width="180">
            <template #default="{ row }">
              <el-date-picker
                v-model="row.effectiveTo"
                type="datetime"
                clearable
                placeholder="结束时间"
              />
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ $index }">
              <el-button type="danger" plain size="small" @click="handleRemoveObjectLinkRow($index)">
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <div class="empty-tip" v-if="!loadingObjectLinks && objectLinkRows.length === 0">
          当前用户还没有任何考核对象绑定，先在上方选择对象并添加。
        </div>
      </div>
      <template #footer>
        <el-button @click="objectLinkVisible = false">关闭</el-button>
        <el-button type="primary" :loading="savingObjectLinks" @click="handleSaveObjectLinks">
          保存绑定
        </el-button>
      </template>
    </el-dialog>
    <el-dialog
      v-model="objectLinkTypeConfigVisible"
      width="560px"
      title="配置关系类型"
      destroy-on-close
    >
      <div class="object-link-type-config">
        <div class="object-link-type-create-row">
          <el-input
            v-model="objectLinkTypeDraftInput"
            maxlength="30"
            placeholder="输入新的关系类型（如 owner）"
            @keyup.enter="handleAddObjectLinkTypeConfigRow"
          />
          <el-button type="primary" @click="handleAddObjectLinkTypeConfigRow">添加</el-button>
        </div>
        <div class="object-link-type-list">
          <el-tag
            v-for="(item, index) in objectLinkTypeConfigRows"
            :key="`object-link-type-config-${item}`"
            closable
            @close="handleRemoveObjectLinkTypeConfigRow(index)"
          >
            {{ item }}
          </el-tag>
        </div>
        <div class="empty-tip" v-if="objectLinkTypeConfigRows.length === 0">
          暂无关系类型，请先添加至少一项。
        </div>
      </div>
      <template #footer>
        <el-button @click="objectLinkTypeConfigVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="savingObjectLinkTypeOptions"
          @click="handleSaveObjectLinkTypeOptions"
        >
          保存配置
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { listAssessmentObjects, listAssessmentYears } from "@/api/assessment";
import { http } from "@/api/http";
import {
  getSystemSettings,
  listUserObjectLinks,
  replaceUserObjectLinks,
  updateSystemSettings,
} from "@/api/system-admin";
import { useAppStore } from "@/stores/app";
import { useUnsavedStore } from "@/stores/unsaved";
import type { AssessmentObjectItem, AssessmentYearItem } from "@/types/assessment";
import type {
  ReplaceUserObjectLinkItem,
  SystemSettingsResponse,
  UserGroupItem,
  UserGroupListResponse,
  UserObjectLinkAccessLevel,
  UserObjectLinkItem,
  UserListItem,
  UserListResponse,
  UserStatus,
} from "@/types/system";

type UserFormMode = "create" | "edit";
const objectLinkTypeSettingKey = "assessment.object_link_types";
const defaultObjectLinkTypes = ["member", "owner", "evaluator", "observer"];

interface EditableUserObjectLinkRow {
  id?: number;
  assessmentObjectId: number;
  assessmentObjectName: string;
  assessmentObjectYear: number;
  assessmentObjectType: string;
  assessmentObjectCategory: string;
  assessmentObjectActive: boolean;
  linkType: string;
  accessLevel: UserObjectLinkAccessLevel;
  isPrimary: boolean;
  isActive: boolean;
  effectiveFrom: Date | null;
  effectiveTo: Date | null;
}

const appStore = useAppStore();
const unsavedStore = useUnsavedStore();
const groupFormDirtySourceId = "system-users:group-form";

const loadingUsers = ref(false);
const loadingGroups = ref(false);
const savingGroup = ref(false);
const savingUser = ref(false);
const loadingObjectLinks = ref(false);
const savingObjectLinks = ref(false);
const loadingAssessmentYears = ref(false);
const loadingAssessmentObjects = ref(false);
const loadingObjectLinkTypeOptions = ref(false);
const savingObjectLinkTypeOptions = ref(false);

const rows = ref<UserListItem[]>([]);
const total = ref(0);
const userGroups = ref<UserGroupItem[]>([]);
const assessmentYears = ref<AssessmentYearItem[]>([]);
const assessmentObjectsByYear = ref<Record<number, AssessmentObjectItem[]>>({});
const objectLinkRows = ref<EditableUserObjectLinkRow[]>([]);
const objectLinkTypeOptions = ref<string[]>([]);

const query = reactive({
  page: 1,
  pageSize: 20,
  keyword: "",
  status: "" as UserStatus | "",
});

const isRoot = computed(() => appStore.roles.includes("root") || appStore.primaryRole === "root");
const currentUserID = computed(() => appStore.currentUser?.id ?? 0);

const userFormVisible = ref(false);
const userFormMode = ref<UserFormMode>("create");
const editingUserRow = ref<UserListItem | null>(null);
const userForm = reactive({
  username: "",
  realName: "",
  password: "",
  status: "active" as UserStatus,
  mustChangePassword: true,
  roleIds: [] as number[],
  primaryRoleId: null as number | null,
});

const groupFormVisible = ref(false);
const editingGroupID = ref<number | null>(null);
const groupForm = reactive({
  roleName: "",
  roleCode: "",
  description: "",
});
const groupFormBaseline = ref("");
const objectLinkTypeMaxLength = 30;

const objectLinkVisible = ref(false);
const objectLinkTargetUser = ref<UserListItem | null>(null);
const objectLinkTypeConfigVisible = ref(false);
const objectLinkTypeConfigRows = ref<string[]>([]);
const objectLinkTypeDraftInput = ref("");
const objectLinkDraft = reactive({
  yearId: null as number | null,
  objectId: null as number | null,
  linkType: defaultObjectLinkTypes[0],
  accessLevel: "detail" as UserObjectLinkAccessLevel,
  isPrimary: false,
  isActive: true,
});

const isEditingRootUser = computed(
  () => userFormMode.value === "edit" && editingUserRow.value?.username === "root",
);

const sortedAssessmentYears = computed(() =>
  [...assessmentYears.value].sort((left, right) => right.year - left.year),
);

const draftAssessmentObjectOptions = computed(() => {
  if (!objectLinkDraft.yearId) {
    return [] as AssessmentObjectItem[];
  }
  return assessmentObjectsByYear.value[objectLinkDraft.yearId] ?? [];
});

const objectLinkTypeSelectOptions = computed(() => {
  const result = normalizeObjectLinkTypes(objectLinkTypeOptions.value);
  const seen = new Set(result);
  for (const item of objectLinkRows.value) {
    const normalized = normalizeObjectLinkType(item.linkType);
    if (!normalized || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  if (result.length === 0) {
    return [...defaultObjectLinkTypes];
  }
  return result;
});

function groupFormSignature(): string {
  return JSON.stringify({
    id: editingGroupID.value,
    roleName: groupForm.roleName,
    roleCode: groupForm.roleCode,
    description: groupForm.description,
  });
}

function resetGroupFormBaseline(): void {
  groupFormBaseline.value = groupFormSignature();
  unsavedStore.clearDirty(groupFormDirtySourceId);
}

function mapRoleIDsFromUser(row: UserListItem): number[] {
  const roleIDMap = new Map(userGroups.value.map((item) => [item.roleCode, item.id]));
  return row.roles
    .map((roleCode) => roleIDMap.get(roleCode))
    .filter((value): value is number => typeof value === "number");
}

function resetUserForm(): void {
  userForm.username = "";
  userForm.realName = "";
  userForm.password = "";
  userForm.status = "active";
  userForm.mustChangePassword = true;
  userForm.roleIds = [];
  userForm.primaryRoleId = null;
}

function userGroupName(roleId: number): string {
  return userGroups.value.find((item) => item.id === roleId)?.roleName ?? `角色#${roleId}`;
}

function resetObjectLinkDraft(): void {
  objectLinkDraft.objectId = null;
  objectLinkDraft.linkType = objectLinkTypeSelectOptions.value[0] ?? defaultObjectLinkTypes[0];
  objectLinkDraft.accessLevel = "detail";
  objectLinkDraft.isPrimary = false;
  objectLinkDraft.isActive = true;
}

function toDateOrNull(unixSeconds?: number): Date | null {
  if (!unixSeconds || unixSeconds <= 0) {
    return null;
  }
  return new Date(unixSeconds * 1000);
}

function toUnixSecondsOrUndefined(date: Date | null): number | undefined {
  if (!date) {
    return undefined;
  }
  return Math.floor(date.getTime() / 1000);
}

function normalizeObjectLinkType(linkType: string): string {
  return linkType.trim().toLowerCase();
}

function normalizeObjectLinkTypes(values: string[]): string[] {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const item of values) {
    const normalized = normalizeObjectLinkType(item);
    if (!normalized) {
      continue;
    }
    if (normalized.length > objectLinkTypeMaxLength) {
      continue;
    }
    if (seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  return result;
}

function parseObjectLinkTypesFromSettings(data: SystemSettingsResponse): string[] {
  const raw = data.assessment?.[objectLinkTypeSettingKey];
  if (!Array.isArray(raw)) {
    return [...defaultObjectLinkTypes];
  }
  const values = raw.filter((item): item is string => typeof item === "string");
  const normalized = normalizeObjectLinkTypes(values);
  if (normalized.length === 0) {
    return [...defaultObjectLinkTypes];
  }
  return normalized;
}

async function ensureObjectLinkTypeOptionsLoaded(force = false): Promise<void> {
  if (loadingObjectLinkTypeOptions.value) {
    return;
  }
  if (!force && objectLinkTypeOptions.value.length > 0) {
    return;
  }

  loadingObjectLinkTypeOptions.value = true;
  try {
    const settings = await getSystemSettings();
    objectLinkTypeOptions.value = parseObjectLinkTypesFromSettings(settings);
  } catch (_error) {
    if (objectLinkTypeOptions.value.length === 0) {
      objectLinkTypeOptions.value = [...defaultObjectLinkTypes];
    }
    ElMessage.error("关系类型配置加载失败，已使用默认项");
  } finally {
    loadingObjectLinkTypeOptions.value = false;
  }
}

function openObjectLinkTypeConfigDialog(): void {
  objectLinkTypeConfigRows.value = [...objectLinkTypeSelectOptions.value];
  objectLinkTypeDraftInput.value = "";
  objectLinkTypeConfigVisible.value = true;
}

function handleAddObjectLinkTypeConfigRow(): void {
  const normalized = normalizeObjectLinkType(objectLinkTypeDraftInput.value);
  if (!normalized) {
    ElMessage.warning("请输入关系类型");
    return;
  }
  if (normalized.length > objectLinkTypeMaxLength) {
    ElMessage.warning(`关系类型不能超过 ${objectLinkTypeMaxLength} 个字符`);
    return;
  }
  if (objectLinkTypeConfigRows.value.includes(normalized)) {
    ElMessage.warning("关系类型已存在");
    return;
  }
  objectLinkTypeConfigRows.value.push(normalized);
  objectLinkTypeDraftInput.value = "";
}

function handleRemoveObjectLinkTypeConfigRow(index: number): void {
  objectLinkTypeConfigRows.value.splice(index, 1);
}

async function handleSaveObjectLinkTypeOptions(): Promise<void> {
  const normalized = normalizeObjectLinkTypes(objectLinkTypeConfigRows.value);
  if (normalized.length === 0) {
    ElMessage.warning("至少保留一个关系类型");
    return;
  }

  savingObjectLinkTypeOptions.value = true;
  try {
    const result = await updateSystemSettings([
      {
        settingKey: objectLinkTypeSettingKey,
        settingValue: normalized,
      },
    ]);
    objectLinkTypeOptions.value = parseObjectLinkTypesFromSettings(result);
    objectLinkTypeConfigVisible.value = false;

    const available = new Set(objectLinkTypeSelectOptions.value);
    if (!available.has(objectLinkDraft.linkType)) {
      objectLinkDraft.linkType = objectLinkTypeSelectOptions.value[0] ?? defaultObjectLinkTypes[0];
    }

    ElMessage.success("关系类型配置已保存");
  } catch (_error) {
    ElMessage.error("关系类型配置保存失败");
  } finally {
    savingObjectLinkTypeOptions.value = false;
  }
}

function objectLinkKey(assessmentObjectId: number, linkType: string): string {
  return `${assessmentObjectId}:${normalizeObjectLinkType(linkType)}`;
}

function mapUserObjectLinkRow(item: UserObjectLinkItem): EditableUserObjectLinkRow {
  const normalizedLinkType = normalizeObjectLinkType(item.linkType);
  return {
    id: item.id,
    assessmentObjectId: item.assessmentObjectId,
    assessmentObjectName: item.assessmentObjectName,
    assessmentObjectYear: item.assessmentObjectYear,
    assessmentObjectType: item.assessmentObjectType,
    assessmentObjectCategory: item.assessmentObjectCategory,
    assessmentObjectActive: item.assessmentObjectActive,
    linkType: normalizedLinkType || defaultObjectLinkTypes[0],
    accessLevel: item.accessLevel === "read" ? "read" : "detail",
    isPrimary: item.isPrimary,
    isActive: item.isActive,
    effectiveFrom: toDateOrNull(item.effectiveFrom),
    effectiveTo: toDateOrNull(item.effectiveTo),
  };
}

function assessmentYearTextByID(yearId: number): string {
  return String(assessmentYears.value.find((item) => item.id === yearId)?.year ?? yearId);
}

async function ensureAssessmentYearsLoaded(): Promise<void> {
  if (loadingAssessmentYears.value || assessmentYears.value.length > 0) {
    return;
  }
  loadingAssessmentYears.value = true;
  try {
    assessmentYears.value = await listAssessmentYears();
  } catch (_error) {
    ElMessage.error("考核年度加载失败");
  } finally {
    loadingAssessmentYears.value = false;
  }
}

async function ensureAssessmentObjectsLoaded(yearId: number): Promise<void> {
  if (!yearId || loadingAssessmentObjects.value || assessmentObjectsByYear.value[yearId]) {
    return;
  }
  loadingAssessmentObjects.value = true;
  try {
    const objects = await listAssessmentObjects(yearId);
    assessmentObjectsByYear.value = {
      ...assessmentObjectsByYear.value,
      [yearId]: objects,
    };
  } catch (_error) {
    ElMessage.error("考核对象加载失败");
  } finally {
    loadingAssessmentObjects.value = false;
  }
}

async function loadUserObjectLinks(userId: number): Promise<void> {
  loadingObjectLinks.value = true;
  try {
    const links = await listUserObjectLinks(userId);
    objectLinkRows.value = links.map(mapUserObjectLinkRow);
  } catch (_error) {
    ElMessage.error("用户对象绑定加载失败");
  } finally {
    loadingObjectLinks.value = false;
  }
}

async function handleObjectLinkYearChange(value: number | null): Promise<void> {
  objectLinkDraft.objectId = null;
  if (!value) {
    return;
  }
  await ensureAssessmentObjectsLoaded(value);
}

async function openObjectLinkDialog(row: UserListItem): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  objectLinkTargetUser.value = row;
  objectLinkVisible.value = true;
  objectLinkRows.value = [];
  await ensureObjectLinkTypeOptionsLoaded();
  await ensureAssessmentYearsLoaded();
  if (!objectLinkDraft.yearId && sortedAssessmentYears.value.length > 0) {
    objectLinkDraft.yearId = sortedAssessmentYears.value[0].id;
  }
  if (!objectLinkTypeSelectOptions.value.includes(objectLinkDraft.linkType)) {
    objectLinkDraft.linkType = objectLinkTypeSelectOptions.value[0] ?? defaultObjectLinkTypes[0];
  }
  if (objectLinkDraft.yearId) {
    await ensureAssessmentObjectsLoaded(objectLinkDraft.yearId);
  }
  await loadUserObjectLinks(row.id);
}

function handleAddObjectLinkRow(): void {
  if (!objectLinkDraft.yearId) {
    ElMessage.warning("请选择年度");
    return;
  }
  if (!objectLinkDraft.objectId) {
    ElMessage.warning("请选择考核对象");
    return;
  }

  const linkType = normalizeObjectLinkType(objectLinkDraft.linkType);
  if (!linkType) {
    ElMessage.warning("请输入关系类型");
    return;
  }
  if (linkType.length > objectLinkTypeMaxLength) {
    ElMessage.warning(`关系类型不能超过 ${objectLinkTypeMaxLength} 个字符`);
    return;
  }

  const objectItem = draftAssessmentObjectOptions.value.find(
    (item) => item.id === objectLinkDraft.objectId,
  );
  if (!objectItem) {
    ElMessage.warning("所选考核对象无效，请重新选择");
    return;
  }

  const duplicate = objectLinkRows.value.some(
    (item) => objectLinkKey(item.assessmentObjectId, item.linkType) === objectLinkKey(objectItem.id, linkType),
  );
  if (duplicate) {
    ElMessage.warning("同一对象下关系类型不能重复");
    return;
  }

  objectLinkRows.value.push({
    assessmentObjectId: objectItem.id,
    assessmentObjectName: objectItem.objectName,
    assessmentObjectYear: objectItem.yearId,
    assessmentObjectType: objectItem.objectType,
    assessmentObjectCategory: objectItem.objectCategory,
    assessmentObjectActive: objectItem.isActive,
    linkType,
    accessLevel: objectLinkDraft.accessLevel,
    isPrimary: objectLinkDraft.isPrimary,
    isActive: objectLinkDraft.isActive,
    effectiveFrom: null,
    effectiveTo: null,
  });

  objectLinkDraft.objectId = null;
}

function handleRemoveObjectLinkRow(index: number): void {
  objectLinkRows.value.splice(index, 1);
}

function buildObjectLinkPayloadRows(): ReplaceUserObjectLinkItem[] | null {
  const payload: ReplaceUserObjectLinkItem[] = [];
  const keySet = new Set<string>();

  for (let index = 0; index < objectLinkRows.value.length; index += 1) {
    const item = objectLinkRows.value[index];
    const rowNo = index + 1;
    const linkType = normalizeObjectLinkType(item.linkType);
    if (!linkType) {
      ElMessage.warning(`第 ${rowNo} 行关系类型不能为空`);
      return null;
    }
    if (linkType.length > objectLinkTypeMaxLength) {
      ElMessage.warning(`第 ${rowNo} 行关系类型超过 ${objectLinkTypeMaxLength} 个字符`);
      return null;
    }
    if (item.accessLevel !== "read" && item.accessLevel !== "detail") {
      ElMessage.warning(`第 ${rowNo} 行访问级别无效`);
      return null;
    }
    if (item.effectiveFrom && item.effectiveTo && item.effectiveFrom.getTime() > item.effectiveTo.getTime()) {
      ElMessage.warning(`第 ${rowNo} 行生效时间范围不合法`);
      return null;
    }

    const uniqueKey = objectLinkKey(item.assessmentObjectId, linkType);
    if (keySet.has(uniqueKey)) {
      ElMessage.warning(`第 ${rowNo} 行与其他行重复（对象 + 关系类型）`);
      return null;
    }
    keySet.add(uniqueKey);

    payload.push({
      assessmentObjectId: item.assessmentObjectId,
      linkType,
      accessLevel: item.accessLevel,
      isPrimary: item.isPrimary,
      effectiveFrom: toUnixSecondsOrUndefined(item.effectiveFrom),
      effectiveTo: toUnixSecondsOrUndefined(item.effectiveTo),
      isActive: item.isActive,
    });
  }

  return payload;
}

async function handleSaveObjectLinks(): Promise<void> {
  if (!objectLinkTargetUser.value) {
    return;
  }
  const payload = buildObjectLinkPayloadRows();
  if (!payload) {
    return;
  }

  savingObjectLinks.value = true;
  try {
    const latestRows = await replaceUserObjectLinks(objectLinkTargetUser.value.id, payload);
    objectLinkRows.value = latestRows.map(mapUserObjectLinkRow);
    ElMessage.success("对象绑定已保存");
  } catch (_error) {
    ElMessage.error("对象绑定保存失败");
  } finally {
    savingObjectLinks.value = false;
  }
}

async function ensureUserGroupsLoaded(): Promise<void> {
  if (loadingGroups.value) {
    return;
  }
  if (userGroups.value.length > 0) {
    return;
  }
  await loadUserGroups();
}

watch(
  () => userForm.roleIds,
  (roleIds) => {
    if (roleIds.length === 0) {
      userForm.primaryRoleId = null;
      return;
    }
    if (!userForm.primaryRoleId || !roleIds.includes(userForm.primaryRoleId)) {
      userForm.primaryRoleId = roleIds[0];
    }
  },
  { deep: true },
);

watch(
  objectLinkTypeSelectOptions,
  (items) => {
    if (!items.includes(objectLinkDraft.linkType)) {
      objectLinkDraft.linkType = items[0] ?? defaultObjectLinkTypes[0];
    }
  },
  { immediate: true },
);

async function loadUsers(): Promise<void> {
  loadingUsers.value = true;
  try {
    const response = await http.get("/api/system/users", {
      params: {
        page: query.page,
        pageSize: query.pageSize,
        keyword: query.keyword || undefined,
        status: query.status || undefined,
      },
    });
    const data = response.data.data as UserListResponse;
    rows.value = data.items;
    total.value = data.total;
  } catch (_error) {
    ElMessage.error("用户列表加载失败");
  } finally {
    loadingUsers.value = false;
  }
}

async function loadUserGroups(): Promise<void> {
  if (!isRoot.value) {
    userGroups.value = [];
    return;
  }
  loadingGroups.value = true;
  try {
    const response = await http.get("/api/system/groups");
    const data = response.data.data as UserGroupListResponse;
    userGroups.value = data.items;
  } catch (_error) {
    ElMessage.error("用户组列表加载失败");
  } finally {
    loadingGroups.value = false;
  }
}

async function handleRefresh(): Promise<void> {
  await loadUsers();
  if (isRoot.value) {
    await loadUserGroups();
  }
}

async function handleSearch(): Promise<void> {
  query.page = 1;
  await loadUsers();
}

async function handlePageChange(page: number): Promise<void> {
  query.page = page;
  await loadUsers();
}

async function handlePageSizeChange(pageSize: number): Promise<void> {
  query.pageSize = pageSize;
  query.page = 1;
  await loadUsers();
}

async function openCreateUserDialog(): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  await ensureUserGroupsLoaded();
  resetUserForm();
  userFormMode.value = "create";
  editingUserRow.value = null;
  userFormVisible.value = true;
}

async function openEditUserDialog(row: UserListItem): Promise<void> {
  if (!isRoot.value) {
    return;
  }
  await ensureUserGroupsLoaded();

  const roleIds = mapRoleIDsFromUser(row);
  const roleIDMap = new Map(userGroups.value.map((item) => [item.roleCode, item.id]));
  const primaryRoleId = roleIDMap.get(row.primaryRole || "") ?? roleIds[0] ?? null;

  userFormMode.value = "edit";
  editingUserRow.value = row;
  userForm.username = row.username;
  userForm.realName = row.realName;
  userForm.password = "";
  userForm.status = row.status;
  userForm.mustChangePassword = row.mustChangePassword;
  userForm.roleIds = roleIds;
  userForm.primaryRoleId = primaryRoleId;
  userFormVisible.value = true;
}

async function handleSubmitUserForm(): Promise<void> {
  if (!isRoot.value) {
    return;
  }

  const username = userForm.username.trim();
  const realName = userForm.realName.trim();
  if (!username) {
    ElMessage.warning("请输入用户名");
    return;
  }
  if (!realName) {
    ElMessage.warning("请输入姓名");
    return;
  }
  if (userForm.roleIds.length === 0) {
    ElMessage.warning("请至少选择一个用户组");
    return;
  }

  const primaryRoleId =
    userForm.primaryRoleId && userForm.roleIds.includes(userForm.primaryRoleId)
      ? userForm.primaryRoleId
      : userForm.roleIds[0];
  if (!primaryRoleId) {
    ElMessage.warning("请选择主角色");
    return;
  }

  const payload = {
    username,
    realName,
    password: userForm.password.trim() || undefined,
    status: userForm.status,
    mustChangePassword: Boolean(userForm.mustChangePassword),
    roleIds: userForm.roleIds,
    primaryRoleId,
  };

  savingUser.value = true;
  try {
    if (userFormMode.value === "create") {
      await http.post("/api/system/users", payload);
      ElMessage.success("用户已新增");
    } else if (editingUserRow.value) {
      await http.put(`/api/system/users/${editingUserRow.value.id}`, payload);
      ElMessage.success("用户已修改");
    }

    userFormVisible.value = false;
    await loadUsers();
  } catch (_error) {
    ElMessage.error(userFormMode.value === "create" ? "新增用户失败" : "修改用户失败");
  } finally {
    savingUser.value = false;
  }
}

function canDeleteUser(row: UserListItem): boolean {
  if (row.username === "root") {
    return false;
  }
  if (currentUserID.value > 0 && row.id === currentUserID.value) {
    return false;
  }
  return true;
}

async function handleDeleteUser(row: UserListItem): Promise<void> {
  if (!isRoot.value || !canDeleteUser(row)) {
    return;
  }

  try {
    await ElMessageBox.confirm(`确认删除用户「${row.username}」吗？`, "删除用户", { type: "warning" });
    await http.delete(`/api/system/users/${row.id}`);
    ElMessage.success("用户已删除");

    if (rows.value.length === 1 && query.page > 1) {
      query.page -= 1;
    }
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除用户失败");
  }
}

function displayRoleNames(row: UserListItem): string[] {
  if (Array.isArray(row.roleNames) && row.roleNames.length > 0) {
    return row.roleNames;
  }
  if (Array.isArray(row.roles) && row.roles.length > 0) {
    return row.roles;
  }
  return [];
}

function roleDisplayName(roleCode: string, row: UserListItem): string {
  if (!roleCode) {
    return "-";
  }
  const codeIndex = row.roles.findIndex((item) => item === roleCode);
  if (codeIndex >= 0 && row.roleNames[codeIndex]) {
    return row.roleNames[codeIndex];
  }
  return roleCode;
}

function openGroupForm(item?: UserGroupItem): void {
  if (!isRoot.value) {
    return;
  }
  if (item) {
    editingGroupID.value = item.id;
    groupForm.roleName = item.roleName;
    groupForm.roleCode = item.roleCode;
    groupForm.description = item.description || "";
  } else {
    editingGroupID.value = null;
    groupForm.roleName = "";
    groupForm.roleCode = "";
    groupForm.description = "";
  }
  groupFormVisible.value = true;
}

async function handleSaveGroup(): Promise<void> {
  const roleName = groupForm.roleName.trim();
  if (!roleName) {
    ElMessage.warning("请输入用户组名称");
    return;
  }
  savingGroup.value = true;
  try {
    const payload = {
      roleName,
      roleCode: groupForm.roleCode.trim() || undefined,
      description: groupForm.description.trim() || undefined,
    };
    if (editingGroupID.value) {
      await http.put(`/api/system/groups/${editingGroupID.value}`, payload);
    } else {
      await http.post("/api/system/groups", payload);
    }
    ElMessage.success("用户组已保存");
    groupFormVisible.value = false;
    await loadUserGroups();
    await loadUsers();
  } catch (_error) {
    ElMessage.error("用户组保存失败");
  } finally {
    savingGroup.value = false;
  }
}

async function handleDeleteGroup(item: UserGroupItem): Promise<void> {
  if (item.isSystem) {
    return;
  }
  try {
    await ElMessageBox.confirm(`确认删除用户组「${item.roleName}」吗？`, "删除用户组", {
      type: "warning",
    });
    await http.delete(`/api/system/groups/${item.id}`);
    ElMessage.success("用户组已删除");
    await loadUserGroups();
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("用户组删除失败");
  }
}

function formatTimestamp(timestamp?: number): string {
  if (!timestamp) {
    return "-";
  }
  return new Date(timestamp * 1000).toLocaleString();
}

function statusTagType(status: UserStatus): "success" | "warning" | "info" | "danger" {
  switch (status) {
    case "active":
      return "success";
    case "inactive":
      return "info";
    case "locked":
      return "danger";
    default:
      return "warning";
  }
}

function statusText(status: UserStatus): string {
  switch (status) {
    case "active":
      return "正常";
    case "inactive":
      return "停用";
    case "locked":
      return "锁定";
    default:
      return status;
  }
}

onMounted(async () => {
  unsavedStore.setSourceMeta(groupFormDirtySourceId, {
    label: "用户组编辑",
    save: handleSaveGroup,
  });
  await loadUsers();
  if (isRoot.value) {
    await loadUserGroups();
    await ensureObjectLinkTypeOptionsLoaded();
  }
});

watch(groupFormVisible, (visible) => {
  if (visible) {
    resetGroupFormBaseline();
    return;
  }
  groupFormBaseline.value = "";
  unsavedStore.clearDirty(groupFormDirtySourceId);
});

watch(
  groupForm,
  () => {
    if (!groupFormVisible.value) {
      unsavedStore.clearDirty(groupFormDirtySourceId);
      return;
    }
    const current = groupFormSignature();
    if (!groupFormBaseline.value || current === groupFormBaseline.value) {
      unsavedStore.clearDirty(groupFormDirtySourceId);
      return;
    }
    unsavedStore.markDirty(groupFormDirtySourceId);
  },
  { deep: true },
);

watch(editingGroupID, () => {
  if (groupFormVisible.value) {
    resetGroupFormBaseline();
  }
});

watch(objectLinkVisible, (visible) => {
  if (visible) {
    return;
  }
  objectLinkTypeConfigVisible.value = false;
  objectLinkTargetUser.value = null;
  objectLinkRows.value = [];
  resetObjectLinkDraft();
});

onBeforeUnmount(() => {
  unsavedStore.unregisterSource(groupFormDirtySourceId);
});
</script>

<style scoped>
.users-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar {
  display: grid;
  grid-template-columns: 1fr 160px auto;
  gap: 12px;
  margin-bottom: 12px;
}

.action-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.group-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.role-checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.object-link-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.object-link-create-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.object-link-table {
  width: 100%;
}

.object-name {
  font-weight: 600;
  color: #303133;
  margin-bottom: 6px;
}

.object-meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.object-link-type-config {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.object-link-type-create-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
}

.object-link-type-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 40px;
  padding: 10px;
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
}

.empty-tip {
  color: #909399;
}

@media (max-width: 900px) {
  .toolbar {
    grid-template-columns: 1fr;
  }
}
</style>

