<template>
  <div class="audit-view">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>审计日志</strong>
          <el-button :loading="loading" @click="loadAuditLogs">刷新</el-button>
        </div>
      </template>

      <div class="toolbar">
        <el-input v-model="query.keyword" clearable placeholder="关键字（操作详情/操作人）" @keyup.enter="handleSearch" />
        <el-select v-model="query.actionType" clearable placeholder="操作类型">
          <el-option label="创建" value="create" />
          <el-option label="更新" value="update" />
          <el-option label="删除" value="delete" />
          <el-option label="导入" value="import" />
          <el-option label="导出" value="export" />
          <el-option label="备份" value="backup" />
          <el-option label="恢复" value="restore" />
          <el-option label="登录" value="login" />
          <el-option label="登出" value="logout" />
        </el-select>
        <el-input v-model="query.targetType" clearable placeholder="目标表（如 system_settings）" />
        <el-date-picker
          v-model="timeRange"
          type="datetimerange"
          value-format="x"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
        />
        <el-button type="primary" @click="handleSearch">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="id" label="编号" width="80" />
        <el-table-column label="操作时间" width="180">
          <template #default="{ row }">
            {{ formatTimestamp(row.createdAt) }}
          </template>
        </el-table-column>
        <el-table-column label="操作人" width="150">
          <template #default="{ row }">
            {{ row.realName || row.username || "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="actionType" label="操作类型" width="120" />
        <el-table-column label="事件编码" min-width="180">
          <template #default="{ row }">
            {{ row.eventCode || "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="targetType" label="目标类型" width="180" />
        <el-table-column label="目标ID" width="100">
          <template #default="{ row }">
            {{ row.targetId ?? "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="ipAddress" label="IP" width="140" />
        <el-table-column label="操作摘要" min-width="300">
          <template #default="{ row }">
            <el-tooltip :content="row.summary || row.actionDetail" placement="top" :show-after="300">
              <span class="summary">{{ row.summary || row.actionDetail || "-" }}</span>
            </el-tooltip>
            <el-tag v-if="row.changeCount > 0" size="small" class="change-tag">变更 {{ row.changeCount }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-button size="small" @click="openDetail(row.id)">详情</el-button>
              <el-button
                size="small"
                type="warning"
                plain
                :disabled="!canRollback || rollbackLoadingId === row.id"
                @click="handleRollback(row.id)"
              >
                回滚
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

    <el-dialog v-model="detailVisible" title="审计详情" width="860px" destroy-on-close>
      <el-descriptions :column="2" border v-if="detail">
        <el-descriptions-item label="日志ID">{{ detail.id }}</el-descriptions-item>
        <el-descriptions-item label="操作时间">{{ formatTimestamp(detail.createdAt) }}</el-descriptions-item>
        <el-descriptions-item label="操作人">{{ detail.realName || detail.username || "-" }}</el-descriptions-item>
        <el-descriptions-item label="操作类型">{{ detail.actionType }}</el-descriptions-item>
        <el-descriptions-item label="事件编码">{{ detail.eventCode || "-" }}</el-descriptions-item>
        <el-descriptions-item label="操作摘要">{{ detail.summary || "-" }}</el-descriptions-item>
        <el-descriptions-item label="目标类型">{{ detail.targetType || "-" }}</el-descriptions-item>
        <el-descriptions-item label="目标ID">{{ detail.targetId ?? "-" }}</el-descriptions-item>
      </el-descriptions>

      <el-divider content-position="left">字段差异</el-divider>
      <el-empty v-if="!detail || detail.diffs.length === 0" description="该记录未提供可展示的字段差异" />
      <el-table v-else :data="detail.diffs" border>
        <el-table-column label="字段" min-width="220">
          <template #default="{ row }">
            {{ row.label || row.field }}
          </template>
        </el-table-column>
        <el-table-column prop="changeType" label="变更类型" width="120" />
        <el-table-column label="回滚前">
          <template #default="{ row }">
            <pre class="json-block">{{ stringifyValue(row.before) }}</pre>
          </template>
        </el-table-column>
        <el-table-column label="回滚后">
          <template #default="{ row }">
            <pre class="json-block">{{ stringifyValue(row.after) }}</pre>
          </template>
        </el-table-column>
      </el-table>

      <el-divider content-position="left">原始详情</el-divider>
      <pre class="json-block">{{ detail ? stringifyValue(detail.detail) : "{}" }}</pre>

      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button
          type="warning"
          :disabled="!canRollback || !detail?.canRollback"
          :loading="rollbackLoadingId === detail?.id"
          @click="detail && handleRollback(detail.id)"
        >
          回滚该记录
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { getAuditLogDetail, listAuditLogs, rollbackAuditLog } from "@/api/system-admin";
import { useAppStore } from "@/stores/app";
import type { AuditLogDetail, AuditLogItem } from "@/types/system";

const appStore = useAppStore();
const canRollback = computed(() => appStore.hasPermission("audit:rollback"));

const loading = ref(false);
const rollbackLoadingId = ref<number | null>(null);
const rows = ref<AuditLogItem[]>([]);
const total = ref(0);
const timeRange = ref<string[]>([]);
const query = reactive({
  page: 1,
  pageSize: 20,
  actionType: "",
  targetType: "",
  keyword: "",
});

const detailVisible = ref(false);
const detail = ref<AuditLogDetail | null>(null);

async function loadAuditLogs(): Promise<void> {
  loading.value = true;
  try {
    const [startMs, endMs] = timeRange.value;
    const startAt = startMs ? Math.floor(Number(startMs) / 1000) : undefined;
    const endAt = endMs ? Math.floor(Number(endMs) / 1000) : undefined;

    const result = await listAuditLogs({
      page: query.page,
      pageSize: query.pageSize,
      actionType: query.actionType || undefined,
      targetType: query.targetType || undefined,
      keyword: query.keyword || undefined,
      startAt,
      endAt,
    });
    rows.value = result.items;
    total.value = result.total;
  } catch (_error) {
    ElMessage.error("审计日志加载失败");
  } finally {
    loading.value = false;
  }
}

async function openDetail(auditId: number): Promise<void> {
  try {
    detail.value = await getAuditLogDetail(auditId);
    detailVisible.value = true;
  } catch (_error) {
    ElMessage.error("审计详情加载失败");
  }
}

async function handleRollback(auditId: number): Promise<void> {
  if (!canRollback.value) {
    return;
  }
  try {
    await ElMessageBox.confirm("确认回滚这条审计记录对应的数据变更吗？", "回滚确认", {
      type: "warning",
      confirmButtonText: "确认回滚",
      cancelButtonText: "取消",
    });
    rollbackLoadingId.value = auditId;
    await rollbackAuditLog(auditId);
    ElMessage.success("回滚成功");
    await loadAuditLogs();
    if (detail.value?.id === auditId) {
      detail.value = await getAuditLogDetail(auditId);
    }
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    ElMessage.error("回滚失败");
  } finally {
    rollbackLoadingId.value = null;
  }
}

async function handleSearch(): Promise<void> {
  query.page = 1;
  await loadAuditLogs();
}

async function handlePageChange(page: number): Promise<void> {
  query.page = page;
  await loadAuditLogs();
}

async function handlePageSizeChange(pageSize: number): Promise<void> {
  query.pageSize = pageSize;
  query.page = 1;
  await loadAuditLogs();
}

function formatTimestamp(value: number): string {
  if (!value) {
    return "-";
  }
  return new Date(value * 1000).toLocaleString();
}

function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "-";
  }
  if (typeof value === "string") {
    return value;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch (_error) {
    return String(value);
  }
}

onMounted(async () => {
  await loadAuditLogs();
});
</script>

<style scoped>
.audit-view {
  display: grid;
  gap: 16px;
}

.header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.toolbar {
  display: grid;
  grid-template-columns: 1.2fr 160px 220px 1.2fr auto;
  gap: 10px;
  margin-bottom: 12px;
}

.action-row {
  display: flex;
  gap: 8px;
}

.summary {
  display: inline-block;
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-right: 8px;
}

.change-tag {
  vertical-align: middle;
}

.pager {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
}

.json-block {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.45;
}

@media (max-width: 1280px) {
  .toolbar {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
