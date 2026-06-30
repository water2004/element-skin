<template>
  <div class="animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>开发者应用</h1>
        <p>注册 OAuth 应用，管理用户委托权限与服务端 app-only 能力</p>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div class="space-y-6">
        <UiCard class="p-6">
          <div class="mb-5 flex items-center justify-between gap-3">
            <div>
              <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">应用列表</h2>
              <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
                Authorization Code、Device Code 与 Client Credentials 都使用这里的 client。
              </p>
            </div>
            <el-button :loading="loading" @click="loadApps">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-empty v-if="!loading && apps.length === 0" description="还没有 OAuth 应用" />
          <div v-else class="grid gap-3">
            <button
              v-for="app in apps"
              :key="app.client_id"
              type="button"
              class="rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)] p-4 text-left transition hover:-translate-y-0.5 hover:border-[var(--el-color-primary)]"
              :class="{ 'border-[var(--el-color-primary)] ring-2 ring-[var(--el-color-primary-light-8)]': selectedClientId === app.client_id }"
              @click="selectApp(app.client_id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-[var(--color-heading)]">{{ app.name }}</div>
                  <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
                    {{ app.client_id }}
                  </div>
                </div>
                <el-tag size="small" :type="app.status === 'active' ? 'success' : 'info'">
                  {{ app.status === 'active' ? '启用' : '停用' }}
                </el-tag>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="code in app.permissions.slice(0, 4)"
                  :key="code"
                  :label="shortPermission(code)"
                  tone="sky"
                />
                <el-text v-if="app.permissions.length > 4" size="small" type="info">
                  +{{ app.permissions.length - 4 }}
                </el-text>
              </div>
            </button>
          </div>
        </UiCard>

        <UiCard class="p-6">
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">新建应用</h2>
          <el-form class="mt-5" label-position="top">
            <div class="grid gap-4 md:grid-cols-2">
              <el-form-item label="名称">
                <el-input v-model="form.name" maxlength="80" show-word-limit />
              </el-form-item>
              <el-form-item label="类型">
                <el-select v-model="form.client_type">
                  <el-option label="Confidential" value="confidential" />
                  <el-option label="Public" value="public" />
                </el-select>
              </el-form-item>
            </div>
            <el-form-item label="回调地址">
              <el-input v-model="form.redirect_uri" placeholder="https://app.example/callback" />
            </el-form-item>
            <el-form-item label="网站地址">
              <el-input v-model="form.website_url" placeholder="https://app.example" />
            </el-form-item>
            <el-form-item label="说明">
              <el-input v-model="form.description" type="textarea" :rows="2" maxlength="160" />
            </el-form-item>
            <el-form-item label="用户委托权限上限">
              <el-select
                v-model="form.permissions"
                multiple
                filterable
                collapse-tags
                collapse-tags-tooltip
                class="w-full"
              >
                <el-option
                  v-for="item in delegablePermissions"
                  :key="item.code"
                  :label="`${item.code} · ${item.description}`"
                  :value="item.code"
                />
              </el-select>
            </el-form-item>
            <div class="flex justify-end">
              <el-button type="primary" :loading="creating" @click="createApp">
                <el-icon><Plus /></el-icon>
                创建应用
              </el-button>
            </div>
          </el-form>
        </UiCard>
      </div>

      <UiCard class="p-6">
        <el-empty v-if="!selectedApp" description="选择一个应用查看详情" />
        <div v-else class="space-y-5">
          <div>
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">
              {{ selectedApp.name }}
            </h2>
            <p class="mt-2 mb-0 break-all text-xs text-[var(--color-text-light)]">
              {{ selectedApp.client_id }}
            </p>
          </div>

          <el-alert
            v-if="newSecret"
            type="success"
            :closable="false"
            title="Client Secret 只显示一次"
          >
            <div class="mt-2 flex gap-2">
              <el-input :model-value="newSecret" readonly />
              <el-button @click="copyText(newSecret)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </el-alert>

          <div class="grid gap-2 text-sm">
            <div class="flex items-center justify-between gap-3">
              <span class="text-[var(--color-text-light)]">类型</span>
              <el-tag>{{ selectedApp.client_type }}</el-tag>
            </div>
            <div class="flex items-center justify-between gap-3">
              <span class="text-[var(--color-text-light)]">回调</span>
              <span class="min-w-0 truncate">{{ selectedApp.redirect_uri }}</span>
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <el-button
              v-if="selectedApp.client_type === 'confidential'"
              :loading="rotating"
              @click="rotateSecret"
            >
              <el-icon><Key /></el-icon>
              轮换密钥
            </el-button>
            <el-button type="danger" :loading="deleting" @click="deleteSelected">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </div>

          <el-divider />

          <div>
            <div class="mb-3 flex items-center justify-between">
              <h3 class="m-0 text-base font-semibold text-[var(--color-heading)]">App-only 权限</h3>
              <el-button v-if="canManageClientPermissions" text :loading="permissionLoading" @click="loadClientPermissions">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </div>
            <el-alert
              v-if="!canManageClientPermissions"
              type="info"
              :closable="false"
              title="需要权限管理能力才能编辑 app-only 权限"
            />
            <div v-else class="space-y-4">
              <div class="flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="code in clientPermissionInfo?.effective_permissions ?? []"
                  :key="code"
                  :label="shortPermission(code)"
                  tone="emerald"
                  removable
                  @remove="clearClientPermission(code)"
                />
                <el-text v-if="(clientPermissionInfo?.effective_permissions.length ?? 0) === 0" size="small" type="info">
                  暂无 app-only 权限
                </el-text>
              </div>
              <div class="flex gap-2">
                <el-select
                  v-model="selectedPermission"
                  filterable
                  placeholder="选择要授予的权限"
                  class="min-w-0 flex-1"
                >
                  <el-option
                    v-for="code in appOnlyPermissionOptions"
                    :key="code"
                    :label="code"
                    :value="code"
                  />
                </el-select>
                <el-button type="primary" :disabled="!selectedPermission" @click="grantClientPermission">
                  <el-icon><Lock /></el-icon>
                  授予
                </el-button>
              </div>
            </div>
          </div>
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Delete, Key, Lock, Plus, Refresh } from '@element-plus/icons-vue'
import {
  clearOAuthClientPermission,
  createOAuthApp,
  deleteOAuthApp,
  getOAuthClientPermissions,
  getPermissionCatalog,
  listOAuthApps,
  rotateOAuthSecret,
  setOAuthClientPermission,
  type OAuthClient,
  type OAuthClientPermissions,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import { getErrorMessage } from '@/utils/error'

const user = inject<Ref<User | null>>('user', ref(null))

const apps = ref<OAuthClient[]>([])
const catalog = ref<PermissionDefinition[]>([])
const selectedClientId = ref('')
const loading = ref(false)
const creating = ref(false)
const rotating = ref(false)
const deleting = ref(false)
const permissionLoading = ref(false)
const newSecret = ref('')
const clientPermissionInfo = ref<OAuthClientPermissions | null>(null)
const selectedPermission = ref('')

const form = reactive({
  name: '',
  description: '',
  redirect_uri: '',
  website_url: '',
  client_type: 'confidential' as 'public' | 'confidential',
  permissions: [] as string[],
})

const selectedApp = computed(() => apps.value.find((app) => app.client_id === selectedClientId.value) ?? null)
const delegablePermissions = computed(() =>
  catalog.value.filter((item) => item.scope !== 'system' && item.scope !== 'server'),
)
const appOnlyPermissionOptions = computed(() => clientPermissionInfo.value?.session_allowed_scopes ?? [])
const canManageClientPermissions = computed(() => {
  const permissions = user.value?.permissions ?? []
  return permissions.includes('permission.read.any') && permissions.includes('permission.grant.any')
})

onMounted(async () => {
  await Promise.all([loadCatalog(), loadApps()])
})

async function loadCatalog() {
  const res = await getPermissionCatalog()
  catalog.value = res.data.permissions
}

async function loadApps() {
  loading.value = true
  try {
    const res = await listOAuthApps()
    apps.value = res.data.items
    if (!selectedClientId.value && apps.value[0]) selectApp(apps.value[0].client_id)
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载应用失败'))
  } finally {
    loading.value = false
  }
}

async function selectApp(clientId: string) {
  selectedClientId.value = clientId
  newSecret.value = ''
  clientPermissionInfo.value = null
  if (canManageClientPermissions.value) await loadClientPermissions()
}

async function createApp() {
  if (!form.name || !form.redirect_uri || form.permissions.length === 0) {
    ElMessage.warning('请填写名称、回调地址并选择至少一个权限')
    return
  }
  creating.value = true
  try {
    const res = await createOAuthApp({ ...form })
    apps.value.unshift(res.data)
    selectedClientId.value = res.data.client_id
    newSecret.value = res.data.client_secret ?? ''
    form.name = ''
    form.description = ''
    form.redirect_uri = ''
    form.website_url = ''
    form.permissions = []
    ElMessage.success('应用已创建')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '创建应用失败'))
  } finally {
    creating.value = false
  }
}

