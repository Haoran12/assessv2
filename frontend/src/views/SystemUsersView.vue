<template>
  <div class="users-page">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>用户管理</strong>
          <el-button type="primary" :loading="loadingUsers" @click="handleRefresh">
            刷新
          </el-button>
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
        <el-table-column label="操作" min-width="420" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" :disabled="!canManageUsers" @click="handleResetPassword(row.id)">
                重置密码
              </el-button>
              <el-select
                :model-value="row.status"
                size="small"
                style="width: 120px"
                :disabled="!canManageUsers"
                @change="(value) => handleStatusChange(row.id, String(value))"
              >
                <el-option label="正常" value="active" />
                <el-option label="停用" value="inactive" />
                <el-option label="锁定" value="locked" />
              </el-select>
              <el-button
                v-if="isRoot"
                size="small"
                type="primary"
                plain
                :disabled="loadingGroups"
                @click="openUserGroupDialog(row)"
              >
                编辑用户组
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
              <el-button
                size="small"
                :disabled="row.isSystem"
                @click="openGroupForm(row)"
              >
                编辑
              </el-button>
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
      v-model="userGroupDialogVisible"
      width="560px"
      :title="`编辑用户组 - ${activeUser?.username ?? ''}`"
      destroy-on-close
    >
      <div v-if="userGroups.length === 0" class="empty-tip">
        暂无可选用户组
      </div>
      <template v-else>
        <el-form label-width="86px">
          <el-form-item label="用户组" required>
            <el-checkbox-group v-model="selectedRoleIDs" class="role-checkbox-group">
              <el-checkbox
                v-for="group in userGroups"
                :key="group.id"
                :label="group.id"
                border
              >
                {{ group.roleName }} ({{ group.roleCode }})
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>
          <el-form-item label="主角色" required>
            <el-radio-group v-model="primaryRoleID">
              <el-radio
                v-for="roleId in selectedRoleIDs"
                :key="`primary-${roleId}`"
                :label="roleId"
              >
                {{ userGroupName(roleId) }}
              </el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="userGroupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="updatingUserGroups" @click="handleSubmitUserGroups">
          保存
        </el-button>
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
        <el-button type="primary" :loading="savingGroup" @click="handleSaveGroup">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { http } from "@/api/http";
import { useAppStore } from "@/stores/app";
import type {
  UserGroupItem,
  UserGroupListResponse,
  UserListItem,
  UserListResponse,
  UserStatus,
} from "@/types/system";

const appStore = useAppStore();

const loadingUsers = ref(false);
const loadingGroups = ref(false);
const savingGroup = ref(false);
const updatingUserGroups = ref(false);

const rows = ref<UserListItem[]>([]);
const total = ref(0);
const userGroups = ref<UserGroupItem[]>([]);

const query = reactive({
  page: 1,
  pageSize: 20,
  keyword: "",
  status: "" as UserStatus | "",
});

const canManageUsers = computed(() => appStore.hasPermission("user:update"));
const isRoot = computed(() => appStore.roles.includes("root") || appStore.primaryRole === "root");

const userGroupDialogVisible = ref(false);
const activeUser = ref<UserListItem | null>(null);
const selectedRoleIDs = ref<number[]>([]);
const primaryRoleID = ref<number | null>(null);

const groupFormVisible = ref(false);
const editingGroupID = ref<number | null>(null);
const groupForm = reactive({
  roleName: "",
  roleCode: "",
  description: "",
});

watch(selectedRoleIDs, (roleIds) => {
  if (roleIds.length === 0) {
    primaryRoleID.value = null;
    return;
  }
  if (!primaryRoleID.value || !roleIds.includes(primaryRoleID.value)) {
    primaryRoleID.value = roleIds[0];
  }
});

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

async function handleResetPassword(userID: number): Promise<void> {
  if (!canManageUsers.value) {
    return;
  }
  try {
    const { value } = await ElMessageBox.prompt(
      "请输入新密码，留空则使用系统默认密码",
      "重置密码",
      {
        confirmButtonText: "确认",
        cancelButtonText: "取消",
        inputType: "password",
        inputValue: "",
      },
    );

    await http.post(`/api/system/users/${userID}/reset-password`, {
      newPassword: value?.trim() ? value.trim() : undefined,
    });
    ElMessage.success("密码已重置");
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("密码重置失败");
  }
}

async function handleStatusChange(userID: number, status: string): Promise<void> {
  if (!canManageUsers.value) {
    return;
  }
  const nextStatus = status as UserStatus;
  try {
    await ElMessageBox.confirm(
      `确认将用户 #${userID} 状态设为「${statusText(nextStatus)}」？`,
      "更新状态",
      { type: "warning" },
    );
    await http.put(`/api/system/users/${userID}/status`, { status: nextStatus });
    ElMessage.success("状态已更新");
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      await loadUsers();
      return;
    }
    ElMessage.error("状态更新失败");
    await loadUsers();
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

function openUserGroupDialog(row: UserListItem): void {
  if (!isRoot.value) {
    return;
  }
  activeUser.value = row;
  const roleIDMap = new Map(userGroups.value.map((item) => [item.roleCode, item.id]));
  selectedRoleIDs.value = row.roles
    .map((roleCode) => roleIDMap.get(roleCode))
    .filter((value): value is number => typeof value === "number");
  const primary = roleIDMap.get(row.primaryRole || "");
  primaryRoleID.value = primary || selectedRoleIDs.value[0] || null;
  userGroupDialogVisible.value = true;
}

function userGroupName(roleId: number): string {
  const item = userGroups.value.find((group) => group.id === roleId);
  return item ? `${item.roleName} (${item.roleCode})` : `#${roleId}`;
}

async function handleSubmitUserGroups(): Promise<void> {
  if (!activeUser.value) {
    return;
  }
  if (selectedRoleIDs.value.length === 0) {
    ElMessage.warning("请至少选择一个用户组");
    return;
  }
  const primary = primaryRoleID.value && selectedRoleIDs.value.includes(primaryRoleID.value)
    ? primaryRoleID.value
    : selectedRoleIDs.value[0];
  if (!primary) {
    ElMessage.warning("请选择主角色");
    return;
  }

  updatingUserGroups.value = true;
  try {
    await http.put(`/api/system/users/${activeUser.value.id}/groups`, {
      roleIds: selectedRoleIDs.value,
      primaryRoleId: primary,
    });
    ElMessage.success("用户组已更新");
    userGroupDialogVisible.value = false;
    await loadUsers();
  } catch (_error) {
    ElMessage.error("用户组更新失败");
  } finally {
    updatingUserGroups.value = false;
  }
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
    await ElMessageBox.confirm(
      `确认删除用户组「${item.roleName}」吗？`,
      "删除用户组",
      { type: "warning" },
    );
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
  await loadUsers();
  if (isRoot.value) {
    await loadUserGroups();
  }
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

.empty-tip {
  color: #909399;
}

@media (max-width: 900px) {
  .toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
