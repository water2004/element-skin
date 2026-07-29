<template>
  <div class="max-w-[1100px] mx-auto py-5 animate-fade-in">
    <PageHeader
      title="Fallback 服务配置"
      subtitle="管理外部 Yggdrasil 或 Mojang API 的回退逻辑与白名单"
    >
      <template #icon><Connection /></template>
      <template #actions>
        <el-button
          type="primary"
          :icon="Check"
          @click="saveSettings"
          :loading="saving"
          class="hover-lift"
        >
          保存更改
        </el-button>
      </template>
    </PageHeader>

    <!-- Global Strategy Card -->
    <UiCard class="mb-6" shadow="never">
      <template #header>
        <div class="flex justify-between items-center">
          <div class="flex items-center gap-2 font-semibold text-[var(--color-heading)]">
            <el-icon><Setting /></el-icon>
            <span>全局调度策略</span>
          </div>
        </div>
      </template>
      <div class="flex flex-col gap-4 py-2">
        <UiSegmented v-model="settings.fallback_strategy" variant="modern">
          <el-radio-button value="serial">
            <div class="flex items-center gap-2 font-medium">
              <el-icon><Sort /></el-icon>
              <span>顺序重试</span>
            </div>
          </el-radio-button>
          <el-radio-button value="parallel">
            <div class="flex items-center gap-2 font-medium">
              <el-icon><Operation /></el-icon>
              <span>并发请求</span>
            </div>
          </el-radio-button>
        </UiSegmented>
        <div
          class="text-[13px] text-[var(--color-text-light)] bg-[var(--color-background-soft)] py-3 px-4 rounded-lg border-l-4 border-l-[var(--el-color-primary)]"
        >
          <p v-if="settings.fallback_strategy === 'serial'">
            系统将按照列表优先级顺序逐个尝试 Fallback 端点，直到获得成功响应或遍历完所有服务。
          </p>
          <p v-else>
            系统将同时向所有启用的端点发起并发请求，并采用最快返回的有效响应，适用于追求高性能的场景。
          </p>
        </div>
        <div
          class="flex items-center justify-between gap-4 py-3 px-4 bg-[var(--color-background-soft)] rounded-lg"
        >
          <div class="flex flex-col gap-1">
            <span class="font-semibold text-[var(--color-heading)] text-sm">健康探测周期</span>
            <span class="text-xs text-[var(--color-text-light)]"
              >后台每隔此秒数对所有端点发起一次探测，结果保留 24 小时</span
            >
          </div>
          <el-input-number
            v-model="settings.fallback_probe_interval"
            :min="60"
            :max="86400"
            :step="60"
            controls-position="right"
            class="probe-interval-input"
          />
        </div>
      </div>
    </UiCard>

    <!-- Endpoints List -->
    <UiCard shadow="never">
      <template #header>
        <div class="flex justify-between items-center">
          <div class="flex items-center gap-2 font-semibold text-[var(--color-heading)]">
            <el-icon><List /></el-icon>
            <span>Fallback 服务链</span>
          </div>
          <el-button size="small" :icon="Plus" @click="addFallback" plain class="hover-lift"
            >添加端点</el-button
          >
        </div>
      </template>

      <el-table
        :data="fallbacks"
        row-key="rowKey"
        class="modern-table"
        @expand-change="handleExpandChange"
      >
        <el-table-column type="expand">
          <template #default="{ $index }">
            <FallbackEndpointEditor
              v-model:row="fallbacks[$index]!"
              @load-whitelist="fetchWhitelist"
              @add-user="addUser"
              @remove-user="removeUser"
            />
          </template>
        </el-table-column>

        <el-table-column label="服务备注" min-width="240">
          <template #default="scope">
            <div class="flex items-center gap-3">
              <div class="priority-pill-box">{{ scope.$index + 1 }}</div>
              <el-input
                v-model="scope.row.note"
                placeholder="设置端点备注 (如: Mojang 官方)"
                class="flat-input-box"
              />
              <div class="flex gap-2 ml-auto row-indicators-box">
                <el-tooltip content="Profile 转发" v-if="scope.row.enable_profile">
                  <el-icon class="i-profile"><User /></el-icon>
                </el-tooltip>
                <el-tooltip content="Auth 认证" v-if="scope.row.enable_hasjoined">
                  <el-icon class="i-auth"><Lock /></el-icon>
                </el-tooltip>
                <el-tooltip content="白名单保护" v-if="scope.row.enable_whitelist">
                  <el-icon class="i-whitelist"><ShieldCheck /></el-icon>
                </el-tooltip>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="调度控制" width="160" align="right">
          <template #default="scope">
            <div class="flex gap-2 justify-end">
              <el-tooltip content="上移">
                <el-button
                  :icon="ArrowUp"
                  size="small"
                  circle
                  @click="moveUp(scope.$index)"
                  :disabled="scope.$index === 0"
                />
              </el-tooltip>
              <el-tooltip content="下移">
                <el-button
                  :icon="ArrowDown"
                  size="small"
                  circle
                  @click="moveDown(scope.$index)"
                  :disabled="scope.$index === fallbacks.length - 1"
                />
              </el-tooltip>
              <el-button
                :icon="Delete"
                size="small"
                type="danger"
                circle
                plain
                @click="removeFallback(scope.$index)"
              />
            </div>
          </template>
        </el-table-column>
      </el-table>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { ElMessage, ElLoading } from 'element-plus'
