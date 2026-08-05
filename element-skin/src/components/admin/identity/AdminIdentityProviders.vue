<template>
  <div class="max-w-[1000px] mx-auto py-5 animate-fade-in">
    <PageHeader title="OIDC 身份提供方" subtitle="配置允许用户登录和绑定的外部 OIDC 端点">
      <template #icon><Connection /></template>
      <template #actions>
        <el-button v-if="canCreate" type="primary" :icon="Plus" @click="openCreate">
          添加提供方
        </el-button>
      </template>
    </PageHeader>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      class="mb-6"
      title="Microsoft 也是普通 OIDC 身份提供方"
      description="选择 Microsoft 适配器只会开放本站的正版角色能力；身份登录、绑定、重新授权和令牌存储仍走同一套 OIDC 路径。"
    />

    <div v-loading="loading" class="min-h-[220px]">
      <div v-if="providers.length" class="grid gap-4">
        <UiCard v-for="provider in providers" :key="provider.id" hoverable>
          <div class="flex flex-col md:flex-row md:items-start gap-4">
            <el-avatar :size="48" :src="provider.icon_url || undefined">
              {{ provider.name.charAt(0).toUpperCase() }}
            </el-avatar>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold text-lg text-[var(--color-heading)]">
                  {{ provider.name }}
                </span>
                <el-tag :type="provider.enabled ? 'success' : 'info'">
                  {{ provider.enabled ? '已启用' : '已停用' }}
                </el-tag>
                <el-tag v-if="provider.adapter === 'microsoft'" type="success">Microsoft</el-tag>
                <el-tag v-else>通用 OIDC</el-tag>
              </div>
              <div class="mt-2 text-sm text-[var(--color-text-light)] break-all">
                {{ provider.issuer_url }}
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <el-tag v-if="provider.login_enabled" size="small">登录</el-tag>
                <el-tag v-if="provider.link_enabled" size="small">绑定</el-tag>
                <el-tag v-if="provider.registration_enabled" size="small">注册衔接</el-tag>
                <el-tag v-for="scope in provider.scopes" :key="scope" size="small" type="info">
                  {{ scope }}
                </el-tag>
              </div>
            </div>
            <div class="flex gap-2">
              <el-button v-if="canUpdate" @click="openEdit(provider)">编辑</el-button>
              <el-button v-if="canDelete" type="danger" plain @click="removeProvider(provider)">
                删除
              </el-button>
            </div>
          </div>
        </UiCard>
      </div>
      <el-empty v-else-if="!loading && canRead" description="尚未配置外部 OIDC 提供方" />
    </div>

    <UiDialog
      v-model="dialogVisible"
      :title="editingId ? '编辑 OIDC 提供方' : '添加 OIDC 提供方'"
      width="680px"
      :close-on-click-modal="false"
    >
      <el-form label-position="top" :model="form">
        <div class="grid md:grid-cols-2 gap-x-4">
          <el-form-item label="显示名称" required>
            <el-input v-model="form.name" placeholder="例如 Microsoft" />
          </el-form-item>
          <el-form-item label="适配器" required>
            <el-select v-model="form.adapter" class="w-full" @change="applyAdapterDefaults">
              <el-option label="通用 OIDC" value="generic_oidc" />
              <el-option label="Microsoft（启用正版能力）" value="microsoft" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="Issuer URL" required>
          <el-input v-model="form.issuer_url" placeholder="https://issuer.example" />
          <div class="form-tip">保存时服务端会读取并校验 OIDC Discovery 文档。</div>
        </el-form-item>
        <el-form-item label="Client ID" required>
          <el-input v-model="form.client_id" />
        </el-form-item>
        <el-form-item :label="editingId ? 'Client Secret（留空保持不变）' : 'Client Secret'">
          <el-input v-model="form.client_secret" type="password" show-password />
        </el-form-item>
        <el-form-item label="Scopes" required>
          <el-input
            v-model="form.scopes"
            type="textarea"
            :rows="3"
            placeholder="openid profile email"
          />
          <div class="form-tip">
            使用空格或换行分隔。Microsoft 适配器必须包含 XboxLive.signin 和 offline_access。
          </div>
        </el-form-item>
        <el-form-item label="图标 URL">
          <el-input v-model="form.icon_url" placeholder="https://example.com/icon.svg" />
        </el-form-item>
        <el-form-item label="显示顺序">
          <el-input-number v-model="form.display_order" :min="-10000" :max="10000" />
        </el-form-item>
        <div class="grid sm:grid-cols-2 gap-x-4">
          <el-form-item label="提供方状态"
            ><el-switch v-model="form.enabled" active-text="启用"
          /></el-form-item>
          <el-form-item label="允许登录"><el-switch v-model="form.login_enabled" /></el-form-item>
          <el-form-item label="允许绑定"><el-switch v-model="form.link_enabled" /></el-form-item>
          <el-form-item label="允许衔接新用户注册">
            <el-switch v-model="form.registration_enabled" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <el-button :disabled="saving" @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveProvider">保存</el-button>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Plus } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import {
  createIdentityProvider,
  deleteIdentityProvider,
  getAdminIdentityProviders,
  updateIdentityProvider,
  type IdentityProviderInput,
} from '@/api/admin/identity-providers'
import type { AdminIdentityProvider, User } from '@/api/types'
import { getErrorMessage } from '@/utils/error'

