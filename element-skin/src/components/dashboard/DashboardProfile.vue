<template>
  <div class="profile-section">
    <div class="section-header">
      <h2>个人资料</h2>
    </div>

    <el-card class="profile-form-card">
      <div class="profile-header">
        <el-avatar :size="72" class="profile-avatar">{{ emailInitial }}</el-avatar>
        <div class="profile-meta">
          <h3>{{ user?.display_name || '未设置显示名' }}</h3>
          <p>{{ user?.email }}</p>
        </div>
      </div>

      <el-divider />

      <!-- 封禁状态显示 -->
      <el-alert
        v-if="getUserBanStatus()"
        type="warning"
        :closable="false"
        style="margin-bottom: 20px;"
      >
        <template #title>
          <div style="font-weight: 600; font-size: 16px;">账号已被封禁</div>
        </template>
        <div style="margin-top: 8px; font-size: 14px;">
          <p style="margin: 4px 0;">您的账号已被管理员封禁，暂时无法通过 Minecraft 客户端登录游戏。</p>
          <p style="margin: 4px 0;">但您仍可以正常访问皮肤站进行皮肤管理等操作。</p>
          <p style="margin: 8px 0 0 0; font-size: 15px; color: #e6a23c;">
            <el-icon><Clock /></el-icon>
            <strong>封禁剩余时间：{{ formatBanRemaining() }}</strong>
          </p>
          <p style="margin: 4px 0 0 0; color: #909399; font-size: 13px;">
            解封时间：{{ formatBanUntilTime() }}
          </p>
        </div>
      </el-alert>

      <el-form label-width="120px" :model="form" label-position="left">
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="form.display_name" placeholder="显示名称（可选）" />
        </el-form-item>

        <el-divider content-position="left">修改密码</el-divider>

        <el-form-item label="旧密码">
          <el-input type="password" v-model="form.old_password" placeholder="请输入旧密码" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input type="password" v-model="form.new_password" placeholder="请输入新密码（留空则不修改）" show-password />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input type="password" v-model="form.confirm_password" placeholder="请再次输入新密码" show-password />
        </el-form-item>

        <div class="profile-actions">
          <el-button type="primary" @click="updateProfile" size="large">
            <el-icon><Check /></el-icon>
            保存修改
          </el-button>
          <el-button type="danger" @click="showDeleteDialog = true" size="large" v-if="!user?.is_admin">
            <el-icon><Delete /></el-icon>
            注销账号
          </el-button>
        </div>
      </el-form>
    </el-card>

    <!-- 注销账号确认对话框 -->
    <el-dialog
      v-model="showDeleteDialog"
      title="确认注销账号"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="警告：该操作不可逆！"
        type="error"
        description="注销账号后，您的所有数据（包括角色、皮肤、披风等）将被永久删除，无法恢复。"
        :closable="false"
        style="margin-bottom: 20px;"
      />
      <p style="font-size: 14px; color: #606266;">
        请输入 <strong style="color: #f56c6c;">注销账号</strong> 来确认操作：
      </p>
      <el-input
        v-model="deleteConfirmText"
        placeholder="请输入：注销账号"
        style="margin-top: 10px;"
      />
      <template #footer>
        <el-button @click="showDeleteDialog = false">取消</el-button>
        <el-button
          type="danger"
          @click="confirmDeleteAccount"
          :disabled="deleteConfirmText !== '注销账号'"
        >
          <el-icon><Delete /></el-icon>
          确认注销
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, inject } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Clock, Check, Delete } from '@element-plus/icons-vue'

// Inject shared state from AppLayout
const user = inject('user')
const fetchMe = inject('fetchMe')

const router = useRouter()
const form = ref({ email: '', display_name: '', old_password: '', new_password: '', confirm_password: '' })
const showDeleteDialog = ref(false)
const deleteConfirmText = ref('')

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})

