<template>
  <div class="users-page">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>用户管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingUsers" @click="loadUsers">刷新</el-button>
            <el-button type="primary" @click="openUserDialog()">新增用户</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingUsers" :data="users" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" min-width="150" />
        <el-table-column label="角色" min-width="220">
          <template #default="{ row }">
            <el-tag v-for="name in row.roleNames" :key="`${row.id}-${name}`" size="small" class="mr-4">
              {{ name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="所属组织" min-width="220">
          <template #default="{ row }">
            <el-tag v-for="name in organizationNamesForUser(row)" :key="`${row.id}-org-${name}`" size="small" class="mr-4">
              {{ name }}
            </el-tag>
            <span v-if="organizationNamesForUser(row).length === 0">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openUserDialog(row)">编辑</el-button>
            <el-button link type="danger" :disabled="row.username === 'root'" @click="removeUser(row.id)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card>
      <template #header>
        <div class="header-row">
          <strong>用户组管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingGroups" @click="loadGroups">刷新</el-button>
            <el-button type="primary" @click="openGroupDialog()">新增用户组</el-button>
          </div>
        </div>
      </template>

      <el-table v-loading="loadingGroups" :data="groups" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="roleName" label="名称" min-width="180" />
        <el-table-column prop="roleCode" label="编码" min-width="180" />
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="row.isSystem" @click="openGroupDialog(row)">编辑</el-button>
            <el-button link type="danger" :disabled="row.isSystem" @click="removeGroup(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="userDialogVisible" width="620px" :title="editingUserId ? '编辑用户' : '新增用户'">
      <el-form label-width="110px">
        <el-form-item label="用户名">
          <el-input v-model="userForm.username" :disabled="userForm.username === 'root'" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="userForm.password" type="password" show-password placeholder="编辑时留空不修改" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="userForm.status" style="width: 100%">
            <el-option label="active" value="active" />
            <el-option label="inactive" value="inactive" />
            <el-option label="locked" value="locked" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户组">
          <el-checkbox-group v-model="userForm.roleIds">
            <el-checkbox v-for="group in groups" :key="group.id" :label="group.id">
              {{ group.roleName }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="所在组织">
          <el-select
            v-model="userForm.organizationIds"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :loading="loadingOrganizations"
            style="width: 100%"
            placeholder="请选择组织"
          >
            <el-option v-for="item in organizations" :key="item.id" :label="item.orgName" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="userDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingUser" @click="saveUser">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="groupDialogVisible" width="520px" :title="editingGroupId ? '编辑用户组' : '新增用户组'">
      <el-form label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="groupForm.roleName" />
        </el-form-item>
        <el-form-item label="编码">
          <el-input v-model="groupForm.roleCode" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="groupForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingGroup" @click="saveGroup">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { http } from "@/api/http";
import { listOrganizations } from "@/api/org";
import type { OrganizationItem } from "@/types/org";
import type { UserGroupItem, UserListItem } from "@/types/system";

const users = ref<UserListItem[]>([]);
const groups = ref<UserGroupItem[]>([]);
const organizations = ref<OrganizationItem[]>([]);

const loadingUsers = ref(false);
const loadingGroups = ref(false);
const loadingOrganizations = ref(false);
const savingUser = ref(false);
const savingGroup = ref(false);

const userDialogVisible = ref(false);
const editingUserId = ref<number | null>(null);
const userForm = reactive({
  username: "",
  password: "",
  status: "active",
  roleIds: [] as number[],
  organizationIds: [] as number[],
});

const groupDialogVisible = ref(false);
const editingGroupId = ref<number | null>(null);
const groupForm = reactive({
  roleName: "",
  roleCode: "",
  description: "",
});

const roleIdByCode = (): Record<string, number> => {
  const map: Record<string, number> = {};
  for (const item of groups.value) {
    map[item.roleCode] = item.id;
  }
  return map;
};

async function loadUsers(): Promise<void> {
  loadingUsers.value = true;
  try {
    const response = await http.get("/api/system/users", { params: { page: 1, pageSize: 200 } });
    users.value = response.data?.data?.items || [];
  } finally {
    loadingUsers.value = false;
  }
}

async function loadGroups(): Promise<void> {
  loadingGroups.value = true;
  try {
    const response = await http.get("/api/system/groups");
    groups.value = response.data?.data?.items || [];
  } finally {
    loadingGroups.value = false;
  }
}

async function loadOrganizations(): Promise<void> {
  loadingOrganizations.value = true;
  try {
    organizations.value = await listOrganizations({ status: "active" });
  } finally {
    loadingOrganizations.value = false;
  }
}

function normalizeNumberIds(values: unknown[]): number[] {
  const normalized: number[] = [];
  const seen = new Set<number>();
  for (const value of values) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      continue;
    }
    if (seen.has(parsed)) {
      continue;
    }
    seen.add(parsed);
    normalized.push(parsed);
  }
  return normalized;
}

function organizationTypeById(organizationId: number): string {
  return organizations.value.find((item) => item.id === organizationId)?.orgType || "company";
}

function organizationNamesForUser(user: UserListItem): string[] {
  const names = user.organizations
    .map((scope) => organizations.value.find((item) => item.id === scope.organizationId)?.orgName || "")
    .filter((name) => name.length > 0);
  return Array.from(new Set(names));
}

function extractErrorMessage(error: unknown, fallback: string): string {
  const message = (error as { response?: { data?: { message?: unknown } } })?.response?.data?.message;
  if (typeof message === "string" && message.trim()) {
    return message.trim();
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return fallback;
}

function openUserDialog(user?: UserListItem): void {
  if (!user) {
    editingUserId.value = null;
    userForm.username = "";
    userForm.password = "";
    userForm.status = "active";
    userForm.roleIds = [];
    userForm.organizationIds = [];
    userDialogVisible.value = true;
    return;
  }

  const map = roleIdByCode();
  editingUserId.value = user.id;
  userForm.username = user.username;
  userForm.password = "";
  userForm.status = user.status;
  userForm.roleIds = user.roles.map((code) => map[code]).filter((id) => Number.isFinite(id));
  userForm.organizationIds = normalizeNumberIds(user.organizations.map((item) => item.organizationId));
  userDialogVisible.value = true;
}

async function saveUser(): Promise<void> {
  if (!userForm.username.trim()) {
    ElMessage.warning("用户名不能为空");
    return;
  }
  const roleIds = normalizeNumberIds(userForm.roleIds);
  if (roleIds.length === 0) {
    ElMessage.warning("请至少选择一个用户组");
    return;
  }
  const organizationIds = normalizeNumberIds(userForm.organizationIds);
  if (userForm.username !== "root" && organizationIds.length === 0) {
    ElMessage.warning("请至少选择一个所在组织");
    return;
  }

  const organizationScopes = organizationIds.map((organizationId, index) => ({
    organizationType: organizationTypeById(organizationId),
    organizationId,
    isPrimary: index === 0,
  }));

  savingUser.value = true;
  try {
    const payload = {
      username: userForm.username.trim(),
      password: userForm.password.trim() || undefined,
      status: userForm.status,
      mustChangePassword: false,
      roleIds,
      primaryRoleId: roleIds[0],
      organizations: organizationScopes,
    };
    if (editingUserId.value) {
      await http.put(`/api/system/users/${editingUserId.value}`, payload);
    } else {
      await http.post("/api/system/users", payload);
    }
    userDialogVisible.value = false;
    ElMessage.success("用户已保存");
    await loadUsers();
  } catch (error) {
    ElMessage.error(extractErrorMessage(error, "保存用户失败"));
  } finally {
    savingUser.value = false;
  }
}

async function removeUser(userId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("确认删除该用户吗？", "删除用户", { type: "warning" });
    await http.delete(`/api/system/users/${userId}`);
    ElMessage.success("用户已删除");
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除用户失败");
  }
}

function openGroupDialog(group?: UserGroupItem): void {
  if (!group) {
    editingGroupId.value = null;
    groupForm.roleName = "";
    groupForm.roleCode = "";
    groupForm.description = "";
    groupDialogVisible.value = true;
    return;
  }
  editingGroupId.value = group.id;
  groupForm.roleName = group.roleName;
  groupForm.roleCode = group.roleCode;
  groupForm.description = group.description || "";
  groupDialogVisible.value = true;
}

async function saveGroup(): Promise<void> {
  if (!groupForm.roleName.trim()) {
    ElMessage.warning("用户组名称不能为空");
    return;
  }
  savingGroup.value = true;
  try {
    const payload = {
      roleName: groupForm.roleName.trim(),
      roleCode: groupForm.roleCode.trim() || undefined,
      description: groupForm.description.trim() || undefined,
    };
    if (editingGroupId.value) {
      await http.put(`/api/system/groups/${editingGroupId.value}`, payload);
    } else {
      await http.post("/api/system/groups", payload);
    }
    groupDialogVisible.value = false;
    ElMessage.success("用户组已保存");
    await Promise.all([loadGroups(), loadUsers()]);
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存用户组失败";
    ElMessage.error(message);
  } finally {
    savingGroup.value = false;
  }
}

async function removeGroup(groupId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("确认删除该用户组吗？", "删除用户组", { type: "warning" });
    await http.delete(`/api/system/groups/${groupId}`);
    ElMessage.success("用户组已删除");
    await Promise.all([loadGroups(), loadUsers()]);
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除用户组失败");
  }
}

onMounted(async () => {
  await Promise.all([loadGroups(), loadUsers(), loadOrganizations()]);
});
</script>

<style scoped>
.users-page {
  display: grid;
  gap: 16px;
}

.header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.mr-4 {
  margin-right: 4px;
}
</style>
