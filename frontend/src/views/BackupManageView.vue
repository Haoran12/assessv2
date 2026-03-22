<template>
  <div class="backup-view">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>备份管理</strong>
          <div class="header-actions">
            <el-button v-if="activeTab === 'org'" :loading="orgLoading" @click="loadOrgPackages">刷新</el-button>
            <el-button v-else :loading="snapshotLoading" @click="loadSnapshots">刷新</el-button>
            <el-button
              v-if="activeTab === 'org'"
              type="primary"
              :disabled="!canOrgUpdate"
              @click="openCreateOrgDialog"
            >
              创建组织备份包
            </el-button>
            <el-button
              v-else
              type="primary"
              :loading="snapshotCreating"
              :disabled="!canSnapshotUpdate"
              @click="handleCreateSnapshot"
            >
              手动全量备份
            </el-button>
          </div>
        </div>
      </template>

      <el-tabs v-model="activeTab" class="backup-tabs">
        <el-tab-pane v-if="canOrgView" label="组织备份包" name="org">
          <div class="toolbar">
            <el-select
              v-model="orgQuery.rootOrganizationId"
              clearable
              filterable
              placeholder="按根组织筛选"
              style="width: 300px"
              @change="handleOrgSearch"
            >
              <el-option
                v-for="org in orgOptions"
                :key="org.id"
                :label="`${org.orgName} (#${org.id})`"
                :value="org.id"
              />
            </el-select>
            <el-button type="primary" @click="handleOrgSearch">查询</el-button>
          </div>

          <el-alert
            title="组织备份包仅包含指定根组织及其子组织的数据。恢复时需输入 CONFIRM_ORG_RESTORE，并会自动先创建一份“恢复前全量快照”。"
            type="warning"
            :closable="false"
            class="tips"
          />

          <el-table v-loading="orgLoading" :data="orgRows" border>
            <el-table-column prop="id" label="编号" width="80" />
            <el-table-column prop="backupName" label="备份包文件" min-width="260" />
            <el-table-column label="根组织" min-width="180">
              <template #default="{ row }">
                {{ orgNameText(row.rootOrganizationId) }}
              </template>
            </el-table-column>
            <el-table-column label="范围组织数" width="110">
              <template #default="{ row }">
                {{ row.scopedOrganizationIds?.length ?? 0 }}
              </template>
            </el-table-column>
            <el-table-column prop="sanitizedHistoryRefsCount" label="历史引用净化" width="120" />
            <el-table-column label="大小" width="130">
              <template #default="{ row }">
                {{ formatFileSize(row.fileSize) }}
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="180">
              <template #default="{ row }">
                {{ formatTimestamp(row.createdAt) }}
              </template>
            </el-table-column>
            <el-table-column label="描述" min-width="200">
              <template #default="{ row }">
                {{ row.description || "-" }}
              </template>
            </el-table-column>
            <el-table-column label="操作" min-width="210" fixed="right">
              <template #default="{ row }">
                <div class="action-row">
                  <el-button
                    size="small"
                    :loading="orgDownloadingId === row.id"
                    @click="handleDownloadOrgPackage(row.id, row.backupName)"
                  >
                    下载
                  </el-button>
                  <el-button
                    size="small"
                    type="warning"
                    plain
                    :disabled="!canOrgUpdate"
                    :loading="orgRestoringId === row.id"
                    @click="handleRestoreOrgPackage(row.id, row.rootOrganizationId)"
                  >
                    恢复
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>

          <div class="pager">
            <el-pagination
              background
              layout="total, prev, pager, next, sizes"
              :total="orgTotal"
              :page-size="orgQuery.pageSize"
              :current-page="orgQuery.page"
              :page-sizes="[10, 20, 50, 100]"
              @current-change="handleOrgPageChange"
              @size-change="handleOrgPageSizeChange"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane v-if="canSnapshotView" label="全量快照" name="snapshot">
          <div class="toolbar">
            <el-select v-model="snapshotQuery.type" clearable placeholder="备份类型" @change="handleSnapshotSearch">
              <el-option label="手动备份" value="manual" />
              <el-option label="自动备份" value="auto" />
              <el-option label="导入前备份" value="before_import" />
              <el-option label="恢复前备份" value="before_restore" />
            </el-select>
            <el-button type="primary" @click="handleSnapshotSearch">查询</el-button>
          </div>

          <el-alert
            title="全量恢复会覆盖当前业务数据，恢复时必须输入 CONFIRM_RESTORE。"
            type="warning"
            :closable="false"
            class="tips"
          />

          <el-table v-loading="snapshotLoading" :data="snapshotRows" border>
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
                  <el-button
                    size="small"
                    :loading="snapshotDownloadingId === row.id"
                    @click="handleDownloadSnapshot(row.id, row.backupName)"
                  >
                    下载
                  </el-button>
                  <el-button
                    size="small"
                    type="warning"
                    plain
                    :disabled="!canSnapshotUpdate"
                    :loading="snapshotRestoringId === row.id"
                    @click="handleRestoreSnapshot(row.id)"
                  >
                    恢复
                  </el-button>
                  <el-button
                    size="small"
                    type="danger"
                    plain
                    :disabled="!canSnapshotUpdate"
                    :loading="snapshotDeletingId === row.id"
                    @click="handleDeleteSnapshot(row.id)"
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
              :total="snapshotTotal"
              :page-size="snapshotQuery.pageSize"
              :current-page="snapshotQuery.page"
              :page-sizes="[10, 20, 50, 100]"
              @current-change="handleSnapshotPageChange"
              @size-change="handleSnapshotPageSizeChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>

      <el-empty v-if="!canOrgView && !canSnapshotView" description="当前账号暂无备份查看权限" />
    </el-card>

    <el-dialog v-model="orgCreateDialogVisible" title="创建组织备份包" width="560px" destroy-on-close>
      <el-form label-width="140px">
        <el-form-item label="根组织" required>
          <el-select v-model="orgCreateForm.rootOrganizationId" filterable placeholder="请选择组织" style="width: 100%">
            <el-option
              v-for="org in orgOptions"
              :key="org.id"
              :label="`${org.orgName} (#${org.id})`"
              :value="org.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="包含员工履历">
          <el-switch v-model="orgCreateForm.includeEmployeeHistory" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="orgCreateForm.description"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
            placeholder="可选，建议填写本次备份用途"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="orgCreateDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="orgCreating" @click="submitCreateOrgPackage">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  createManualBackup,
  createOrgPackage,
  deleteBackup,
  downloadBackupFile,
  downloadOrgPackageFile,
  listBackups,
  listOrgPackages,
  restoreBackup,
  restoreOrgPackage,
} from "@/api/system-admin";
import { listOrganizations } from "@/api/org";
import { useAppStore } from "@/stores/app";
import type { OrganizationItem } from "@/types/org";
import type { BackupRecordItem, BackupType, OrgPackageItem } from "@/types/system";

