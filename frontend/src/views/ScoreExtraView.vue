<template>
  <div class="score-extra-view">
    <el-card>
      <template #header>
        <div class="card-header">
          <strong>M4 分数采集 - 加减分管理</strong>
          <div class="header-actions">
            <el-button :loading="loadingRows" @click="loadRows">刷新</el-button>
            <el-button type="primary" :disabled="!canEdit" @click="openCreateDialog">新增加减分</el-button>
          </div>
        </div>
      </template>

      <div class="filters">
        <el-select v-model="contextYearId" placeholder="年度" style="width: 150px">
          <el-option
            v-for="item in contextStore.years"
            :key="item.id"
            :label="`${item.year} - ${item.yearName}`"
            :value="item.id"
          />
        </el-select>
        <el-select v-model="contextPeriodCode" placeholder="周期" style="width: 140px">
          <el-option v-for="item in periodOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select
          v-model="filters.objectId"
          clearable
          filterable
          placeholder="考核对象"
          style="width: 260px"
          @change="loadRows"
        >
          <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
        </el-select>
        <el-select
          v-model="filters.pointType"
          clearable
          placeholder="分值类型"
          style="width: 140px"
          @change="loadRows"
        >
          <el-option label="加分" value="add" />
          <el-option label="减分" value="deduct" />
        </el-select>
      </div>

      <el-alert title="加减分范围：-20 ~ +20，且必须填写原因" type="warning" :closable="false" class="range-tip" />

      <el-table v-loading="loadingRows" :data="rows" border>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="考核对象" min-width="180">
          <template #default="{ row }">{{ objectNameMap[row.objectId] || `对象#${row.objectId}` }}</template>
        </el-table-column>
        <el-table-column prop="pointType" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.pointType === 'add' ? 'success' : 'danger'">
              {{ row.pointType === "add" ? "加分" : "减分" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="分值" width="110">
          <template #default="{ row }">
            {{ signedPointsText(row.pointType, row.points) }}
          </template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="220" />
        <el-table-column prop="evidence" label="佐证材料" min-width="180">
          <template #default="{ row }">{{ row.evidence || "-" }}</template>
        </el-table-column>
        <el-table-column label="审批状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.approvedBy ? 'success' : 'info'">{{ row.approvedBy ? "已审批" : "待审批" }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="录入时间" min-width="170">
          <template #default="{ row }">{{ formatTimestamp(row.inputAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :disabled="!canEdit" @click="openEditDialog(row)">编辑</el-button>
            <el-button
              link
              type="success"
              :disabled="!canEdit || Boolean(row.approvedBy)"
              @click="handleApprove(row.id)"
            >
              审批
            </el-button>
            <el-button link type="danger" :disabled="!canEdit" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="620px">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="110px">
        <el-form-item label="年度">
          <el-input :model-value="currentYearLabel" disabled />
        </el-form-item>
        <el-form-item label="周期">
          <el-input :model-value="form.periodCode" disabled />
        </el-form-item>
        <el-form-item label="考核对象" prop="objectId">
          <el-select
            v-model="form.objectId"
            filterable
            :disabled="mode === 'edit'"
            placeholder="请选择考核对象"
            style="width: 100%"
          >
            <el-option v-for="item in objects" :key="item.id" :label="item.objectName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="分值类型" prop="pointType">
          <el-radio-group v-model="form.pointType">
            <el-radio-button label="add">加分</el-radio-button>
            <el-radio-button label="deduct">减分</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="分值" prop="points">
          <el-input-number v-model="form.points" :min="0.01" :max="20" :precision="6" controls-position="right" />
        </el-form-item>
        <el-form-item label="原因" prop="reason">
          <el-input v-model="form.reason" type="textarea" :rows="3" maxlength="400" show-word-limit />
        </el-form-item>
        <el-form-item label="佐证材料">
          <el-input v-model="form.evidence" maxlength="300" placeholder="可填写附件编号、链接、说明等" />
        </el-form-item>
        <el-form-item label="同步审批">
          <el-switch v-model="form.approve" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage, ElMessageBox } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";
import { listAssessmentObjects } from "@/api/assessment";
import { approveExtraPoint, createExtraPoint, deleteExtraPoint, listExtraPoints, updateExtraPoint } from "@/api/score";
import type { AssessmentObjectItem } from "@/types/assessment";
import type { ExtraPointItem, ExtraPointType, ScorePeriodCode } from "@/types/score";
import { PERIOD_OPTIONS, formatFloat, formatTimestamp, toObjectNameMap } from "@/utils/assessment";

const appStore = useAppStore();
const contextStore = useContextStore();
const canEdit = computed(() => appStore.hasPermission("score:update"));
const periodOptions = PERIOD_OPTIONS;

const loadingRows = ref(false);
const objects = ref<AssessmentObjectItem[]>([]);
const rows = ref<ExtraPointItem[]>([]);

const filters = reactive({
  objectId: undefined as number | undefined,
  pointType: undefined as ExtraPointType | undefined,
});

const contextYearId = computed({
  get: () => contextStore.yearId,
  set: (value) => contextStore.setYear(value),
});
const contextPeriodCode = computed({
  get: () => contextStore.periodCode,
  set: (value) => contextStore.setPeriodCode(value),
});

const objectNameMap = computed(() => toObjectNameMap(objects.value));
const currentYearLabel = computed(() => {
  const hit = contextStore.currentYear;
  return hit ? `${hit.year} - ${hit.yearName}` : "-";
});

const dialogVisible = ref(false);
const submitting = ref(false);
const mode = ref<"create" | "edit">("create");
const formRef = ref<FormInstance>();
const form = reactive({
  id: 0,
  yearId: 0,
  periodCode: "Q1" as ScorePeriodCode,
  objectId: undefined as number | undefined,
  pointType: "add" as ExtraPointType,
  points: 1,
  reason: "",
  evidence: "",
  approve: false,
});

const dialogTitle = computed(() => (mode.value === "create" ? "新增加减分" : "编辑加减分"));

const formRules: FormRules = {
  objectId: [{ required: true, message: "请选择考核对象", trigger: "change" }],
  pointType: [{ required: true, message: "请选择分值类型", trigger: "change" }],
  points: [
    {
      validator: (_rule, value: unknown, callback: (error?: Error) => void) => {
        const points = Number(value);
        if (!Number.isFinite(points) || points <= 0 || points > 20) {
          callback(new Error("分值必须在 0~20 之间"));
          return;
        }
        callback();
      },
      trigger: "blur",
    },
  ],
  reason: [{ required: true, message: "请填写加减分原因", trigger: "blur" }],
};

function signedPointsText(pointType: ExtraPointType, points: number): string {
  const sign = pointType === "deduct" ? "-" : "+";
  return `${sign}${formatFloat(points, 2)}`;
}

async function loadObjectsForYear(yearId: number): Promise<void> {
  objects.value = await listAssessmentObjects(yearId);
  if (filters.objectId && !objects.value.some((item) => item.id === filters.objectId)) {
    filters.objectId = undefined;
  }
}

async function loadRows(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    return;
  }
  loadingRows.value = true;
  try {
    rows.value = await listExtraPoints({
      yearId: contextStore.yearId,
      periodCode: contextStore.periodCode as ScorePeriodCode,
      objectId: filters.objectId,
      pointType: filters.pointType,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : "加减分列表加载失败";
    ElMessage.error(message);
  } finally {
    loadingRows.value = false;
  }
}

async function handleContextChanged(): Promise<void> {
  if (!contextStore.yearId) {
    rows.value = [];
    objects.value = [];
    return;
  }
  try {
    await loadObjectsForYear(contextStore.yearId);
    await loadRows();
  } catch (error) {
    const message = error instanceof Error ? error.message : "上下文加载失败";
    ElMessage.error(message);
  }
}

function openCreateDialog(): void {
  if (!contextStore.yearId) {
    ElMessage.warning("请先选择年度");
    return;
  }
  mode.value = "create";
  form.id = 0;
  form.yearId = contextStore.yearId;
  form.periodCode = contextStore.periodCode as ScorePeriodCode;
  form.objectId = filters.objectId;
  form.pointType = "add";
  form.points = 1;
  form.reason = "";
  form.evidence = "";
  form.approve = false;
  dialogVisible.value = true;
}

function openEditDialog(row: ExtraPointItem): void {
  mode.value = "edit";
  form.id = row.id;
  form.yearId = row.yearId;
  form.periodCode = row.periodCode;
  form.objectId = row.objectId;
  form.pointType = row.pointType;
  form.points = row.points;
  form.reason = row.reason;
  form.evidence = row.evidence || "";
  form.approve = Boolean(row.approvedBy);
  dialogVisible.value = true;
}

async function submitForm(): Promise<void> {
  if (!formRef.value) {
    return;
  }
  const valid = await formRef.value.validate().catch(() => false);
  if (!valid) {
    return;
  }
  submitting.value = true;
  try {
    if (mode.value === "create") {
      await createExtraPoint({
        yearId: form.yearId,
        periodCode: form.periodCode,
        objectId: form.objectId!,
        pointType: form.pointType,
        points: Number(form.points),
        reason: form.reason.trim(),
        evidence: form.evidence.trim() || undefined,
        approve: form.approve,
      });
      ElMessage.success("加减分新增成功");
    } else {
      await updateExtraPoint(form.id, {
        pointType: form.pointType,
        points: Number(form.points),
        reason: form.reason.trim(),
        evidence: form.evidence.trim() || undefined,
        approve: form.approve,
      });
      ElMessage.success("加减分更新成功");
    }
    dialogVisible.value = false;
    await loadRows();
  } catch (error) {
    const message = error instanceof Error ? error.message : "保存失败";
    ElMessage.error(message);
  } finally {
    submitting.value = false;
  }
}

async function handleApprove(extraPointId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("确认审批该条加减分记录吗？", "审批确认", { type: "warning" });
    await approveExtraPoint(extraPointId);
    ElMessage.success("审批成功");
    await loadRows();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "审批失败";
    ElMessage.error(message);
  }
}

async function handleDelete(extraPointId: number): Promise<void> {
  try {
    await ElMessageBox.confirm("确认删除该条加减分记录吗？", "删除确认", { type: "warning" });
    await deleteExtraPoint(extraPointId);
    ElMessage.success("删除成功");
    await loadRows();
  } catch (error) {
    if (String(error).includes("cancel")) {
      return;
    }
    const message = error instanceof Error ? error.message : "删除失败";
    ElMessage.error(message);
  }
}

onMounted(async () => {
  try {
    await contextStore.ensureInitialized();
    await handleContextChanged();
  } catch (error) {
    const message = error instanceof Error ? error.message : "页面初始化失败";
    ElMessage.error(message);
  }
});

watch(
  () => [contextStore.yearId, contextStore.periodCode],
  async () => {
    await handleContextChanged();
  },
);
</script>

<style scoped>
.score-extra-view {
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

.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 12px;
}

.range-tip {
  margin-bottom: 12px;
}

@media (max-width: 960px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
