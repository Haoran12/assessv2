<template>
  <div class="backup-view">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>备份恢复</strong>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadBackups">刷新</el-button>
            <el-button type="primary" :loading="creating" :disabled="!canUpdate" @click="handleCreateBackup">
              手动备份
            </el-button>
          </div>
        </div>
      </template>

      <div class="toolbar">
        <el-select v-model="query.type" clearable placeholder="备份类型" @change="handleSearch">
          <el-option label="手动备份" value="manual" />
          <el-option label="自动备份" value="auto" />
          <el-option label="导入前备份" value="before_import" />
          <el-option label="恢复前备份" value="before_restore" />
        </el-select>
        <el-button type="primary" @click="handleSearch">查询</el-button>
      </div>

      <el-alert
        title="恢复会覆盖当前业务数据，系统会先自动创建“恢复前备份”。恢复时需输入 CONFIRM_RESTORE 二次确认。"
        type="warning"
        :closable="false"
        class="tips"
      />

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column prop="backupName" label="备份文件" min-width="280" />
        <el-table-column label="类型" width="130">
          <template #default="{ row }">
            <el-tag :type="backupTypeTag(row.backupType)">{{ backupTypeText(row.backupType) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="140">
          <template #default="{ row }">
            {{ formatFileSize(row.fileSize) }}
          </template>
        </el-table-column>
        <el-table-column label="描述" min-width="220">
          <template #default="{ row }">
            {{ row.description || "-" }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTimestamp(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="260" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" :loading="downloadingId === row.id" @click="handleDownload(row.id, row.backupName)">
                下载
              </el-button>
              <el-button
                size="small"
                type="warning"
                plain
                :disabled="!canUpdate"
                :loading="restoringId === row.id"
                @click="handleRestore(row.id)"
              >
                恢复
              </el-button>
              <el-button
                size="small"
                type="danger"
                plain
                :disabled="!canUpdate"
                :loading="deletingId === row.id"
                @click="handleDelete(row.id)"
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { createManualBackup, deleteBackup, downloadBackupFile, listBackups, restoreBackup } from "@/api/system-admin";
import { useAppStore } from "@/stores/app";
import type { BackupRecordItem, BackupType } from "@/types/system";

const appStore = useAppStore();
const canUpdate = computed(() => appStore.hasPermission("backup:update"));

const loading = ref(false);
const creating = ref(false);
const deletingId = ref<number | null>(null);
const restoringId = ref<number | null>(null);
const downloadingId = ref<number | null>(null);

const rows = ref<BackupRecordItem[]>([]);
const total = ref(0);
const query = reactive({
  page: 1,
  pageSize: 20,
  type: "" as BackupType | "",
});

async function loadBackups(): Promise<void> {
  loading.value = true;
  try {
    const result = await listBackups({
      page: query.page,
      pageSize: query.pageSize,
      type: query.type,
    });
    rows.value = result.items;
    total.value = result.total;
  } catch (_error) {
    ElMessage.error("备份列表加载失败");
  } finally {
    loading.value = false;
  }
}

async function handleCreateBackup(): Promise<void> {
  if (!canUpdate.value) {
    return;
  }

  try {
    const { value } = await ElMessageBox.prompt("请输入备份说明（可选）", "创建手动备份", {
      inputPlaceholder: "例如：季度结算前备份",
      confirmButtonText: "创建",
      cancelButtonText: "取消",
    });

    creating.value = true;
    await createManualBackup(value?.trim() || "");
    ElMessage.success("备份创建成功");
    await loadBackups();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("备份创建失败");
  } finally {
    creating.value = false;
  }
}

async function handleDownload(backupId: number, backupName: string): Promise<void> {
  downloadingId.value = backupId;
  try {
    const blob = await downloadBackupFile(backupId);
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = backupName || `backup-${backupId}.db.gz`;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    URL.revokeObjectURL(url);
  } catch (_error) {
    ElMessage.error("备份下载失败");
  } finally {
    downloadingId.value = null;
  }
}

async function handleDelete(backupId: number): Promise<void> {
  if (!canUpdate.value) {
    return;
  }

  try {
    await ElMessageBox.confirm("确认删除该备份吗？删除后不可恢复。", "删除备份", {
      type: "warning",
      confirmButtonText: "确认删除",
      cancelButtonText: "取消",
    });
    deletingId.value = backupId;
    await deleteBackup(backupId);
    ElMessage.success("备份已删除");
    await loadBackups();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除备份失败");
  } finally {
    deletingId.value = null;
  }
}

async function handleRestore(backupId: number): Promise<void> {
  if (!canUpdate.value) {
    return;
  }

  try {
    const { value } = await ElMessageBox.prompt(
      "此操作会覆盖当前业务数据，请输入 CONFIRM_RESTORE 继续。",
      "恢复确认",
      {
        inputPlaceholder: "CONFIRM_RESTORE",
        confirmButtonText: "确认恢复",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
    restoringId.value = backupId;
    await restoreBackup(backupId, value?.trim() || "");
    ElMessage.success("备份恢复成功");
    await loadBackups();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("备份恢复失败");
  } finally {
    restoringId.value = null;
  }
}

async function handleSearch(): Promise<void> {
  query.page = 1;
  await loadBackups();
}

async function handlePageChange(page: number): Promise<void> {
  query.page = page;
  await loadBackups();
}

async function handlePageSizeChange(pageSize: number): Promise<void> {
  query.pageSize = pageSize;
  query.page = 1;
  await loadBackups();
}

function backupTypeText(type: BackupType): string {
  switch (type) {
    case "manual":
      return "手动备份";
    case "auto":
      return "自动备份";
    case "before_import":
      return "导入前备份";
    case "before_restore":
      return "恢复前备份";
    default:
      return type;
  }
}

function backupTypeTag(type: BackupType): "success" | "warning" | "info" {
  switch (type) {
    case "manual":
      return "success";
    case "auto":
      return "info";
    default:
      return "warning";
  }
}

function formatTimestamp(value: number): string {
  if (!value) {
    return "-";
  }
  return new Date(value * 1000).toLocaleString();
}

function formatFileSize(bytes: number): string {
  if (!bytes || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(index === 0 ? 0 : 2)} ${units[index]}`;
}

onMounted(async () => {
  await loadBackups();
});
</script>

<style scoped>
.backup-view {
  display: grid;
  gap: 16px;
}

.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}

.tips {
  margin-bottom: 12px;
}

.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 920px) {
  .header-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}
</style>
