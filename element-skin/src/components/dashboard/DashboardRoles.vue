<template>
  <div class="roles-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>角色管理</h1>
          <p>创建并管理您的 Minecraft 角色身份</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button size="large" @click="showYggImportDialog = true" class="btn-gradient btn-gradient-warning">
          <el-icon><Download /></el-icon>
          <span style="margin-left:8px">导入皮肤站角色</span>
        </el-button>
        <el-button size="large" @click="startMicrosoftAuth" class="btn-gradient btn-gradient-success">
          <el-icon><Connection /></el-icon>
          <span style="margin-left:8px">绑定正版角色</span>
        </el-button>
        <el-button size="large" @click="showCreateRoleDialog = true" class="btn-gradient btn-gradient-primary">
          <el-icon><Plus /></el-icon>
          <span style="margin-left:8px">新建角色</span>
        </el-button>
      </div>
    </div>

    <div class="auto-grid">
      <div 
        v-for="(profile, index) in profiles" 
        :key="profile.id" 
        class="surface-card hoverable animate-card-slide clickable-card" 
        :style="{ '--delay-index': index % limit }"
        @click="openPreviewDialog(profile)"
      >
        <div
          class="role-preview"
          :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
        >
          <SkinViewer
            v-if="profile.skin_hash"
            :skinUrl="texturesUrl(profile.skin_hash)"
            :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
            :model="profile.model || 'default'"
            :width="200"
            :height="280"
            is-static
          />
          <el-empty v-else description="未设置皮肤" :image-size="120" />
        </div>
        <div class="role-info">
          <div class="role-name">{{ profile.name }}</div>
          <div class="role-model">模型: {{ profile.model || 'default' }}</div>
        </div>
        <div class="role-actions" @click.stop>
          <el-button
            class="btn-gradient btn-gradient-danger btn-icon-swap"
            @click="deleteRole(profile.id)"
            size="default"
          >
            <span class="btn-label">删除</span>
            <el-icon class="btn-icon"><Delete /></el-icon>
          </el-button>

          <el-button
            v-if="profile.skin_hash"
            class="btn-soft-warning btn-icon-swap"
            @click="clearRoleSkin(profile.id)"
            size="default"
          >
            <span class="btn-label">皮肤</span>
            <el-icon class="btn-icon"><Close /></el-icon>
          </el-button>

          <el-button
            v-if="profile.cape_hash"
            class="btn-soft-warning btn-icon-swap"
            @click="clearRoleCape(profile.id)"
            size="default"
          >
            <span class="btn-label">披风</span>
            <el-icon class="btn-icon"><Close /></el-icon>
          </el-button>
        </div>
      </div>
    </div>

    <div class="pagination-container">
      <el-pagination
        background
        layout="prev, pager, next"
        :total="total"
        :page-size="limit"
        v-model:current-page="currentPage"
        @current-change="handlePageChange"
      />
    </div>

    <!-- 预览对话框 -->
    <el-dialog
      v-model="showPreviewDialog"
      width="800px"
      destroy-on-close
      class="dialog-viewer"
      append-to-body
    >
      <div class="viewer-layout" v-if="selectedProfile">
        <div class="viewer-stage">
          <SkinViewer
            v-if="selectedProfile.skin_hash"
            :skinUrl="texturesUrl(selectedProfile.skin_hash)"
            :capeUrl="selectedProfile.cape_hash ? texturesUrl(selectedProfile.cape_hash) : null"
            :model="selectedProfile.model || 'default'"
            :width="320"
            :height="430"
          />
          <el-empty v-else description="未设置皮肤" />
        </div>

        <div class="viewer-info-panel">
          <section class="viewer-section title-section">
            <div class="viewer-title-row">
              <el-button text circle class="title-edit-btn" @click="focusNameInput">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-input
                ref="nameInputRef"
                v-model="selectedProfile.name"
                class="viewer-title-input"
                placeholder="角色名称"
                @change="updateRoleName"
              />
            </div>
          </section>

          <section class="viewer-section meta-section">
            <div class="viewer-section-label">角色信息</div>
            <div class="viewer-title-row">
              <span class="meta-chip">模型: {{ selectedProfile.model || 'default' }}</span>
            </div>
            <div class="hash-label">UUID: {{ formatUUID(selectedProfile.id) }}</div>
            <div class="hash-label" v-if="selectedProfile.skin_hash">皮肤 HASH: {{ selectedProfile.skin_hash }}</div>
            <div class="hash-label" v-if="selectedProfile.cape_hash">披风 HASH: {{ selectedProfile.cape_hash }}</div>
          </section>

          <section class="viewer-section" v-if="selectedProfile.skin_hash || selectedProfile.cape_hash">
            <div class="viewer-section-label">快捷操作</div>
            <div class="apply-row" style="display: flex; gap: 8px;">
              <el-button 
                v-if="selectedProfile.skin_hash"
                type="warning" 
                plain 
                style="flex: 1; border-radius: 8px;"
                @click="clearRoleSkin(selectedProfile.id)"
              >
                清除皮肤
              </el-button>
              <el-button 
                v-if="selectedProfile.cape_hash"
                type="warning" 
                plain 
                style="flex: 1; border-radius: 8px;"
                @click="clearRoleCape(selectedProfile.id)"
              >
                清除披风
              </el-button>
            </div>
          </section>

          <section class="viewer-section footer-section" style="margin-top: auto;">
             <el-button 
              type="danger" 
              plain 
              style="width: 100%; border-radius: 8px;"
              @click="deleteRole(selectedProfile.id)"
            >
              删除此角色
            </el-button>
          </section>
        </div>
      </div>
    </el-dialog>

    <!-- 新建角色对话框 -->
    <el-dialog v-model="showCreateRoleDialog" title="新建角色" width="420px" append-to-body>
      <el-form label-width="100px">
        <el-form-item label="角色名称">
          <el-input v-model="newRoleName" placeholder="请输入角色名称" maxlength="32" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateRoleDialog = false">取消</el-button>
        <el-button type="primary" @click="createRole">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>

    <!-- 微软正版登录对话框 -->
    <el-dialog
      v-model="showMicrosoftLoginDialog"
      title="绑定正版角色"
      width="400px"
      :close-on-click-modal="false"
      :destroy-on-close="true"
      :before-close="handleMicrosoftDialogClose"
      append-to-body
    >
      <div class="microsoft-login-content">
        <!-- 步骤2: 选择角色 (已找到) -->
        <div v-if="microsoftStep === 'select-profile' && microsoftProfile" class="step-content">
          <div class="selection-item is-checked" style="cursor: default; pointer-events: none;">
            <div class="selection-info">
              <span class="title">{{ microsoftProfile?.name }}</span>
              <span class="subtitle">{{ formatUUID(microsoftProfile?.id || '') }}</span>
            </div>
            <div style="margin-left: auto;">
               <el-tag v-if="microsoftProfile?.has_game" type="success" effect="dark">
                  拥有游戏
               </el-tag>
               <el-tag v-else type="danger" effect="dark">
                  无游戏权限
               </el-tag>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="cancelMicrosoftLogin" :disabled="importing">取消</el-button>
          <el-button
            v-if="microsoftStep === 'select-profile'"
            type="primary"
            @click="importMicrosoftProfile"
            :loading="importing"
            :disabled="!microsoftProfile?.has_game"
          >
            确认导入
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 外部皮肤站角色导入对话框 -->
    <el-dialog 
      v-model="showYggImportDialog" 
      title="从外部皮肤站导入角色" 
      width="450px" 
      append-to-body
      :before-close="handleYggDialogClose"
    >
      <div v-if="yggStep === 'input'">
        <el-form label-position="top">
          <el-form-item label="Yggdrasil API 地址">
            <el-input v-model="yggApiUrl" placeholder="https://skin.example.com/api/yggdrasil" />
            <div class="form-tip">通常以 /api/yggdrasil 结尾</div>
          </el-form-item>
          <el-form-item label="用户名/邮箱">
            <el-input v-model="yggUsername" placeholder="外部皮肤站的登录用户名" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="yggPassword" type="password" show-password placeholder="外部皮肤站的登录密码" />
          </el-form-item>
        </el-form>
      </div>

      <div v-else-if="yggStep === 'select'">
        <p style="margin-bottom: 16px;">请选择要导入的角色：</p>
        <el-radio-group v-model="selectedYggProfile" class="selection-list">
          <el-radio 
            v-for="p in yggProfiles" 
            :key="p.id" 
            :label="p.id" 
            border 
            class="selection-item"
          >
            <div class="selection-info">
              <span class="title">{{ p.name }}</span>
              <span class="subtitle">{{ formatUUID(p.id) }}</span>
            </div>
          </el-radio>
        </el-radio-group>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="handleYggDialogClose" :disabled="yggLoading">取消</el-button>
          <el-button 
            v-if="yggStep === 'input'" 
            type="primary" 
            @click="getYggProfiles" 
            :loading="yggLoading"
          >
            下一步
          </el-button>
          <el-button 
            v-else 
            type="primary" 
            @click="importYggProfile" 
            :loading="yggLoading"
            :disabled="!selectedYggProfile"
          >
            确认导入
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, inject, computed } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Plus, Delete, Close, Check, Select, Warning, Download, Edit } from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'

