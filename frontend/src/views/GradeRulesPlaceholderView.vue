<template>
  <el-card>
    <template #header>
      <strong>{{ title }}</strong>
    </template>
    <el-empty :description="description" />
  </el-card>
</template>

<script setup lang="ts">
const title = "\u7b49\u7b2c\u89c4\u5219";
const description = "\u7b49\u7b2c\u89c4\u5219\u9875\u9762\u6682\u672a\u5b9e\u73b0\uff0c\u5f53\u524d\u4e3a\u5360\u4f4d\u3002";
</script>
