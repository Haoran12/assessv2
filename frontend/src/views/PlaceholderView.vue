<template>
  <el-card>
    <template #header>
      <strong>{{ title }}</strong>
    </template>

    <p>模块页面已创建，可在此继续开发业务功能。</p>

    <el-descriptions :column="1" border>
      <el-descriptions-item label="API 分组">{{ apiGroup }}</el-descriptions-item>
      <el-descriptions-item label="状态">{{ statusText }}</el-descriptions-item>
    </el-descriptions>

    <el-button type="primary" style="margin-top: 16px" @click="checkApi">
      检测后端接口
    </el-button>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { http } from "@/api/http";

const props = defineProps<{
  title: string;
  apiGroup: string;
}>();

const statusText = ref("未检测");

async function checkApi(): Promise<void> {
  try {
    const response = await http.get(`${props.apiGroup}/_ping`);
    statusText.value = response.data?.data?.status ?? "未知";
    ElMessage.success(`${props.title}接口可用。`);
  } catch (_error) {
    statusText.value = "不可用";
    ElMessage.error(`${props.title}接口不可用。`);
  }
}
</script>