const appStore = useAppStore();

const canSnapshotView = computed(() => appStore.hasPermission("backup:view"));
const canSnapshotUpdate = computed(() => appStore.hasPermission("backup:update"));
const canOrgView = computed(() => appStore.hasPermission("backup:org:view"));
const canOrgUpdate = computed(() => appStore.hasPermission("backup:org:update"));

const activeTab = ref<"org" | "snapshot">(canOrgView.value ? "org" : "snapshot");

const snapshotLoading = ref(false);
const snapshotCreating = ref(false);
const snapshotDeletingId = ref<number | null>(null);
const snapshotRestoringId = ref<number | null>(null);
const snapshotDownloadingId = ref<number | null>(null);

const snapshotRows = ref<BackupRecordItem[]>([]);
const snapshotTotal = ref(0);
const snapshotQuery = reactive({
  page: 1,
  pageSize: 20,
  type: "" as BackupType | "",
});

const orgLoading = ref(false);
const orgCreating = ref(false);
const orgDownloadingId = ref<number | null>(null);
const orgRestoringId = ref<number | null>(null);

const orgRows = ref<OrgPackageItem[]>([]);
const orgTotal = ref(0);
const orgQuery = reactive({
  page: 1,
  pageSize: 20,
  rootOrganizationId: undefined as number | undefined,
});

