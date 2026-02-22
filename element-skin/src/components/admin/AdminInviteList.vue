<template>
  <div class="invites-section">
    <div class="section-header">
      <h2>邀请码管理</h2>
      <div style="display: flex; gap: 12px;">
        <el-button type="primary" @click="loadInvites">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="success" @click="showInviteDialog">
          <el-icon><Plus /></el-icon>
          创建邀请码
        </el-button>
      </div>
    </div>

    <el-card class="list-card">
      <el-table :data="invites" style="width: 100%">
        <el-table-column prop="code" label="邀请码" min-width="180">
          <template #default="{ row }">
            <el-text copyable class="code-text">{{ row.code }}</el-text>
          </template>
        </el-table-column>
        <el-table-column label="使用" width="100" align="center">
          <template #default="{ row }">
            <div class="usage-cell">
              <span :style="{ color: getRemainingColor(row) }">
                {{ row.used_count || 0 }}/{{ row.total_uses || '∞' }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="used_by" label="最后使用者" min-width="150" />
        <el-table-column prop="note" label="备注" min-width="180">
          <template #default="{ row }">
            {{ row.note || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="160">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              size="small"
              type="danger"
              @click="deleteInvite(row)"
              link
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 邀请码创建弹窗 -->
    <el-dialog
      v-model="inviteDialogVisible"
      title="创建邀请码"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="生成方式">
          <el-radio-group v-model="inviteMode">
            <el-radio value="auto">自动生成</el-radio>
            <el-radio value="manual">手动输入</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'manual'" label="邀请码">
          <el-input
            v-model="customInviteCode"
            placeholder="请输入自定义邀请码（6-32个字符）"
            maxlength="32"
            show-word-limit
          />
          <el-text size="small" type="info" style="margin-top: 8px;">
            支持字母、数字和常见符号，建议使用易记的格式
          </el-text>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'auto'" label="预览">
          <el-text type="success" size="large" style="font-family: monospace;">
            {{ previewInviteCode }}
          </el-text>
          <el-button
            link
            type="primary"
            @click="refreshPreview"
            style="margin-left: 12px;"
          >
            <el-icon><Refresh /></el-icon>
            换一个
          </el-button>
        </el-form-item>

        <el-form-item label="使用次数">
          <el-radio-group v-model="inviteUsesMode" style="margin-bottom: 12px;">
            <el-radio value="limited">限制次数</el-radio>
            <el-radio value="unlimited">无限使用</el-radio>
          </el-radio-group>
          <el-input-number
            v-if="inviteUsesMode === 'limited'"
            v-model="inviteUses"
            :min="1"
            :max="1000"
            controls-position="right"
            style="width: 100%;"
          />
          <el-text v-if="inviteUsesMode === 'limited'" size="small" type="info" style="margin-top: 8px; display: block;">
            设置该邀请码可以被使用的次数
          </el-text>
          <el-text v-else size="small" type="info" style="margin-top: 8px; display: block;">
            该邀请码可以被无限次使用
          </el-text>
        </el-form-item>

        <el-form-item label="备注">
          <el-input
            v-model="inviteNote"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
            placeholder="可选，填写备注"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="inviteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCreateInvite" :loading="creating">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Check } from '@element-plus/icons-vue'

const invites = ref([])
const inviteDialogVisible = ref(false)
const inviteMode = ref('auto')
const customInviteCode = ref('')
const previewInviteCode = ref('')
const creating = ref(false)
const inviteUsesMode = ref('limited')
const inviteUses = ref(1)
const inviteNote = ref('')

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function formatDate(timestamp) {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN')
}

async function loadInvites() {
  try {
    const res = await axios.get('/admin/invites', { headers: authHeaders() })
    invites.value = res.data
  } catch (e) {
    ElMessage.error('获取邀请码列表失败')
  }
}

function generateRandomCode() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  let code = ''
  for (let i = 0; i < 16; i++) {
    code += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return code
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

function getRemainingColor(row) {
  if (!row.total_uses) return '#67c23a' // 无限制，绿色
  const remaining = row.total_uses - (row.used_count || 0)
  const percentage = remaining / row.total_uses
  if (percentage <= 0) return '#f56c6c' // 红色
  if (percentage <= 0.3) return '#e6a23c' // 黄色
  return '#67c23a' // 绿色
}

async function confirmCreateInvite() {
  const code = inviteMode.value === 'auto' ? previewInviteCode.value : customInviteCode.value.trim()

  if (!code) {
    ElMessage.warning('请输入邀请码')
    return
  }

  if (code.length < 6) {
    ElMessage.warning('邀请码至少需要6个字符')
    return
  }

  if (!/^[a-zA-Z0-9_-]+$/.test(code)) {
    ElMessage.warning('邀请码只能包含字母、数字、下划线和横线')
    return
  }

  creating.value = true
  try {
    const payload = { code }

    if (inviteNote.value.trim()) {
      payload.note = inviteNote.value.trim()
    }

    if (inviteUsesMode.value === 'unlimited') {
      payload.total_uses = null
    } else {
      payload.total_uses = inviteUses.value
    }

    const res = await axios.post('/admin/invites', payload, { headers: authHeaders() })
    ElMessage.success('创建成功！邀请码：' + res.data.code)
    inviteDialogVisible.value = false
    loadInvites()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    creating.value = false
  }
}

async function deleteInvite(invite) {
  try {
    await ElMessageBox.confirm('确定要删除此邀请码吗？', '确认', { type: 'warning' })
    await axios.delete(`/admin/invites/${invite.code}`, { headers: authHeaders() })
    ElMessage.success('删除成功')
    loadInvites()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  loadInvites()
})
</script>

<style scoped>
.invites-section {
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.list-card {
  width: 100%;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.code-text {
  font-family: monospace;
}

@media (max-width: 768px) {
  .invites-section {
    padding: 0;
  }
  .list-card :deep(.el-card__body) {
    padding: 10px;
  }
}
</style>