<template>
  <div class="settings-section">
    <div class="section-header">
      <h2>站点设置</h2>
      <el-button type="primary" @click="loadSettings">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-card class="settings-card">
      <el-form 
        :label-width="labelPosition === 'top' ? 'auto' : '140px'" 
        :model="siteSettings" 
        :label-position="labelPosition"
      >
        <el-form-item label="站点名称">
          <el-input v-model="siteSettings.site_name" placeholder="皮肤站" />
        </el-form-item>
        <el-form-item label="后端 API 地址">
          <el-input v-model="siteSettings.site_url" placeholder="https://skin.example.com" />
        </el-form-item>
        <el-form-item label="需要邀请码注册">
          <el-switch v-model="siteSettings.require_invite" />
        </el-form-item>
        <el-form-item label="允许用户注册">
          <el-switch v-model="siteSettings.allow_register" />
        </el-form-item>
        <el-form-item label="启用公共皮肤库">
          <el-switch v-model="siteSettings.enable_skin_library" />
        </el-form-item>
        <el-form-item label="最大纹理大小">
          <el-input v-model="siteSettings.max_texture_size" type="number">
            <template #suffix>KB</template>
          </el-input>
        </el-form-item>

        <el-divider content-position="left">安全设置</el-divider>
        <el-form-item label="启用强密码检查">
          <el-switch v-model="siteSettings.enable_strong_password_check" />
        </el-form-item>
        <el-form-item label="启用速率限制">
          <el-switch v-model="siteSettings.rate_limit_enabled" />
          <el-text size="small" type="info" style="margin-left:12px;">
            限制单位时间内的请求次数
          </el-text>
        </el-form-item>

        <!-- <el-form-item label="启用强密码检测">
          <el-switch v-model="siteSettings.password_strength_enabled" />
          <el-text size="small" type="info" style="margin-left:12px;">
            关闭后仅保留最少6位长度限制
          </el-text>
        </el-form-item> -->

        <el-form-item label="登录失败限制" v-if="siteSettings.rate_limit_enabled">
          <el-input v-model="siteSettings.rate_limit_auth_attempts" type="number">
            <template #suffix>次</template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">每个时间窗口内允许的最大尝试次数</el-text>
        </el-form-item>

        <el-form-item label="时间窗口" v-if="siteSettings.rate_limit_enabled">
          <el-input v-model="siteSettings.rate_limit_auth_window" type="number">
            <template #suffix>分钟</template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">超限后需等待的时间</el-text>
        </el-form-item>

        <el-divider content-position="left">JWT 认证设置</el-divider>

        <el-form-item label="JWT 过期时间">
          <el-input v-model="siteSettings.jwt_expire_days" type="number">
            <template #suffix>天</template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">用户登录后 Token 的有效期</el-text>
        </el-form-item>
        <el-divider content-position="left">微软正版登录设置</el-divider>
        <el-form-item label="Client ID">
          <el-input v-model="siteSettings.microsoft_client_id" placeholder="Azure 应用的 Client ID">
            <template #prepend>
              <el-icon><Key /></el-icon>
            </template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">
            在 <el-link href="https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" target="_blank" type="primary">Azure Portal</el-link> 创建应用获取
          </el-text>
        </el-form-item>
        <el-form-item label="Client Secret">
          <el-input
            v-model="siteSettings.microsoft_client_secret"
            placeholder="Azure 应用的 Client Secret（必填）"
            type="password"
            show-password
          >
            <template #prepend>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">授权码模式需要 Client Secret</el-text>
        </el-form-item>
        <el-form-item label="Redirect URI">
          <el-input v-model="siteSettings.microsoft_redirect_uri" placeholder="OAuth 回调地址（如：http://localhost:8000/microsoft/callback）">
            <template #prepend>
              <el-icon><Link /></el-icon>
            </template>
          </el-input>
          <el-text size="small" type="info" style="margin-top:4px">必须与 Azure 应用配置中的重定向 URI 完全一致</el-text>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" @click="saveSettings" size="large">
            <el-icon><Check /></el-icon>
            保存设置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { Refresh, Check, Key, Lock, Link } from '@element-plus/icons-vue'

const siteSettings = ref({
  site_name: '皮肤站',
  site_url: '',
  require_invite: false,
  allow_register: true,
  enable_skin_library: true,
  max_texture_size: 1024,
  rate_limit_enabled: true,
  rate_limit_auth_attempts: 5,
  rate_limit_auth_window: 15,
  jwt_expire_days: 7,
  // password_strength_enabled: true,
  microsoft_client_id: '',
  microsoft_client_secret: '',
  microsoft_redirect_uri: '',
  enable_strong_password_check: false
})

const windowWidth = ref(window.innerWidth)
const labelPosition = computed(() => windowWidth.value < 992 ? 'top' : 'right')

function handleResize() {
  windowWidth.value = window.innerWidth
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function loadSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers: authHeaders() })
    if (res.data) {
      Object.assign(siteSettings.value, res.data)
    }
  } catch (e) {
    console.error('Load settings error:', e)
    ElMessage.error('加载设置失败')
  }
}

async function saveSettings() {
  try {
    await axios.post('/admin/settings', siteSettings.value, { headers: authHeaders() })
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

onMounted(() => {
  loadSettings()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.settings-section {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.settings-card {
  width: 100%;
  max-width: 800px;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
}

/* Restrict form width on large screens to avoid super long inputs */
.settings-card .el-form {
  max-width: 600px;
  margin: 0 auto;
}

.settings-card :deep(.el-form-item__label) {
  color: var(--color-text);
}

.settings-card :deep(.el-divider__text) {
  background-color: var(--color-card-background);
  color: var(--color-heading);
}

.settings-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}

.settings-card :deep(.el-button) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-button:hover) {
  transform: scale(1.05);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.3);
}

@media (max-width: 768px) {
  .settings-card {
    padding: 15px;
  }
}
</style>