<template>
  <div class="profile-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>个人资料</h1>
        <p>管理您的账号安全与个性化设置</p>
      </div>
    </div>

    <el-card class="max-w-600 mx-auto p-8 surface-card animate-card-slide profile-form-card">
      <div class="flex items-center gap-4">
        <el-avatar
          :shape="customAvatar ? 'square' : 'circle'"
          :size="72"
          class="profile-avatar"
          :src="customAvatar || ''"
          :class="{ 'has-custom': !!customAvatar }"
        >
          {{ !customAvatar ? emailInitial : '' }}
        </el-avatar>
        <div>
          <h3 class="m-0 text-lg font-semibold text-heading transition-colors">
            {{ user?.display_name || '未设置用户名' }}
          </h3>
          <p class="mt-2 mb-0 text-13 text-light transition-colors">{{ user?.email }}</p>
        </div>
      </div>

      <el-divider />

      <!-- 封禁状态显示 -->
      <el-alert v-if="getUserBanStatus()" type="warning" :closable="false" class="mb-5">
        <template #title>
          <div class="font-semibold text-base">账号已被封禁</div>
        </template>
        <div class="mt-2 text-sm">
          <p class="my-1">您的账号已被管理员封禁，暂时无法通过 Minecraft 客户端登录游戏。</p>
          <p class="my-1">但您仍可以正常访问皮肤站进行皮肤管理等操作。</p>
          <p class="mt-2 mb-0 text-warning">
            <el-icon><Clock /></el-icon>
            <strong>封禁剩余时间：{{ formatBanRemaining() }}</strong>
          </p>
          <p class="mt-1 mb-0 text-13 text-info">解封时间：{{ formatBanUntilTime() }}</p>
        </div>
      </el-alert>

      <el-form label-width="120px" :model="form" label-position="left">
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.display_name" placeholder="请输入用户名" />
        </el-form-item>

        <el-divider content-position="left" class="form-divider password-divider"
          >修改密码</el-divider
        >

        <el-form-item label="旧密码">
          <el-input
            type="password"
            v-model="form.old_password"
            placeholder="请输入旧密码"
            show-password
          />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            type="password"
            v-model="form.new_password"
            placeholder="请输入新密码（留空则不修改）"
            show-password
          />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input
            type="password"
            v-model="form.confirm_password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>

        <el-divider v-if="user" content-position="left" class="form-divider personalize-divider">
          个性化
        </el-divider>
        <el-form-item v-if="user" label="关闭彩蛋">
          <el-switch v-model="disableEasterEgg" />
          <el-text size="small" type="info" class="ml-3"> 效果你猜 </el-text>
        </el-form-item>

        <div class="flex gap-3 justify-end">
          <el-button type="primary" @click="updateProfile" size="large">
            <el-icon><Check /></el-icon>
            保存修改
          </el-button>
          <el-button
            type="danger"
            @click="showDeleteDialog = true"
            size="large"
            v-if="!user?.is_admin"
          >
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
      class="dialog-form"
      :close-on-click-modal="false"
    >
      <el-alert
        title="警告：该操作不可逆！"
        type="error"
        description="注销账号后，您的所有数据（包括角色、皮肤、披风等）将被永久删除，无法恢复。"
        :closable="false"
        class="mb-5"
      />
      <p class="text-sm text-info">
        请输入 <strong class="text-danger">注销账号</strong> 来确认操作：
      </p>
      <el-input v-model="deleteConfirmText" placeholder="请输入：注销账号" class="mt-3" />
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

<script setup lang="ts">
import { ref, computed, watch, inject, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Clock, Check, Delete } from '@element-plus/icons-vue'
import { useAvatar } from '@/composables/useAvatar'
import { changePassword, patchMe, deleteMe } from '@/api/me'
import { isEasterEggDisabled, setEasterEggDisabled } from '@/easter-eggs'
import type { User } from '@/api/types'

const { currentAvatarImg: customAvatar } = useAvatar()

// Inject shared state from AppLayout
const user = inject<Ref<User | null>>('user', ref(null))
const fetchMe = inject<() => Promise<void>>('fetchMe')

const router = useRouter()
const form = ref({
  email: '',
  display_name: '',
  old_password: '',
  new_password: '',
  confirm_password: '',
})
const showDeleteDialog = ref(false)
const deleteConfirmText = ref('')
const disableEasterEgg = ref(isEasterEggDisabled())

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})

watch(
  () => user.value,
  (newUser) => {
    if (newUser) {
      form.value.email = newUser.email
      form.value.display_name = newUser.display_name || ''
    }
  },
  { immediate: true, deep: true },
)

watch(disableEasterEgg, (disabled) => setEasterEggDisabled(disabled))

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

      await changePassword({
        old_password: form.value.old_password,
        new_password: form.value.new_password,
      })

      ElMessage.success('密码修改成功')
      form.value.old_password = ''
      form.value.new_password = ''
      form.value.confirm_password = ''
    }

    const payload = {
      email: form.value.email,
      display_name: form.value.display_name,
    }
    await patchMe(payload)
    ElMessage.success('信息修改成功')
    if (fetchMe) fetchMe()
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function confirmDeleteAccount() {
  try {
    await deleteMe()
    ElMessage.success('账号已注销')
    setTimeout(() => {
      router.push('/')
    }, 1000)
  } catch (e: any) {
    ElMessage.error('注销失败: ' + (e.response?.data?.detail || e.message))
  }
}
</script>

<style scoped>
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

.profile-avatar.has-custom {
  background: transparent !important;
  box-shadow: none !important;
}

.profile-avatar.has-custom :deep(img) {
  object-fit: contain;
}

.profile-form-card :deep(.el-form-item__label) {
  color: var(--color-text);
  transition: color 0.3s ease;
}

.profile-form-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.profile-form-card :deep(.el-divider__text) {
  background-color: var(--color-card-background);
  color: var(--color-heading);
  transition:
    background-color 0.3s ease,
    color 0.3s ease;
}

.profile-form-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}
</style>
