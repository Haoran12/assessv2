<template>
  <div class="users-page">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>用户管理</strong>
          <el-button type="primary" :loading="loading" @click="loadUsers">
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

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="realName" label="姓名" min-width="140" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="主要角色" min-width="120">
          <template #default="{ row }">
            {{ row.primaryRole || "-" }}
          </template>
        </el-table-column>
        <el-table-column label="强制改密" width="130">
          <template #default="{ row }">
            <el-tag :type="row.mustChangePassword ? 'warning' : 'success'">
              {{ row.mustChangePassword ? "是" : "否" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最近登录" min-width="180">
          <template #default="{ row }">
            {{ formatTimestamp(row.lastLoginAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="320" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button
                size="small"
                @click="handleResetPassword(row.id)"
                :disabled="!canManageUsers"
              >
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { http } from "@/api/http";
import { useAppStore } from "@/stores/app";
import type { UserListItem, UserListResponse, UserStatus } from "@/types/system";

const appStore = useAppStore();

const loading = ref(false);
const rows = ref<UserListItem[]>([]);
const total = ref(0);
const query = reactive({
  page: 1,
  pageSize: 20,
  keyword: "",
  status: "" as UserStatus | "",
});

const canManageUsers = computed(() => appStore.hasPermission("user:update"));

async function loadUsers(): Promise<void> {
  loading.value = true;
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
    ElMessage.error("用户列表加载失败。");
  } finally {
    loading.value = false;
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
      "请输入新密码，留空则使用系统默认密码。",
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
    ElMessage.success("密码已重置。");
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("密码重置失败。");
  }
}

async function handleStatusChange(userID: number, status: string): Promise<void> {
  if (!canManageUsers.value) {
    return;
  }
  const nextStatus = status as UserStatus;
  try {
    await ElMessageBox.confirm(
      `确认将用户 #${userID} 状态设为“${statusText(nextStatus)}”？`,
      "更新状态",
      { type: "warning" },
    );
    await http.put(`/api/system/users/${userID}/status`, { status: nextStatus });
    ElMessage.success("状态已更新。");
    await loadUsers();
  } catch (error) {
    if (String(error).includes("cancel")) {
      await loadUsers();
      return;
    }
    ElMessage.error("状态更新失败。");
    await loadUsers();
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

.toolbar {
  display: grid;
  grid-template-columns: 1fr 160px auto;
  gap: 12px;
  margin-bottom: 12px;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 900px) {
  .toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
