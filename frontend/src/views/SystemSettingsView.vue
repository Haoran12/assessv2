<template>
  <div class="settings-view">
    <el-card>
      <template #header>
        <div class="header-row">
          <strong>系统设置</strong>
          <div class="header-actions">
            <el-button :loading="loading" @click="loadSettings">刷新</el-button>
            <el-button type="primary" :loading="saving" :disabled="!canUpdate" @click="handleSave">保存设置</el-button>
          </div>
        </div>
      </template>

      <el-form label-width="170px">
        <el-divider content-position="left">基础设置</el-divider>
        <el-form-item label="系统名称">
          <el-input v-model="form.systemName" maxlength="100" />
        </el-form-item>
        <el-form-item label="系统 Logo">
          <el-input v-model="form.systemLogo" placeholder="可填写 URL 或 Base64" />
        </el-form-item>
        <el-form-item label="时区">
          <el-input v-model="form.systemTimezone" placeholder="例如 Asia/Shanghai" />
        </el-form-item>
        <el-form-item label="分数显示小数位">
          <el-input-number v-model="form.scoreDecimalPlaces" :min="0" :max="6" />
        </el-form-item>

        <el-divider content-position="left">考核设置</el-divider>
        <el-form-item label="默认周期范围(JSON)">
          <el-input v-model="form.assessmentDefaultPeriodRange" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item label="排名规则">
          <el-input v-model="form.assessmentRankingRule" placeholder="例如 dense / competition" />
        </el-form-item>
        <el-form-item label="投票截止时间">
          <el-input v-model="form.voteDeadlineTime" placeholder="HH:mm，例如 18:00" />
        </el-form-item>

        <el-divider content-position="left">安全设置</el-divider>
        <el-form-item label="密码复杂度策略(JSON)">
          <el-input v-model="form.securityPasswordPolicy" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item label="会话超时(分钟)">
          <el-input-number v-model="form.securitySessionTimeoutMinutes" :min="1" :max="1440" />
        </el-form-item>
        <el-form-item label="审计日志保留(天)">
          <el-input-number v-model="form.auditRetentionDays" :min="1" :max="3650" />
        </el-form-item>

        <el-divider content-position="left">备份设置</el-divider>
        <el-form-item label="启用自动备份">
          <el-switch v-model="form.backupAutoEnabled" />
        </el-form-item>
        <el-form-item label="自动备份时间">
          <el-input v-model="form.backupAutoTime" placeholder="HH:mm，例如 02:00" />
        </el-form-item>
        <el-form-item label="备份保留天数">
          <el-input-number v-model="form.backupRetentionDays" :min="1" :max="3650" />
        </el-form-item>
        <el-form-item label="最大备份数量">
          <el-input-number v-model="form.backupMaxCount" :min="1" :max="9999" />
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { getSystemSettings, updateSystemSettings } from "@/api/system-admin";
import { useAppStore } from "@/stores/app";
import { useUnsavedStore } from "@/stores/unsaved";
import type { SystemSettingItem, SystemSettingsResponse } from "@/types/system";

const appStore = useAppStore();
const unsavedStore = useUnsavedStore();
const canUpdate = computed(() => appStore.hasPermission("setting:update"));
const dirtySourceId = "system-settings";

const loading = ref(false);
const saving = ref(false);
const formBaseline = ref("");

const form = reactive({
  systemName: "",
  systemLogo: "",
  systemTimezone: "Asia/Shanghai",
  scoreDecimalPlaces: 2,
  assessmentDefaultPeriodRange: "{}",
  assessmentRankingRule: "dense",
  voteDeadlineTime: "18:00",
  securityPasswordPolicy: "{}",
  securitySessionTimeoutMinutes: 120,
  auditRetentionDays: 180,
  backupAutoEnabled: true,
  backupAutoTime: "02:00",
  backupRetentionDays: 7,
  backupMaxCount: 30,
});

let latestSettings: SystemSettingItem[] = [];

function formSignature(): string {
  return JSON.stringify({
    systemName: form.systemName,
    systemLogo: form.systemLogo,
    systemTimezone: form.systemTimezone,
    scoreDecimalPlaces: form.scoreDecimalPlaces,
    assessmentDefaultPeriodRange: form.assessmentDefaultPeriodRange,
    assessmentRankingRule: form.assessmentRankingRule,
    voteDeadlineTime: form.voteDeadlineTime,
    securityPasswordPolicy: form.securityPasswordPolicy,
    securitySessionTimeoutMinutes: form.securitySessionTimeoutMinutes,
    auditRetentionDays: form.auditRetentionDays,
    backupAutoEnabled: form.backupAutoEnabled,
    backupAutoTime: form.backupAutoTime,
    backupRetentionDays: form.backupRetentionDays,
    backupMaxCount: form.backupMaxCount,
  });
}

function resetBaseline(): void {
  formBaseline.value = formSignature();
  unsavedStore.clearDirty(dirtySourceId);
}

async function loadSettings(): Promise<void> {
  loading.value = true;
  try {
    const result = await getSystemSettings();
    latestSettings = result.items;
    applySettings(result);
  } catch (_error) {
    ElMessage.error("系统设置加载失败");
  } finally {
    loading.value = false;
  }
}