const orgOptions = ref<OrganizationItem[]>([]);
const orgNameMap = computed(() => {
  const mapping = new Map<number, string>();
  for (const org of orgOptions.value) {
    mapping.set(org.id, org.orgName);
  }
  return mapping;
});

const orgCreateDialogVisible = ref(false);
const orgCreateForm = reactive({
  rootOrganizationId: undefined as number | undefined,
  includeEmployeeHistory: true,
  description: "",
});

async function loadSnapshots(): Promise<void> {
  if (!canSnapshotView.value) {
    return;
  }
  snapshotLoading.value = true;
  try {
    const result = await listBackups({
      page: snapshotQuery.page,
      pageSize: snapshotQuery.pageSize,
      type: snapshotQuery.type,
    });
    snapshotRows.value = result.items;
    snapshotTotal.value = result.total;
  } catch (_error) {
    ElMessage.error("全量备份列表加载失败");
  } finally {
    snapshotLoading.value = false;
  }
}

async function handleCreateSnapshot(): Promise<void> {
  if (!canSnapshotUpdate.value) {
    return;
  }

  try {
    const { value } = await ElMessageBox.prompt("请输入备份说明（可选）", "创建手动全量备份", {
      inputPlaceholder: "例如：月度结算前快照",
      confirmButtonText: "创建",
      cancelButtonText: "取消",
    });

    snapshotCreating.value = true;
    await createManualBackup(value?.trim() || "");
    ElMessage.success("全量备份创建成功");
    await loadSnapshots();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("全量备份创建失败");
  } finally {
    snapshotCreating.value = false;
  }
}

async function handleDownloadSnapshot(backupId: number, backupName: string): Promise<void> {
  snapshotDownloadingId.value = backupId;
  try {
    const blob = await downloadBackupFile(backupId);
    downloadBlob(blob, backupName || `backup-${backupId}.db.gz`);
  } catch (_error) {
    ElMessage.error("全量备份下载失败");
  } finally {
    snapshotDownloadingId.value = null;
  }
}

async function handleDeleteSnapshot(backupId: number): Promise<void> {
  if (!canSnapshotUpdate.value) {
    return;
  }

  try {
    await ElMessageBox.confirm("确认删除该全量备份吗？删除后不可恢复。", "删除确认", {
      type: "warning",
      confirmButtonText: "确认删除",
      cancelButtonText: "取消",
    });
    snapshotDeletingId.value = backupId;
    await deleteBackup(backupId);
    ElMessage.success("全量备份已删除");
    await loadSnapshots();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("删除全量备份失败");
  } finally {
    snapshotDeletingId.value = null;
  }
}

async function handleRestoreSnapshot(backupId: number): Promise<void> {
  if (!canSnapshotUpdate.value) {
    return;
  }

  try {
    const { value } = await ElMessageBox.prompt(
      "该操作会覆盖当前业务数据，请输入 CONFIRM_RESTORE 继续。",
      "恢复全量快照",
      {
        inputPlaceholder: "CONFIRM_RESTORE",
        confirmButtonText: "确认恢复",
        cancelButtonText: "取消",
        type: "warning",
      },
    );

    snapshotRestoringId.value = backupId;
    await restoreBackup(backupId, value?.trim() || "");
    ElMessage.success("全量快照恢复成功");
    await loadSnapshots();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("全量快照恢复失败");
  } finally {
    snapshotRestoringId.value = null;
  }
}

async function handleSnapshotSearch(): Promise<void> {
  snapshotQuery.page = 1;
  await loadSnapshots();
}

async function handleSnapshotPageChange(page: number): Promise<void> {
  snapshotQuery.page = page;
  await loadSnapshots();
}

async function handleSnapshotPageSizeChange(pageSize: number): Promise<void> {
  snapshotQuery.pageSize = pageSize;
  snapshotQuery.page = 1;
  await loadSnapshots();
}

async function loadOrgOptions(): Promise<void> {
  if (!canOrgView.value) {
    return;
  }
  try {
    const list = await listOrganizations({ status: "active" });
    orgOptions.value = list;
  } catch (_error) {
    ElMessage.error("组织列表加载失败");
  }
}