import {
  Plus,
  Delete,
  ArrowUp,
  ArrowDown,
  Connection,
  Check,
  Setting,
  Sort,
  Operation,
  List,
  User,
  Lock,
  Ticket as ShieldCheck,
} from '@element-plus/icons-vue'
import { getAdminSettingsGroup } from '@/api/admin/settings'
import { getWhitelist } from '@/api/admin/whitelist'
import PageHeader from '@/components/common/PageHeader.vue'
import FallbackEndpointEditor from '@/components/admin/mojang/FallbackEndpointEditor.vue'
import type { FallbackRow } from '@/components/admin/mojang/types'
import {
  createEmptyFallbackRow,
  moveFallback,
  normalizeFallbackSettings,
  removeFallbackAt,
  type FallbackSettingsForm,
} from '@/components/admin/mojang/fallbackSettings'
import {
  createWhitelistEntryDraft,
  hasWhitelistChanges,
  setLoadedWhitelist,
} from '@/components/admin/mojang/whitelist'
import { saveFallbackConfiguration } from '@/components/admin/mojang/fallbackSave'
import UiCard from '@/components/ui/UiCard.vue'
import UiSegmented from '@/components/ui/UiSegmented.vue'
import { getErrorMessage } from '@/utils/error'

const settings = ref<FallbackSettingsForm>({
  fallback_strategy: 'serial',
  fallback_probe_interval: 600,
})
const fallbacks = ref<FallbackRow[]>([])
const saving = ref(false)

async function fetchSettings() {
  try {
    const res = await getAdminSettingsGroup('fallback')
    const normalized = normalizeFallbackSettings(res.data, fallbacks.value)
    settings.value = normalized.settings
    fallbacks.value = normalized.rows.map((row) => reactive(row))
  } catch {
    ElMessage.error('加载 Fallback 配置失败')
  }
}

async function saveSettings() {
  const loading = ElLoading.service({
    text: '正在同步配置与白名单...',
    background: 'rgba(0, 0, 0, 0.7)',
  })
  saving.value = true
  try {
    const savedSettings = await saveFallbackConfiguration(settings.value, fallbacks.value)
    const normalized = normalizeFallbackSettings(savedSettings, fallbacks.value)
    settings.value = normalized.settings
    fallbacks.value = normalized.rows.map((row) => reactive(row))

    ElMessage.success('所有配置及白名单已成功同步')
  } catch (e: unknown) {
    console.error(e)
    ElMessage.error('保存失败: ' + getErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
    loading.close()
  }
}

function addFallback() {
  fallbacks.value.push(reactive(createEmptyFallbackRow(fallbacks.value.length)))
}

function removeFallback(index: number) {
  fallbacks.value = removeFallbackAt(fallbacks.value, index)
}

function moveUp(index: number) {
  fallbacks.value = moveFallback(fallbacks.value, index, -1)
}

function moveDown(index: number) {
  fallbacks.value = moveFallback(fallbacks.value, index, 1)
}

function handleExpandChange(row: FallbackRow, expandedRows: FallbackRow[]) {
  const isExpanded = expandedRows.find((r) => r.rowKey === row.rowKey)
  if (isExpanded && row.enable_whitelist && row.id && !row._loaded) {
    fetchWhitelist(row)
  }
}

async function fetchWhitelist(row: FallbackRow) {
  if (!row.id) return
  try {
    const res = await getWhitelist(row.id)
    setLoadedWhitelist(row, res.data.items)
  } catch {
    ElMessage.error(`白名单加载失败: ${row.note || '未命名端点'}`)
  }
}

function addUser(row: FallbackRow) {
  const draft = createWhitelistEntryDraft(row, row._new_user)
  if (!draft.ok) {
    if (draft.reason === 'duplicate') {
      ElMessage.warning('用户已在列表中')
    }
    return
  }
  row._whitelist.unshift(draft.entry)
  row._new_user = ''
}

function removeUser(row: FallbackRow, username: string) {
  row._whitelist = row._whitelist.filter((u) => u.username !== username)
}

onMounted(fetchSettings)
</script>

<style scoped>
.probe-interval-input {
  width: 200px;
}

/* Note Column */
.priority-pill-box {
  background: var(--el-color-primary);
  color: #fff;
  font-size: 11px;
  font-weight: bold;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  flex-shrink: 0;
}
.flat-input-box :deep(.el-input__wrapper) {
  box-shadow: none !important;
  padding: 0;
  background: transparent;
}
.flat-input-box :deep(.el-input__inner) {
  font-weight: 500;
  color: var(--color-heading);
}
.row-indicators-box .el-icon {
  font-size: 14px;
}
.i-profile {
  color: var(--el-color-success);
}
.i-auth {
  color: var(--el-color-primary);
}
.i-whitelist {
  color: var(--el-color-warning);
}
</style>