function applySettings(result: SystemSettingsResponse): void {
  form.systemName = settingString(result, "system.name", "AssessV2");
  form.systemLogo = settingString(result, "system.logo", "");
  form.systemTimezone = settingString(result, "system.timezone", "Asia/Shanghai");
  form.scoreDecimalPlaces = settingNumber(result, "score.decimal_places", 2);
  form.assessmentDefaultPeriodRange = settingJSONText(result, "assessment.default_period_range", {});
  form.assessmentRankingRule = settingString(result, "assessment.ranking_rule", "dense");
  form.voteDeadlineTime = settingString(result, "vote.deadline_time", "18:00");
  form.securityPasswordPolicy = settingJSONText(result, "security.password_policy", {});
  form.securitySessionTimeoutMinutes = settingNumber(result, "security.session_timeout_minutes", 120);
  form.auditRetentionDays = settingNumber(result, "audit.retention_days", 180);
  form.backupAutoEnabled = settingBoolean(result, "backup.auto_enabled", true);
  form.backupAutoTime = settingString(result, "backup.auto_time", "02:00");
  form.backupRetentionDays = settingNumber(result, "backup.retention_days", 7);
  form.backupMaxCount = settingNumber(result, "backup.max_count", 30);
  resetBaseline();
}

async function handleSave(): Promise<void> {
  if (!canUpdate.value) {
    return;
  }
  if (!isTimeText(form.backupAutoTime) || !isTimeText(form.voteDeadlineTime)) {
    ElMessage.warning("时间格式必须为 HH:mm，例如 02:00");
    return;
  }

  let assessmentPeriodRangeObject: unknown;
  let passwordPolicyObject: unknown;
  try {
    assessmentPeriodRangeObject = JSON.parse(form.assessmentDefaultPeriodRange);
    passwordPolicyObject = JSON.parse(form.securityPasswordPolicy);
  } catch (_error) {
    ElMessage.warning("JSON 配置格式不正确，请检查后重试");
    return;
  }

  saving.value = true;
  try {
    const result = await updateSystemSettings([
      { settingKey: "system.name", settingValue: form.systemName.trim() },
      { settingKey: "system.logo", settingValue: form.systemLogo.trim() },
      { settingKey: "system.timezone", settingValue: form.systemTimezone.trim() },
      { settingKey: "score.decimal_places", settingValue: Number(form.scoreDecimalPlaces) },
      { settingKey: "assessment.default_period_range", settingValue: assessmentPeriodRangeObject },
      { settingKey: "assessment.ranking_rule", settingValue: form.assessmentRankingRule.trim() },
      { settingKey: "vote.deadline_time", settingValue: form.voteDeadlineTime.trim() },
      { settingKey: "security.password_policy", settingValue: passwordPolicyObject },
      { settingKey: "security.session_timeout_minutes", settingValue: Number(form.securitySessionTimeoutMinutes) },
      { settingKey: "audit.retention_days", settingValue: Number(form.auditRetentionDays) },
      { settingKey: "backup.auto_enabled", settingValue: Boolean(form.backupAutoEnabled) },
      { settingKey: "backup.auto_time", settingValue: form.backupAutoTime.trim() },
      { settingKey: "backup.retention_days", settingValue: Number(form.backupRetentionDays) },
      { settingKey: "backup.max_count", settingValue: Number(form.backupMaxCount) },
    ]);
    latestSettings = result.items;
    applySettings(result);
    ElMessage.success("系统设置已保存");
  } catch (_error) {
    ElMessage.error("系统设置保存失败");
  } finally {
    saving.value = false;
  }
}

function findSettingValue(result: SystemSettingsResponse, key: string): unknown {
  const item = result.items.find((row) => row.settingKey === key);
  if (!item) {
    return undefined;
  }
  return item.value;
}

function settingString(result: SystemSettingsResponse, key: string, fallback: string): string {
  const value = findSettingValue(result, key);
  if (typeof value === "string") {
    return value;
  }
  return fallback;
}

function settingNumber(result: SystemSettingsResponse, key: string, fallback: number): number {
  const value = findSettingValue(result, key);
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  return fallback;
}

function settingBoolean(result: SystemSettingsResponse, key: string, fallback: boolean): boolean {
  const value = findSettingValue(result, key);
  if (typeof value === "boolean") {
    return value;
  }
  return fallback;
}

function settingJSONText(result: SystemSettingsResponse, key: string, fallback: unknown): string {
  const value = findSettingValue(result, key);
  const source = value ?? fallback;
  try {
    return JSON.stringify(source, null, 2);
  } catch (_error) {
    return "{}";
  }
}

function isTimeText(value: string): boolean {
  return /^\d{2}:\d{2}$/.test(value.trim());
}

onMounted(async () => {
  unsavedStore.setSourceMeta(dirtySourceId, {
    label: "系统设置",
    save: handleSave,
  });
  await loadSettings();
});

watch(
  form,
  () => {
    if (!formBaseline.value) {
      return;
    }
    const current = formSignature();
    if (current === formBaseline.value) {
      unsavedStore.clearDirty(dirtySourceId);
      return;
    }
    unsavedStore.markDirty(dirtySourceId);
  },
  { deep: true },
);

onBeforeUnmount(() => {
  unsavedStore.unregisterSource(dirtySourceId);
});
</script>

<style scoped>
.settings-view {
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

@media (max-width: 900px) {
  .header-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
}
</style>