// Inject shared state from AppLayout
const user = inject('user')
const fetchMe = inject('fetchMe')
const isDark = inject('isDark')

const router = useRouter()

const profiles = ref([])
const total = ref(0)
const currentPage = ref(1)
const limit = 12

const showCreateRoleDialog = ref(false)
const newRoleName = ref('')
const showMicrosoftLoginDialog = ref(false)
const microsoftStep = ref('select-profile')
const microsoftProfile = ref(null)
const importing = ref(false)

const showPreviewDialog = ref(false)
const selectedProfile = ref(null)
const nameInputRef = ref(null)

function focusNameInput() {
  nameInputRef.value?.focus()
}

const showYggImportDialog = ref(false)
const yggStep = ref('input')
const yggApiUrl = ref('')
const yggUsername = ref('')
const yggPassword = ref('')
const yggProfiles = ref([])
const selectedYggProfile = ref(null)
const yggLoading = ref(false)

function openPreviewDialog(profile) {
  selectedProfile.value = profile
  showPreviewDialog.value = true
}

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  const base = import.meta.env.BASE_URL
  return `${base}static/textures/${hash}.png`.replace(/\/+/g, '/')
}

async function fetchProfiles() {
  try {
    const params = {
      page: currentPage.value,
      limit: limit
    }
    const res = await axios.get('/me/profiles', { headers: authHeaders(), params })
    profiles.value = res.data.items
    total.value = res.data.total
  } catch (e) {
    ElMessage.error('加载角色失败')
  }
}