interface ProviderForm {
  name: string
  issuer_url: string
  client_id: string
  client_secret: string
  scopes: string
  adapter: 'generic_oidc' | 'microsoft'
  icon_url: string
  enabled: boolean
  login_enabled: boolean
  link_enabled: boolean
  registration_enabled: boolean
  display_order: number
}

const user = inject<Ref<User | null>>('user', ref(null))
const providers = ref<AdminIdentityProvider[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref('')
const permissionSet = computed(() => new Set(user.value?.permissions || []))
const canRead = computed(() => permissionSet.value.has('identity_provider.read.any'))
const canCreate = computed(() => permissionSet.value.has('identity_provider.create.any'))
const canUpdate = computed(() => permissionSet.value.has('identity_provider.update.any'))
const canDelete = computed(() => permissionSet.value.has('identity_provider.delete.any'))

const form = reactive<ProviderForm>(emptyForm())

function emptyForm(): ProviderForm {
  return {
    name: '',
    issuer_url: '',
    client_id: '',
    client_secret: '',
    scopes: 'openid profile email',
    adapter: 'generic_oidc',
    icon_url: '',
    enabled: true,
    login_enabled: true,
    link_enabled: true,
    registration_enabled: true,
    display_order: 0,
  }
}

function replaceForm(value: ProviderForm) {
  Object.assign(form, value)
}

function applyAdapterDefaults() {
  if (form.adapter !== 'microsoft') return
  const scopes = new Set(form.scopes.split(/\s+/).filter(Boolean))
  for (const scope of ['openid', 'profile', 'email', 'XboxLive.signin', 'offline_access']) {
    scopes.add(scope)
  }
  form.scopes = [...scopes].join(' ')
}

function openCreate() {
  editingId.value = ''
  replaceForm(emptyForm())
  dialogVisible.value = true
}

function openEdit(provider: AdminIdentityProvider) {
  editingId.value = provider.id
  replaceForm({
    name: provider.name,
    issuer_url: provider.issuer_url,
    client_id: provider.client_id,
    client_secret: '',
    scopes: provider.scopes.join(' '),
    adapter: provider.adapter,
    icon_url: provider.icon_url,
    enabled: provider.enabled,
    login_enabled: provider.login_enabled,
    link_enabled: provider.link_enabled,
    registration_enabled: provider.registration_enabled,
    display_order: provider.display_order,
  })
  dialogVisible.value = true
}

function payload(): IdentityProviderInput {
  const result: IdentityProviderInput = {
    name: form.name.trim(),
    issuer_url: form.issuer_url.trim(),
    client_id: form.client_id.trim(),
    scopes: form.scopes.split(/\s+/).filter(Boolean),
    adapter: form.adapter,
    icon_url: form.icon_url.trim(),
    enabled: form.enabled,
    login_enabled: form.login_enabled,
    link_enabled: form.link_enabled,
    registration_enabled: form.registration_enabled,
    display_order: form.display_order,
  }
  if (form.client_secret) result.client_secret = form.client_secret
  return result
}

async function loadProviders() {
  if (!canRead.value) return
  loading.value = true
  try {
    providers.value = (await getAdminIdentityProviders()).data.items
  } catch (e: unknown) {
    ElMessage.error('加载提供方失败: ' + getErrorMessage(e, '加载失败'))
  } finally {
    loading.value = false
  }
}

async function saveProvider() {
  if (!form.name.trim() || !form.issuer_url.trim() || !form.client_id.trim()) {
    ElMessage.warning('请填写名称、Issuer URL 和 Client ID')
    return
  }
  try {
    saving.value = true
    if (editingId.value) await updateIdentityProvider(editingId.value, payload())
    else await createIdentityProvider(payload())
    dialogVisible.value = false
    ElMessage.success(editingId.value ? '提供方已更新' : '提供方已添加')
    await loadProviders()
  } catch (e: unknown) {
    ElMessage.error('保存失败: ' + getErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function removeProvider(provider: AdminIdentityProvider) {
  try {
    await ElMessageBox.confirm(
      '只有未被任何外部身份引用的提供方才能删除。也可以先停用它。',
      `删除 ${provider.name}`,
      { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
    )
    await deleteIdentityProvider(provider.id)
    ElMessage.success('提供方已删除')
    await loadProviders()
  } catch (e: unknown) {
    if (e !== 'cancel' && e !== 'close') {
      ElMessage.error('删除失败: ' + getErrorMessage(e, '删除失败'))
    }
  }
}

onMounted(loadProviders)
</script>
