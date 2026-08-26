<template>
  <div class="mx-auto max-w-[1000px] space-y-6 py-5 animate-fade-in">
    <PageHeader
      :title="providerId ? '编辑身份提供方' : '添加身份提供方'"
      subtitle="配置连接方式、客户端凭据以及站点开放的身份能力"
    >
      <template #icon><Connection /></template>
      <template #actions>
        <ActionBar>
          <el-button :icon="ArrowLeft" @click="backToList">返回列表</el-button>
        </ActionBar>
      </template>
    </PageHeader>

    <OidcRedirectUriNotice :uri="redirectUri" />

    <div v-loading="loading" class="min-h-[360px] space-y-6">
      <template v-if="!loading">
        <UiCard class="p-6">
          <div class="mb-5">
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">连接配置</h2>
            <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
              {{
                form.adapter === 'qq'
                  ? '保存时后端使用内置的 QQ 互联端点，不会请求 Discovery 文档。'
                  : '保存时后端会读取并验证 Issuer 的 OIDC Discovery 文档。'
              }}
            </p>
          </div>

          <el-form label-position="top" :model="form">
            <div class="grid gap-x-4 md:grid-cols-2">
              <el-form-item label="显示名称" required>
                <el-input v-model="form.name" placeholder="例如 Microsoft" />
              </el-form-item>
              <el-form-item label="适配器" required>
                <el-select v-model="form.adapter" class="w-full" @change="applyAdapterDefaults">
                  <el-option
                    v-for="option in identityProviderAdapterOptions()"
                    :key="option.value"
                    :label="option.label"
                    :value="option.value"
                  />
                </el-select>
              </el-form-item>
            </div>

            <template v-if="form.adapter === 'qq'">
              <el-alert
                class="mb-5"
                type="info"
                :closable="false"
                show-icon
                title="QQ 互联端点由系统内置"
                description="无需填写 Issuer URL 和 Scopes。请在 QQ 互联管理中心创建网站应用，在应用设置中把 Redirect URI 登记为网站回调域，填入获得的 APP ID 和 APP Key。"
              />
            </template>
            <template v-else>
              <el-form-item label="Issuer URL" required>
                <el-input v-model="form.issuer_url" placeholder="https://issuer.example" />
              </el-form-item>
              <el-alert
                v-if="form.adapter === 'microsoft'"
                class="mb-5"
                type="info"
                :closable="false"
                show-icon
                title="Microsoft 正版登录使用个人账户租户"
                description="Entra 应用必须允许个人 Microsoft 账户；推荐的 Supported account types 是“任何组织目录中的帐户和个人 Microsoft 帐户”。"
              />
            </template>

            <div class="grid gap-x-4 md:grid-cols-2">
              <el-form-item :label="form.adapter === 'qq' ? 'APP ID（Client ID）' : 'Client ID'" required>
                <el-input v-model="form.client_id" />
              </el-form-item>
              <el-form-item
                :label="
                  form.adapter === 'qq'
                    ? providerId
                      ? 'APP Key（留空保持不变）'
                      : 'APP Key'
                    : providerId
                      ? 'Client Secret（留空保持不变）'
                      : 'Client Secret'
                "
                :required="!providerId"
              >
                <el-input v-model="form.client_secret" type="password" show-password />
              </el-form-item>
            </div>

            <el-form-item v-if="form.adapter !== 'qq'" label="Scopes" required>
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

            <div class="grid gap-x-4 md:grid-cols-2">
              <el-form-item label="图标 URL">
                <el-input v-model="form.icon_url" placeholder="https://example.com/icon.svg" />
              </el-form-item>
              <el-form-item label="显示顺序">
                <el-input-number
                  v-model="form.display_order"
                  class="!w-full"
                  :min="-10000"
                  :max="10000"
                />
              </el-form-item>
            </div>
          </el-form>
        </UiCard>

        <UiCard class="p-6">
          <div class="mb-5">
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">开放能力</h2>
            <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
              登录只面向已绑定的外部身份；未绑定的外部登录会引导用户先账号密码登录，再到控制台绑定。
            </p>
          </div>
          <div class="grid gap-4 md:grid-cols-3">
            <UiOptionCard as="label">
              <div class="w-full">
                <div class="flex items-center justify-between gap-3">
                  <span class="font-medium text-[var(--color-heading)]">提供方状态</span>
                  <el-switch v-model="form.enabled" />
                </div>
                <div class="mt-2 text-xs text-[var(--color-text-light)]">
                  关闭后不再展示或接受授权。
                </div>
              </div>
            </UiOptionCard>
            <UiOptionCard as="label">
              <div class="w-full">
                <div class="flex items-center justify-between gap-3">
                  <span class="font-medium text-[var(--color-heading)]">允许登录</span>
                  <el-switch v-model="form.login_enabled" />
                </div>
                <div class="mt-2 text-xs text-[var(--color-text-light)]">
                  允许已绑定的外部身份登录本站。
                </div>
              </div>
            </UiOptionCard>
            <UiOptionCard as="label">
              <div class="w-full">
                <div class="flex items-center justify-between gap-3">
                  <span class="font-medium text-[var(--color-heading)]">允许绑定</span>
                  <el-switch v-model="form.link_enabled" />
                </div>
                <div class="mt-2 text-xs text-[var(--color-text-light)]">
                  允许已有用户连接或重连身份。
                </div>
              </div>
            </UiOptionCard>
          </div>
        </UiCard>

        <UiCard class="p-5">
          <ActionBar>
            <el-button :disabled="saving" @click="backToList">取消</el-button>
            <el-button type="primary" :loading="saving" @click="saveProvider">
              {{ providerId ? '保存修改' : '添加提供方' }}
            </el-button>
          </ActionBar>
        </UiCard>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Connection } from '@element-plus/icons-vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createIdentityProvider,
  getAdminIdentityProvider,
  updateIdentityProvider,
} from '@/api/admin/identity-providers'
import { getIdentityProviders } from '@/api/identity'
import ActionBar from '@/components/common/ActionBar.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiOptionCard from '@/components/ui/UiOptionCard.vue'
import { getErrorMessage } from '@/utils/error'
import {
  identityProviderAdapterOptions,
} from '@/utils/identityAdapter'
import {
  applyIdentityProviderAdapterDefaults,
  emptyIdentityProviderForm,
  identityProviderPayload,
  identityProviderValidationError,
  type IdentityProviderFormState,
} from './identityProviderFormState'
import OidcRedirectUriNotice from './OidcRedirectUriNotice.vue'