function handlePageChange(page) {
  currentPage.value = page
  fetchProfiles()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

async function createRole() {
  const name = (newRoleName.value || '').trim()
  if (!name) return ElMessage.error('请输入角色名称')
  try {
    await axios.post('/me/profiles', { name }, { headers: authHeaders() })
    newRoleName.value = ''
    showCreateRoleDialog.value = false
    ElMessage.success('创建成功')
    fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteRole(pid) {
  try {
    await axios.delete(`/me/profiles/${pid}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    showPreviewDialog.value = false
    fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

async function updateRoleName() {
  if (!selectedProfile.value) return
  const pid = selectedProfile.value.id
  const newName = (selectedProfile.value.name || '').trim()

  if (!newName) {
    ElMessage.error('角色名不能为空')
    return
  }

  try {
    await axios.patch(`/me/profiles/${pid}`, { name: newName }, { headers: authHeaders() })
    ElMessage.success('名称已修改')
    fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e) {
    ElMessage.error('修改失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function clearRoleSkin(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的皮肤吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/skin`, { headers: authHeaders() })
    ElMessage.success('皮肤已清除')
    showPreviewDialog.value = false
    fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function clearRoleCape(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的披风吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/cape`, { headers: authHeaders() })
    ElMessage.success('披风已清除')
    showPreviewDialog.value = false
    fetchProfiles()
    if (fetchMe) fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

// 微软正版登录相关函数
function formatUUID(uuid) {
  if (!uuid) return ''
  if (uuid.length === 32) {
    return `${uuid.slice(0, 8)}-${uuid.slice(8, 12)}-${uuid.slice(12, 16)}-${uuid.slice(16, 20)}-${uuid.slice(20)}`
  }
  return uuid
}

async function startMicrosoftAuth() {
  try {
    const response = await axios.get('/microsoft/auth-url', { headers: authHeaders() })
    const authUrl = response.data.auth_url
    sessionStorage.setItem('ms_auth_state', response.data.state)
    window.location.href = authUrl
  } catch (error) {
    ElMessage.error('启动微软登录失败: ' + (error.response?.data?.detail || error.message))
  }
}

async function importMicrosoftProfile() {
  if (!microsoftProfile.value) return

  try {
    importing.value = true
    // Do NOT switch step, just show loading on button

    const skinData = microsoftProfile.value.skins?.[0]
    const capeData = microsoftProfile.value.capes?.[0]

    const importData = {
      profile_id: microsoftProfile.value.id,
      profile_name: microsoftProfile.value.name,
      skin_url: skinData?.url || null,
      skin_variant: skinData?.variant || 'classic',
      cape_url: capeData?.url || null
    }

    await axios.post('/microsoft/import-profile', importData, { headers: authHeaders() })

    ElMessage.success('正版角色导入成功！')

    showMicrosoftLoginDialog.value = false
    // Delay clearing the profile slightly to allow transition, or just leave it since dialog is destroying anyway
    // But safely clearing it prevents state leak if reopened somehow without reload (unlikely but possible)
    setTimeout(() => {
        microsoftProfile.value = null
        microsoftStep.value = 'select-profile'
    }, 300)

    // Refresh data in background
    try {
      fetchProfiles()
      if (fetchMe) await fetchMe()
    } catch (e) {
      console.warn('Failed to refresh user profile:', e)
    }

  } catch (error) {
    ElMessage.error('导入失败: ' + (error.response?.data?.detail || error.message))
  } finally {
    importing.value = false
  }
}

function cancelMicrosoftLogin() {
  showMicrosoftLoginDialog.value = false
  microsoftStep.value = 'select-profile'
  microsoftProfile.value = null
  importing.value = false
}

function handleMicrosoftDialogClose(done) {
  if (importing.value) {
    return; // Prevent closing while importing
  }
  cancelMicrosoftLogin()
  if (done) done()
}

// Yggdrasil 相关函数
async function getYggProfiles() {
  if (!yggApiUrl.value || !yggUsername.value || !yggPassword.value) {
    return ElMessage.warning('请填写完整信息')
  }
  try {
    yggLoading.value = true
    const res = await axios.post('/remote-ygg/get-profiles', {
      api_url: yggApiUrl.value,
      username: yggUsername.value,
      password: yggPassword.value
    }, { headers: authHeaders() })
    
    yggProfiles.value = res.data.profiles
    if (yggProfiles.value.length === 0) {
      ElMessage.warning('该账户下没有角色')
    } else {
      yggStep.value = 'select'
      selectedYggProfile.value = yggProfiles.value[0].id
    }
  } catch (e) {
    ElMessage.error('获取失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    yggLoading.value = false
  }
}

async function importYggProfile() {
  if (!selectedYggProfile.value) return
  const profile = yggProfiles.value.find(p => p.id === selectedYggProfile.value)
  if (!profile) return

  try {
    yggLoading.value = true
    await axios.post('/remote-ygg/import-profile', {
      api_url: yggApiUrl.value,
      profile_id: profile.id,
      profile_name: profile.name
    }, { headers: authHeaders() })
    
    ElMessage.success('导入成功')
    showYggImportDialog.value = false
    fetchProfiles()
    if (fetchMe) fetchMe()
    resetYggImport()
  } catch (e) {
    ElMessage.error('导入失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    yggLoading.value = false
  }
}

function resetYggImport() {
  yggStep.value = 'input'
  yggApiUrl.value = ''
  yggUsername.value = ''
  yggPassword.value = ''
  yggProfiles.value = []
  selectedYggProfile.value = null
}

function handleYggDialogClose(done) {
  if (yggLoading.value) return
  resetYggImport()
  showYggImportDialog.value = false
  if (done && typeof done === 'function') done()
}

onMounted(async () => {
  fetchProfiles()
  const urlParams = new URLSearchParams(window.location.search)
  const msToken = urlParams.get('ms_token')
  const error = urlParams.get('error')

  if (error) {
    ElMessage.error('微软登录失败: ' + error)
    router.replace({ query: {} })
  } else if (msToken) {
    try {
      const response = await axios.post('/microsoft/get-profile',
        { ms_token: msToken },
        { headers: authHeaders() }
      )

      microsoftProfile.value = response.data.profile
      microsoftProfile.value.has_game = response.data.has_game
      microsoftStep.value = 'select-profile'
      showMicrosoftLoginDialog.value = true

      ElMessage.success('授权成功！')
    } catch (e) {
      ElMessage.error('获取角色信息失败: ' + (e.response?.data?.detail || e.message))
    }
    router.replace({ query: {} })
  }
})
</script>

<style>
/* Global Styles for Teleported Elements */
@import "@/assets/styles/dialogs.css";
@import "@/assets/styles/item-viewer.css";
</style>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/headers.css";
@import "@/assets/styles/buttons.css";
@import "@/assets/styles/cards.css";

.roles-section {
}

.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.role-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 8px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.role-actions {
  display: flex;
  flex-direction: row;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--color-border);
  background: var(--color-background-soft);
  align-items: center;
}

.role-actions .el-button {
  flex: 1;
  min-width: 0;
}

.clickable-card {
  cursor: pointer;
}

/* Microsoft Login Specific Styles */
.microsoft-login-content {
  padding: 10px 0;
}

.step-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}
</style>