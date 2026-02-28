<template>
  <div class="settings-section">
    <div class="page-header">
      <div class="header-content">
        <el-icon class="header-icon"><Setting /></el-icon>
        <div class="header-text">
          <h2>站点设置</h2>
          <p class="subtitle">管理站点基础配置、安全策略及第三方集成</p>
        </div>
      </div>
      <el-button type="primary" :icon="Refresh" @click="loadAllSettings" plain>
        重新加载所有
      </el-button>
    </div>

    <!-- Site Config -->
    <el-card class="modern-settings-card mb-6" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="title">
            <el-icon><Monitor /></el-icon>
            <span>基础设置</span>
          </div>
          <el-button type="primary" size="small" @click="saveGroup('site')" :loading="saving.site">保存</el-button>
        </div>
      </template>
      <el-form label-position="top" :model="settings.site">

        <el-row :gutter="20">
          <el-col :span="8">
            <el-form-item label="允许新用户注册">
              <el-switch v-model="settings.site.allow_register" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="强制邀请码">
              <el-switch v-model="settings.site.require_invite" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="启用公共皮肤库">
              <el-switch v-model="settings.site.enable_skin_library" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="最大纹理大小 (KB)">
          <el-input-number v-model="settings.site.max_texture_size" :min="64" :step="128" />
        </el-form-item>
        <el-divider />
                <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="站点名称">
              <el-input v-model="settings.site.site_name" placeholder="皮肤站" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="站点副标题">
              <el-input v-model="settings.site.site_subtitle" placeholder="简洁、高效、现代的 Minecraft 皮肤管理站" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="24">
            <el-form-item label="页脚附加信息">
              <el-input v-model="settings.site.footer_text" placeholder="Copyright © 2026 Element Skin" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="24">
            <el-collapse class="regulatory-section" v-model="regulatoryCollapse">
              <el-collapse-item title="监管信息" name="regulatory-info">
                <el-row :gutter="20">
                  <el-col :span="12">
                    <el-form-item label="ICP 备案信息">
                      <el-input v-model="settings.site.filing_icp" placeholder="留空则不展示" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="ICP 备案链接">
                      <el-input v-model="settings.site.filing_icp_link" placeholder="留空则不展示" />
                    </el-form-item>
                  </el-col>
                </el-row>
                <el-row :gutter="20">
                  <el-col :span="12">
                    <el-form-item label="公安备案信息">
                      <el-input v-model="settings.site.filing_mps" placeholder="留空则不展示" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="公安备案链接">
                      <el-input v-model="settings.site.filing_mps_link" placeholder="留空则不展示" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </el-collapse-item>
            </el-collapse>
          </el-col>
        </el-row>
      </el-form>
    </el-card>

    <!-- Security Config -->
    <el-card class="modern-settings-card mb-6" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="title">
            <el-icon><Lock /></el-icon>
            <span>安全与速率限制</span>
          </div>
          <el-button type="primary" size="small" @click="saveGroup('security')" :loading="saving.security">保存</el-button>
        </div>
      </template>
      <el-form label-position="top" :model="settings.security">
        <el-form-item label="强密码检查">
          <el-switch v-model="settings.security.enable_strong_password_check" />
          <span class="hint ml-4">启用后，用户注册或修改密码将执行严格的复杂度检查。</span>
        </el-form-item>
        <el-divider />
        <el-form-item label="身份验证速率限制">
          <el-switch v-model="settings.security.rate_limit_enabled" />
          <span class="hint ml-4">启用后将限制登录、重置密码等接口的尝试频率。</span>
        </el-form-item>
        <el-row :gutter="20" v-if="settings.security.rate_limit_enabled">
          <el-col :span="12">
            <el-form-item label="尝试次数上限">
              <el-input-number v-model="settings.security.rate_limit_auth_attempts" :min="1" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="时间窗口 (分钟)">
              <el-input-number v-model="settings.security.rate_limit_auth_window" :min="1" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </el-card>

    <!-- Auth / JWT Config -->
    <el-card class="modern-settings-card mb-6" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="title">
            <el-icon><Key /></el-icon>
            <span>令牌与认证 (JWT)</span>
          </div>
          <el-button type="primary" size="small" @click="saveGroup('auth')" :loading="saving.auth">保存</el-button>
        </div>
      </template>
      <el-form label-position="top" :model="settings.auth">
        <el-form-item label="令牌有效期 (天)">
          <el-input-number v-model="settings.auth.jwt_expire_days" :min="1" :max="365" />
          <p class="hint">用户登录后，其身份令牌将在该天数后失效。</p>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Microsoft Config -->
    <el-card class="modern-settings-card mb-6" shadow="never">
      <template #header>
        <div class="card-header">
          <div class="title">
            <el-icon><Link /></el-icon>
            <span>微软正版登录集成</span>
          </div>
          <el-button type="primary" size="small" @click="saveGroup('microsoft')" :loading="saving.microsoft">保存</el-button>
        </div>
      </template>
      <el-form label-position="top" :model="settings.microsoft">
        <el-form-item label="Azure Client ID">
          <el-input v-model="settings.microsoft.microsoft_client_id" placeholder="Azure AD 应用 ID" />
        </el-form-item>
        <el-form-item label="Azure Client Secret">
          <el-input v-model="settings.microsoft.microsoft_client_secret" type="password" show-password placeholder="保持空白以不修改" />
        </el-form-item>
        <el-form-item label="Redirect URI">
          <el-input v-model="settings.microsoft.microsoft_redirect_uri" placeholder="https://your-skin-site.com/auth/microsoft/callback" />
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { 
  Refresh, Check, Setting, Monitor, Lock, Key, Link 
} from '@element-plus/icons-vue'