async function loadOrgPackages(): Promise<void> {
  if (!canOrgView.value) {
    return;
  }
  orgLoading.value = true;
  try {
    const result = await listOrgPackages({
      page: orgQuery.page,
      pageSize: orgQuery.pageSize,
      rootOrganizationId: orgQuery.rootOrganizationId,
    });
    orgRows.value = result.items;
    orgTotal.value = result.total;
  } catch (_error) {
    ElMessage.error("组织备份包列表加载失败");
  } finally {
    orgLoading.value = false;
  }
}

function openCreateOrgDialog(): void {
  if (!canOrgUpdate.value) {
    return;
  }
  if (orgOptions.value.length === 0) {
    ElMessage.warning("当前暂无可选组织，请先检查组织数据");
    return;
  }
  orgCreateForm.rootOrganizationId = orgQuery.rootOrganizationId ?? orgOptions.value[0]?.id;
  orgCreateForm.includeEmployeeHistory = true;
  orgCreateForm.description = "";
  orgCreateDialogVisible.value = true;
}

async function submitCreateOrgPackage(): Promise<void> {
  if (!canOrgUpdate.value) {
    return;
  }
  if (!orgCreateForm.rootOrganizationId) {
    ElMessage.warning("请选择根组织");
    return;
  }

  orgCreating.value = true;
  try {
    await createOrgPackage({
      rootOrganizationId: orgCreateForm.rootOrganizationId,
      includeEmployeeHistory: orgCreateForm.includeEmployeeHistory,
      description: orgCreateForm.description.trim(),
    });
    ElMessage.success("组织备份包创建成功");
    orgCreateDialogVisible.value = false;
    await loadOrgPackages();
  } catch (_error) {
    ElMessage.error("组织备份包创建失败");
  } finally {
    orgCreating.value = false;
  }
}

async function handleDownloadOrgPackage(backupId: number, backupName: string): Promise<void> {
  orgDownloadingId.value = backupId;
  try {
    const blob = await downloadOrgPackageFile(backupId);
    downloadBlob(blob, backupName || `org-package-${backupId}.tar.gz`);
  } catch (_error) {
    ElMessage.error("组织备份包下载失败");
  } finally {
    orgDownloadingId.value = null;
  }
}

async function handleRestoreOrgPackage(backupId: number, rootOrganizationId: number): Promise<void> {
  if (!canOrgUpdate.value) {
    return;
  }

  try {
    const { value } = await ElMessageBox.prompt(
      "该操作将按组织范围覆盖业务数据，并自动创建恢复前全量快照。请输入 CONFIRM_ORG_RESTORE 继续。",
      "恢复组织备份包",
      {
        inputPlaceholder: "CONFIRM_ORG_RESTORE",
        confirmButtonText: "确认恢复",
        cancelButtonText: "取消",
        type: "warning",
      },
    );

    orgRestoringId.value = backupId;
    await restoreOrgPackage(backupId, {
      confirmText: value?.trim() || "",
      mode: "replace_scope",
      targetRootOrganizationId: rootOrganizationId,
    });
    ElMessage.success("组织备份包恢复成功");
    await loadOrgPackages();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("组织备份包恢复失败");
  } finally {
    orgRestoringId.value = null;
  }
}

async function handleOrgSearch(): Promise<void> {
  orgQuery.page = 1;
  await loadOrgPackages();
}

async function handleOrgPageChange(page: number): Promise<void> {
  orgQuery.page = page;
  await loadOrgPackages();
}

async function handleOrgPageSizeChange(pageSize: number): Promise<void> {
  orgQuery.pageSize = pageSize;
  orgQuery.page = 1;
  await loadOrgPackages();
}

function orgNameText(orgId: number): string {
  return orgNameMap.value.get(orgId) || `组织 #${orgId}`;
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

function downloadBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}

onMounted(async () => {
  if (canOrgView.value) {
    await loadOrgOptions();
    await loadOrgPackages();
  }
  if (canSnapshotView.value) {
    await loadSnapshots();
  }
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

.backup-tabs :deep(.el-tabs__header) {
  margin-bottom: 14px;
}

.toolbar {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
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
