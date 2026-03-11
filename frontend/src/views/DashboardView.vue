<template>
  <div class="dashboard">
    <el-card>
      <template #header>
        <strong>M1 进度</strong>
      </template>
      <p>
        认证与权限基线已接入真实后端接口，可在此基础上继续迭代业务功能。
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
    name: "数据库认证",
    description: "登录采用 bcrypt + JWT，并通过 /api/system/profile 拉取用户信息。",
  },
  {
    name: "权限守卫",
    description: "路由访问与菜单显示均按权限点校验。",
  },
  {
    name: "强制改密",
    description: "mustChangePassword=true 时仅允许进入修改密码页。",
  },
  {
    name: "用户管理",
    description: "已支持用户列表、重置密码与账号状态变更。",
  },
  {
    name: "会话管理",
    description: "刷新后自动恢复会话，令牌失效自动回到登录页。",
  },
  {
    name: "审计就绪",
    description: "后端已记录登录、登出与改密审计日志。",
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