const settings = reactive({
  site: {
    site_name: '',
    site_subtitle: '',
    require_invite: false,
    allow_register: true,
    enable_skin_library: true,
    max_texture_size: 1024,
    footer_text: '',
    filing_icp: '',
    filing_icp_link: '',
    filing_mps: '',
    filing_mps_link: ''
  },
  security: {
    rate_limit_enabled: true,
    rate_limit_auth_attempts: 5,
    rate_limit_auth_window: 15,
    enable_strong_password_check: false
  },
  auth: {
    jwt_expire_days: 7
  },
  microsoft: {
    microsoft_client_id: '',
    microsoft_client_secret: '',
    microsoft_redirect_uri: ''
  }
})

const saving = reactive({
  site: false,
  security: false,
  auth: false,
  microsoft: false
})

const regulatoryCollapse = ref([])

const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

async function loadGroup(group) {
  try {
    const res = await axios.get(`/admin/settings/${group}`, { headers: authHeaders() })
    Object.assign(settings[group], res.data)
  } catch (e) {
    ElMessage.error(`加载 ${group} 设置失败`)
  }
}

async function loadAllSettings() {
  await Promise.all([
    loadGroup('site'),
    loadGroup('security'),
    loadGroup('auth'),
    loadGroup('microsoft')
  ])
}

async function saveGroup(group) {
  saving[group] = true
  try {
    await axios.post(`/admin/settings/${group}`, settings[group], { headers: authHeaders() })
    ElMessage.success('设置已更新')
    if (group === 'microsoft') {
       settings.microsoft.microsoft_client_secret = '' // Clear local secret field
    }
  } catch (e) {
    ElMessage.error('保存失败')
  } finally {
    saving[group] = false
  }
}

onMounted(loadAllSettings)
</script>

<style scoped>
.settings-section {
  max-width: 900px;
  margin: 0 auto;
  padding: 20px 0;
  animation: fadeIn 0.4s ease-out;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 30px;
}
.header-content { display: flex; align-items: center; gap: 16px; }
.header-icon {
  font-size: 28px;
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  padding: 10px;
  border-radius: 10px;
}
.header-text h2 { margin: 0; font-size: 20px; font-weight: 600; }
.header-text .subtitle { margin: 4px 0 0 0; color: var(--color-text-light); font-size: 13px; }

.modern-settings-card {
  border: 1px solid var(--color-border);
  border-radius: 12px;
}
.card-header { display: flex; justify-content: space-between; align-items: center; }
.card-header .title { display: flex; align-items: center; gap: 8px; font-weight: 600; }

.hint { font-size: 12px; color: var(--color-text-light); line-height: 1.5; margin-top: 4px; display: block; }
.mb-6 { margin-bottom: 24px; }
.ml-4 { margin-left: 16px; }

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>