const route = useRoute()
const router = useRouter()
const providerId = computed(() => String(route.params.provider_id || ''))
const loading = ref(true)
const saving = ref(false)
const redirectUri = ref('')
const form = reactive<IdentityProviderFormState>(emptyIdentityProviderForm())

onMounted(load)

async function load() {
  loading.value = true
  try {
    const publicProviders = await getIdentityProviders()
    redirectUri.value = publicProviders.data.redirect_uri
    if (!providerId.value) return
    const response = await getAdminIdentityProvider(providerId.value)
    const provider = response.data
    Object.assign(form, {
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
      display_order: provider.display_order,
    })
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载提供方配置失败'))
    await router.push({ name: 'admin-identity-providers' })
  } finally {
    loading.value = false
  }
}

function applyAdapterDefaults() {
  applyIdentityProviderAdapterDefaults(form)
}

function validateForm() {
  const detail = identityProviderValidationError(form, !providerId.value)
  if (!detail) return true
  ElMessage.warning(detail)
  return false
}

async function saveProvider() {
  if (!validateForm()) return
  saving.value = true
  try {
    if (providerId.value) {
      await updateIdentityProvider(providerId.value, identityProviderPayload(form))
      ElMessage.success('提供方已更新')
      return
    }
    const response = await createIdentityProvider(identityProviderPayload(form))
    ElMessage.success('提供方已添加')
    form.client_secret = ''
    await router.replace({
      name: 'admin-identity-provider-edit',
      params: { provider_id: response.data.id },
    })
  } catch (error) {
    ElMessage.error(getErrorMessage(error, providerId.value ? '更新提供方失败' : '添加提供方失败'))
  } finally {
    saving.value = false
  }
}

function backToList() {
  void router.push({ name: 'admin-identity-providers' })
}
</script>