async function rotateSecret() {
  if (!selectedApp.value) return
  rotating.value = true
  try {
    const res = await rotateOAuthSecret(selectedApp.value.client_id)
    newSecret.value = res.data.client_secret ?? ''
    ElMessage.success('密钥已轮换')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '轮换失败'))
  } finally {
    rotating.value = false
  }
}

async function deleteSelected() {
  if (!selectedApp.value) return
  await ElMessageBox.confirm('删除后应用将无法继续完成 OAuth 授权，确认删除？', '删除应用')
  deleting.value = true
  try {
    await deleteOAuthApp(selectedApp.value.client_id)
    apps.value = apps.value.filter((app) => app.client_id !== selectedClientId.value)
    selectedClientId.value = apps.value[0]?.client_id ?? ''
    ElMessage.success('应用已删除')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '删除失败'))
  } finally {
    deleting.value = false
  }
}

async function loadClientPermissions() {
  if (!selectedApp.value) return
  permissionLoading.value = true
  try {
    const res = await getOAuthClientPermissions(selectedApp.value.client_id)
    clientPermissionInfo.value = res.data
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载 app-only 权限失败'))
  } finally {
    permissionLoading.value = false
  }
}

async function grantClientPermission() {
  if (!selectedApp.value || !selectedPermission.value) return
  await setOAuthClientPermission(selectedApp.value.client_id, selectedPermission.value, 'allow')
  selectedPermission.value = ''
  await loadClientPermissions()
}

async function clearClientPermission(code: string) {
  if (!selectedApp.value) return
  await clearOAuthClientPermission(selectedApp.value.client_id, code)
  await loadClientPermissions()
}

async function copyText(text: string) {
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

function shortPermission(code: string) {
  const parts = code.split('.')
  return parts.length === 3 ? `${parts[0]}.${parts[1]}.${parts[2]}` : code
}
</script>
