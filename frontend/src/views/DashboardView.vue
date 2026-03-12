<template>
  <div class="dashboard">
    <el-card>
      <template #header>
        <strong>M1-M3 交付概览</strong>
      </template>
      <p>
        当前前端已接入真实登录、权限守卫、组织架构、年度周期与规则配置核心交互，可直接联调后端接口。
      </p>
    </el-card>

    <el-row :gutter="16">
      <el-col v-for="item in cards" :key="item.name" :span="8">
        <el-card class="dashboard-card">
          <template #header>
            <strong>{{ item.name }}</strong>
          </template>
          <p>{{ item.description }}</p>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
const cards = [
  {
    name: "M1 认证与权限",
    description: "支持真实登录、首次改密、会话恢复、会话过期跳转与用户权限菜单。",
  },
  {
    name: "M2 组织架构",
    description: "支持组织树、组织/部门/人员 CRUD、人员调动与历史追踪。",
  },
  {
    name: "M2 年度周期",
    description: "支持年度创建复制、5 周期查看、状态流转与锁定约束操作。",
  },
  {
    name: "M3 规则配置",
    description: "支持规则模块配置、权重校验、模板保存和跨年度应用。",
  },
  {
    name: "模板与复用",
    description: "支持按对象类型筛选模板并应用到新年度/周期规则。",
  },
  {
    name: "联调准备",
    description: "页面已按后端接口契约实现，可直接开展接口联调和冒烟测试。",
  },
];
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard-card {
  min-height: 150px;
}

@media (max-width: 1100px) {
  .el-col {
    margin-bottom: 16px;
  }
}
</style>
