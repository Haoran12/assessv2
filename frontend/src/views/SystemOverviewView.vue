<template>
  <div class="overview-view">
    <el-card>
      <template #header>
        <strong>系统概览</strong>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="当前用户">{{ appStore.username || "-" }}</el-descriptions-item>
        <el-descriptions-item label="主角色">{{ appStore.primaryRole || "-" }}</el-descriptions-item>
        <el-descriptions-item label="当前场次">
          {{ contextStore.currentSession?.displayName || "未选择" }}
        </el-descriptions-item>
        <el-descriptions-item label="当前组织">
          {{ contextStore.currentSession?.organizationName || "未选择" }}
        </el-descriptions-item>
        <el-descriptions-item label="当前周期">
          {{ contextStore.currentPeriod?.periodName || "未选择" }}
        </el-descriptions-item>
        <el-descriptions-item label="当前对象类型">
          {{ contextStore.currentObjectGroup?.groupName || "未选择" }}
        </el-descriptions-item>
      </el-descriptions>

      <el-alert
        class="mt-12"
        title="当前版本已按“考核场次独立”架构运行，规则绑定与对象分组均基于所选场次生效。"
        type="info"
        :closable="false"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { useAppStore } from "@/stores/app";
import { useContextStore } from "@/stores/context";

const appStore = useAppStore();
const contextStore = useContextStore();

onMounted(async () => {
  await contextStore.ensureInitialized();
});
</script>

<style scoped>
.overview-view {
  display: grid;
  gap: 16px;
}

.mt-12 {
  margin-top: 12px;
}
</style>