watch(() => user.value, (newUser) => {
  if (newUser) {
    form.value.email = newUser.email
    form.value.display_name = newUser.display_name || ''
  }
}, { immediate: true, deep: true })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function getUserBanStatus() {
  if (!user.value?.banned_until) return false
  return Date.now() < user.value.banned_until
}

function formatBanRemaining() {
  if (!user.value?.banned_until) return ''
  const remaining = user.value.banned_until - Date.now()
  if (remaining <= 0) return '已到期'

  const days = Math.floor(remaining / (1000 * 60 * 60 * 24))
  const hours = Math.floor((remaining % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((remaining % (1000 * 60 * 60)) / (1000 * 60))

  if (days > 0) {
    return `${days}天 ${hours}小时 ${minutes}分钟`
  } else if (hours > 0) {
    return `${hours}小时 ${minutes}分钟`
  } else {
    return `${minutes}分钟`
  }
}

function formatBanUntilTime() {
  if (!user.value?.banned_until) return ''
  const until = new Date(user.value.banned_until)
  return until.toLocaleString('zh-CN')
}

async function updateProfile() {
  try {
    if (form.value.new_password) {
      if (!form.value.old_password) {
        ElMessage.error('请输入旧密码')
        return
      }
      if (form.value.new_password.length < 6) {
        ElMessage.error('新密码长度不能少于6个字符')
        return
      }
      if (form.value.new_password !== form.value.confirm_password) {
        ElMessage.error('两次输入的新密码不一致')
        return
      }

      await axios.post('/me/password', {
        old_password: form.value.old_password,
        new_password: form.value.new_password
      }, { headers: authHeaders() })

      ElMessage.success('密码修改成功')
      form.value.old_password = ''
      form.value.new_password = ''
      form.value.confirm_password = ''
    }

    const payload = {
      email: form.value.email,
      display_name: form.value.display_name
    }
    await axios.patch('/me', payload, { headers: authHeaders() })
    ElMessage.success('信息修改成功')
    fetchMe()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function confirmDeleteAccount() {
  try {
    await axios.delete('/me', { headers: authHeaders() })
    ElMessage.success('账号已注销')
    localStorage.removeItem('jwt')
    localStorage.removeItem('accessToken')
    setTimeout(() => {
      router.push('/')
    }, 1000)
  } catch (e) {
    ElMessage.error('注销失败: ' + (e.response?.data?.detail || e.message))
  }
}
</script>

<style scoped>
.profile-section {
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.profile-form-card {
  max-width: 600px;
  margin: 0 auto;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
}

.profile-header {
  display: flex;
  align-items: center;
  gap: 16px;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
}

.profile-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: bold;
  font-size: 24px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.profile-avatar:hover {
  transform: scale(1.1) rotate(5deg);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.15);
}

.profile-meta h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-heading);
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.2s backwards;
}

.profile-meta p {
  margin: 6px 0 0;
  color: var(--color-text-light);
  font-size: 13px;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.25s backwards;
}

.profile-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.6s backwards;
}

.profile-form-card :deep(.el-form-item__label) {
  color: var(--color-text);
}

.profile-form-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

.profile-form-card :deep(.el-divider) {
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

.profile-form-card :deep(.el-divider__text) {
  background-color: var(--color-card-background);
  color: var(--color-heading);
}

/* 1. Email */
.profile-form-card :deep(.el-form-item):nth-child(1) { animation-delay: 0.3s; }
/* 2. Display Name */
.profile-form-card :deep(.el-form-item):nth-child(2) { animation-delay: 0.35s; }
/* 3. Divider */
.profile-form-card :deep(.el-divider) { animation-delay: 0.4s; }
/* 4. Old Password */
.profile-form-card :deep(.el-form-item):nth-child(4) { animation-delay: 0.45s; }
/* 5. New Password */
.profile-form-card :deep(.el-form-item):nth-child(5) { animation-delay: 0.5s; }
/* 6. Confirm Password */
.profile-form-card :deep(.el-form-item):nth-child(6) { animation-delay: 0.55s; }

.profile-form-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}
</style>