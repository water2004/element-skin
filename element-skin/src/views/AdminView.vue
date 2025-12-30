<template>
  <div class="admin-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="admin-sidebar">
        <div class="admin-title">
          <el-icon size="24"><Tools /></el-icon>
          <span>管理面板</span>
        </div>
        <div class="title-divider"></div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/admin/settings">
            <el-icon><Setting /></el-icon>
            <span>站点设置</span>
          </el-menu-item>
          <el-menu-item index="/admin/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item index="/admin/invites">
            <el-icon><Ticket /></el-icon>
            <span>邀请码管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Back /></el-icon>
            <span>返回用户面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="admin-main">
        <router-view />
      </el-main>
    </el-container>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Tools, Setting, User, Ticket, Back } from '@element-plus/icons-vue'

const route = useRoute()

const activeRoute = computed(() => route.path)
</script>

<style scoped>
.admin-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.admin-container :deep(.el-container) {
  min-height: 100vh;
}

.admin-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.admin-title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px 0;
  margin: 0 16px 4px 16px;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.title-divider {
  height: 1px;
  background: #ebeef5;
  margin: 0 16px 4px 16px;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 16px;
  padding: 0 16px !important;
  border-radius: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.sidebar-menu .el-menu-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 3px;
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  transform: translateX(-100%);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-menu .el-menu-item:hover::before {
  transform: translateX(0);
}

.sidebar-menu .el-menu-item:hover {
  background-color: #ecf5ff;
  transform: translateX(4px);
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
  transform: translateX(0);
}

.admin-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
}
</style>
