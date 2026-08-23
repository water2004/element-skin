<template>
  <div class="expanded-wrapper">
    <div class="config-section">
      <div class="text-[13px] font-semibold text-[var(--color-text-light)] mb-3 uppercase">
        API 接口定义
      </div>
      <div class="url-grid">
        <div class="url-item">
          <label>Session URL</label>
          <el-input v-model="row.session_url" placeholder="https://sessionserver.mojang.com" />
        </div>
        <div class="url-item">
          <label>Account URL</label>
          <el-input v-model="row.account_url" placeholder="https://api.mojang.com" />
        </div>
        <div class="url-item">
          <label>Services URL</label>
          <el-input v-model="row.services_url" placeholder="https://api.minecraftservices.com" />
        </div>
        <div class="url-item">
          <label>材质域名 (逗号分隔)</label>
          <el-input v-model="row.skin_domains_text" placeholder="textures.minecraft.net" />
        </div>
        <div class="url-item">
          <label>缓存 TTL (秒)</label>
          <el-input-number v-model="row.cache_ttl" :min="0" :controls="true" class="w-full" />
        </div>
      </div>
    </div>

    <div class="config-section mt-6">
      <div class="text-[13px] font-semibold text-[var(--color-text-light)] mb-3 uppercase">
        功能与权限控制
      </div>
      <div class="features-panel">
        <div class="feature-card-item" :class="{ active: row.enable_profile }">
          <div class="flex items-start gap-3">
            <el-switch v-model="row.enable_profile" />
            <div class="flex flex-col">
              <span class="text-sm font-semibold text-[var(--color-heading)]">Profile 转发</span>
              <span class="text-[11px] text-[var(--color-text-light)] mt-1"
                >允许向此端点查询 UUID 和皮肤材质</span
              >
            </div>
          </div>
        </div>
        <div class="feature-card-item" :class="{ active: row.enable_hasjoined }">
          <div class="flex items-start gap-3">
            <el-switch v-model="row.enable_hasjoined" />
            <div class="flex flex-col">
              <span class="text-sm font-semibold text-[var(--color-heading)]">Auth 认证回退</span>
              <span class="text-[11px] text-[var(--color-text-light)] mt-1"
                >本地验证失败后尝试以此端点验证 session</span
              >
            </div>
          </div>
        </div>
        <div class="feature-card-item" :class="{ active: row.enable_whitelist }">
          <div class="flex items-start gap-3">
            <el-switch v-model="row.enable_whitelist" @change="handleWhitelistToggle" />
            <div class="flex flex-col">
              <span class="text-sm font-semibold text-[var(--color-heading)]">开启白名单</span>
              <span class="text-[11px] text-[var(--color-text-light)] mt-1"
                >仅允许特定玩家使用此端点进行验证</span
              >
            </div>
          </div>
        </div>
      </div>
    </div>

    <transition name="el-zoom-in-top">
      <div v-if="row.enable_whitelist" class="whitelist-box mt-6">
        <div class="section-header-small">
          <div class="text-[13px] font-semibold text-[var(--color-text-light)] mb-3 uppercase">
            端点白名单列表
            <DirtyTag :visible="hasWhitelistChanges(row)" />
          </div>
          <div class="add-user-form-box">
            <el-input
              v-model="row._new_user"
              placeholder="输入 Minecraft ID"
              size="small"
              @keyup.enter="emit('addUser', row)"
            >
              <template #append>
                <el-button @click="emit('addUser', row)">添加</el-button>
              </template>
            </el-input>
          </div>
        </div>

        <el-table :data="row._whitelist || []" size="small" class="inner-table" max-height="250">
          <el-table-column prop="username" label="玩家 ID" />
          <el-table-column prop="created_at" label="添加时间" width="160">
            <template #default="scope">
              {{ new Date(scope.row.created_at).toLocaleDateString() }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="60" align="center">
            <template #default="scope">
              <el-button
                type="danger"
                :icon="Delete"
                size="small"
                @click="emit('removeUser', row, scope.row.username)"
                link
              />
            </template>
          </el-table-column>
        </el-table>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { Delete } from '@element-plus/icons-vue'

import type { FallbackRow } from '@/components/admin/mojang/types'
import { hasWhitelistChanges } from '@/components/admin/mojang/whitelist'
import DirtyTag from '@/components/common/DirtyTag.vue'

const row = defineModel<FallbackRow>('row', { required: true })

const emit = defineEmits<{
  loadWhitelist: [row: FallbackRow]
  addUser: [row: FallbackRow]
  removeUser: [row: FallbackRow, username: string]
}>()

function handleWhitelistToggle(val: string | number | boolean) {
  if (val && row.value.id && !row.value._loaded) {
    emit('loadWhitelist', row.value)
  }
}
</script>

<style scoped>
.expanded-wrapper {
  padding: 24px 30px;
  background: var(--color-background-soft);
  border-top: 1px solid var(--color-border);
}

.url-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.url-item label {
  display: block;
  font-size: 12px;
  color: var(--color-text-light);
  margin-bottom: 6px;
}

.features-panel {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.feature-card-item {
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
  padding: 16px;
  border-radius: 10px;
  transition: var(--transition-base);
}

.feature-card-item.active {
  border-color: var(--el-color-primary-light-5);
  background: rgba(64, 158, 255, 0.05);
}

.whitelist-box {
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 20px;
}

.section-header-small {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.add-user-form-box {
  width: 300px;
}

@media (max-width: 768px) {
  .url-grid,
  .features-panel {
    grid-template-columns: 1fr;
  }

  .expanded-wrapper {
    padding: 16px;
  }

  .section-header-small {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .add-user-form-box {
    width: 100%;
  }
}
</style>
