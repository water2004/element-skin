<template>
  <div class="invites-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><Ticket /></div>
        <div class="page-header-text">
          <h2>邀请码管理</h2>
          <p class="subtitle">创建并管理用于限制新用户注册的邀请码</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-button :icon="Refresh" @click="loadInvites" plain class="hover-lift">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="showInviteDialog" class="hover-lift">创建邀请码</el-button>
      </div>
    </div>

    <el-card class="surface-card" shadow="never">
      <el-table :data="invites" style="width: 100%" class="modern-table">
        <el-table-column prop="code" label="邀请码" min-width="180">
          <template #default="{ row }">
            <el-text copyable class="code-text">{{ row.code }}</el-text>
          </template>
        </el-table-column>
        <el-table-column label="可用次数" width="120" align="center">
          <template #default="{ row }">
            <div class="usage-tag" :style="{ backgroundColor: getRemainingBg(row), color: getRemainingColor(row) }">
              {{ row.used_count || 0 }} / {{ row.total_uses || '∞' }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="used_by" label="最后使用者" min-width="150">
          <template #default="{ row }">
            {{ row.used_by || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="note" label="备注说明" min-width="180">
          <template #default="{ row }">
            <span class="note-text">{{ row.note || '无' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="160">
          <template #default="{ row }">
            <span class="time-text">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              size="small"
              type="danger"
              :icon="Delete"
              @click="deleteInvite(row)"
              link
            />
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Create Invite Dialog -->
    <el-dialog
      v-model="inviteDialogVisible"
      title="创建新邀请码"
      width="500px"
      class="dialog-viewer"
    >
      <div style="padding: 24px">
        <el-form label-position="top">
          <el-form-item label="生成模式">
            <el-radio-group v-model="inviteMode" class="capsule-radio">
              <el-radio-button value="auto">自动随机</el-radio-button>
              <el-radio-button value="manual">手动指定</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item v-if="inviteMode === 'manual'" label="邀请码文本">
            <el-input v-model="customInviteCode" placeholder="6-32位 字母/数字/下划线" maxlength="32" show-word-limit />
          </el-form-item>

          <el-form-item v-else label="随机预览">
            <div class="code-preview-box">
              <span>{{ previewInviteCode }}</span>
              <el-button link :icon="Refresh" @click="refreshPreview">换一个</el-button>
            </div>
          </el-form-item>

          <el-divider />

          <el-form-item label="使用限制">
            <el-radio-group v-model="inviteUsesMode" class="mb-2">
              <el-radio value="limited">次数限制</el-radio>
              <el-radio value="unlimited">无限使用</el-radio>
            </el-radio-group>
            <el-input-number
              v-if="inviteUsesMode === 'limited'"
              v-model="inviteUses"
              :min="1"
              :max="1000"
              style="width: 100%"
            />
          </el-form-item>

          <el-form-item label="备注 (可选)">
            <el-input v-model="inviteNote" type="textarea" :rows="2" placeholder="填写该邀请码的用途..." />
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <div style="padding: 0 24px 24px">
          <el-button @click="inviteDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="confirmCreateInvite" :loading="creating" class="hover-lift">确认创建</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Check, Delete, Ticket } from '@element-plus/icons-vue'

const invites = ref([])
const inviteDialogVisible = ref(false)
const inviteMode = ref('auto')
const customInviteCode = ref('')
const previewInviteCode = ref('')
const creating = ref(false)
const inviteUsesMode = ref('limited')
const inviteUses = ref(1)
const inviteNote = ref('')

const authHeaders = () => ({ Authorization: 'Bearer ' + localStorage.getItem('jwt') })

function formatDate(ts) {
  return ts ? new Date(ts).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : '-'
}

async function loadInvites() {
  try {
    const res = await axios.get('/admin/invites', { headers: authHeaders() })
    invites.value = res.data
  } catch (e) {
    ElMessage.error('加载列表失败')
  }
}

function generateRandomCode() {
  return Math.random().toString(36).substring(2, 10).toUpperCase() + Math.random().toString(36).substring(2, 10).toUpperCase()
}

function showInviteDialog() {
  inviteMode.value = 'auto'
  customInviteCode.value = ''
  previewInviteCode.value = generateRandomCode()
  inviteUsesMode.value = 'limited'
  inviteUses.value = 1
  inviteNote.value = ''
  inviteDialogVisible.value = true
}

function refreshPreview() {
  previewInviteCode.value = generateRandomCode()
}

const getRemainingBg = (row) => {
  if (!row.total_uses) return 'rgba(103, 194, 58, 0.1)'
  const rem = row.total_uses - row.used_count
  if (rem <= 0) return 'rgba(245, 108, 108, 0.1)'
  if (rem <= row.total_uses * 0.2) return 'rgba(230, 162, 60, 0.1)'
  return 'rgba(64, 158, 255, 0.1)'
}

const getRemainingColor = (row) => {
  if (!row.total_uses) return 'var(--el-color-success)'
  const rem = row.total_uses - row.used_count
  if (rem <= 0) return 'var(--el-color-danger)'
  if (rem <= row.total_uses * 0.2) return 'var(--el-color-warning)'
  return 'var(--el-color-primary)'
}

async function confirmCreateInvite() {
  const code = inviteMode.value === 'auto' ? previewInviteCode.value : customInviteCode.value.trim()
  if (!code || code.length < 6) return ElMessage.warning('邀请码长度不足')
  
  creating.value = true
  try {
    const payload = { 
      code, 
      note: inviteNote.value,
      total_uses: inviteUsesMode.value === 'unlimited' ? null : inviteUses.value
    }
    await axios.post('/admin/invites', payload, { headers: authHeaders() })
    ElMessage.success('创建成功')
    inviteDialogVisible.value = false
    loadInvites()
  } catch (e) {
    ElMessage.error(e.response?.data?.detail || '创建失败')
  } finally {
    creating.value = false
  }
}

async function deleteInvite(invite) {
  try {
    await ElMessageBox.confirm('确定删除该邀请码吗？', '确认', { type: 'warning' })
    await axios.delete(`/admin/invites/${invite.code}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    loadInvites()
  } catch (e) {}
}

onMounted(loadInvites)
</script>

<style>
@import "@/assets/styles/dialogs.css";
</style>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/headers.css";
@import "@/assets/styles/tags.css";
@import "@/assets/styles/buttons.css";

.invites-section { max-width: 1000px; margin: 0 auto; padding: 20px 0; }

.code-text { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-weight: 600; color: var(--color-heading); }
.usage-tag { display: inline-block; padding: 2px 10px; border-radius: 20px; font-size: 12px; font-weight: 600; }
.note-text { color: var(--color-text-light); font-size: 13px; }
.time-text { color: var(--color-text-light); font-size: 12px; }

.code-preview-box { display: flex; align-items: center; justify-content: space-between; background: var(--color-background-soft); padding: 12px 16px; border-radius: 8px; border: 1px dashed var(--el-color-primary); }
.code-preview-box span { font-family: monospace; font-size: 18px; font-weight: bold; color: var(--el-color-primary); }

.mb-2 { margin-bottom: 8px; }
</style>