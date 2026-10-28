<template>
  <PageContainer variant="form" class="profile-section animate-fade-in">
    <PageHeader title="个人资料" subtitle="管理您的账号安全与个性化设置"></PageHeader>

    <UiCard class="max-w-[600px] mx-auto p-8 profile-form-card">
      <div class="flex items-center gap-4">
        <el-avatar
          :shape="customAvatar ? 'square' : 'circle'"
          :size="72"
          class="profile-avatar shrink-0"
          :src="customAvatar || ''"
          :class="{ 'has-custom': !!customAvatar }"
        >
          {{ !customAvatar ? emailInitial : '' }}
        </el-avatar>
        <div class="min-w-0 break-words">
          <h3 class="m-0 text-lg font-semibold text-[var(--color-heading)] transition-colors">
            {{ user?.display_name || '未设置用户名' }}
          </h3>
          <p class="mt-2 mb-0 break-all text-sm text-[var(--color-text-light)] transition-colors">
            {{ user?.email }}
          </p>
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
          <p class="mt-2 mb-0 text-[var(--el-color-warning)]">
            <el-icon><Clock /></el-icon>
            <strong>封禁剩余时间：{{ formatBanRemaining() }}</strong>
          </p>
          <p class="mt-1 mb-0 text-[13px] text-[var(--el-text-color-secondary)]">
            解封时间：{{ formatBanUntilTime() }}
          </p>
        </div>
      </el-alert>

      <el-form class="profile-form" label-width="120px" :model="form" label-position="left">
        <el-form-item label="邮箱">
          <div class="flex w-full min-w-0 flex-wrap gap-3">
            <el-input :model-value="user?.email || ''" disabled />
            <el-button v-if="canUpdateOwnAccount" @click="showEmailChangeDialog = true">
              <el-icon><Message /></el-icon>
              重设邮箱
            </el-button>
          </div>
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
          <div class="flex items-center gap-6">
            <el-switch v-model="disableEasterEgg" />
            <span class="text-[13px] text-[var(--color-text-light)]">效果你猜</span>
          </div>
        </el-form-item>

        <div class="flex flex-wrap gap-3 justify-end">
          <el-button type="primary" @click="updateProfile" size="large">
            <el-icon><Check /></el-icon>
            保存修改
          </el-button>
          <el-button
            type="danger"
            @click="showDeleteDialog = true"
            size="large"
            v-if="canDeleteOwnAccount"
          >
            <el-icon><Delete /></el-icon>
            注销账号
          </el-button>
        </div>
      </el-form>
    </UiCard>

    <EmailChangeDialog
      v-if="canUpdateOwnAccount"
      v-model="showEmailChangeDialog"
      @changed="handleEmailChanged"
    />

    <!-- 注销账号确认对话框 -->
    <UiDialog
      v-model="showDeleteDialog"
      title="确认注销账号"
      :close-on-click-modal="false"
      :busy="deletingAccount"
      @closed="deleteConfirmText = ''"
    >
      <el-alert
        title="警告：该操作不可逆！"
        type="error"
        description="注销账号后，您的所有数据（包括角色、皮肤、披风等）将被永久删除，无法恢复。"
        :closable="false"
        class="mb-5"
      />
      <p class="text-sm text-[var(--el-text-color-secondary)]">
        请输入 <strong class="text-[var(--el-color-danger)]">注销账号</strong> 来确认操作：
      </p>
      <el-input
        v-model="deleteConfirmText"
        :disabled="deletingAccount"
        placeholder="请输入：注销账号"
        class="mt-3"
      />
      <template #footer="{ requestClose }">
        <el-button :disabled="deletingAccount" @click="requestClose">取消</el-button>
        <el-button
          type="danger"
          @click="confirmDeleteAccount"
          :disabled="deleteConfirmText !== '注销账号'"
          :loading="deletingAccount"
        >
          <el-icon><Delete /></el-icon>
          确认注销
        </el-button>
      </template>
    </UiDialog>
  </PageContainer>
</template>

<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Clock, Check, Delete, Message } from '@element-plus/icons-vue'
import { useAvatar } from '@/composables/useAvatar'
import { changePassword, patchMe, deleteMe } from '@/api/me'
import { isEasterEggDisabled, setEasterEggDisabled } from '@/easter-eggs'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import EmailChangeDialog from './EmailChangeDialog.vue'
import { useSessionStore } from '@/stores/session'
import { storeToRefs } from 'pinia'
import { getErrorMessage } from '@/utils/error'

const { currentAvatarImg: customAvatar } = useAvatar()

const sessionStore = useSessionStore()
const { user } = storeToRefs(sessionStore)
const { fetchMe } = sessionStore

const router = useRouter()
const form = ref({
  display_name: '',
  old_password: '',
  new_password: '',
  confirm_password: '',
})
const showDeleteDialog = ref(false)
const showEmailChangeDialog = ref(false)
const deleteConfirmText = ref('')
const deletingAccount = ref(false)
onBeforeUnmount(() => {
  deleteConfirmText.value = ''
})
const disableEasterEgg = ref(isEasterEggDisabled())

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})
const canDeleteOwnAccount = computed(
  () => !(user.value?.permissions || []).includes('account.delete.any'),
)
const canUpdateOwnAccount = computed(() =>
  (user.value?.permissions || []).includes('account.update.self'),
)

watch(
  () => user.value,
  (newUser) => {
    if (newUser) {
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
      display_name: form.value.display_name,
    }
    await patchMe(payload)
    ElMessage.success('信息修改成功')
    fetchMe()
  } catch (e: unknown) {
    ElMessage.error('保存失败: ' + getErrorMessage(e, '保存失败'))
  }
}

async function handleEmailChanged() {
  await fetchMe()
}

async function confirmDeleteAccount() {
  if (deletingAccount.value || deleteConfirmText.value !== '注销账号') return
  deletingAccount.value = true
  try {
    await deleteMe()
  } catch (e: unknown) {
    ElMessage.error('注销失败: ' + getErrorMessage(e, '注销失败'))
    return
  } finally {
    deletingAccount.value = false
  }
  showDeleteDialog.value = false
  deleteConfirmText.value = ''
  sessionStore.clearSession()
  ElMessage.success('账号已注销')
  await router.replace('/')
}
</script>

<style scoped>
.profile-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: bold;
  font-size: 24px;
  transition:
    color var(--motion-duration-base) var(--motion-easing-standard),
    background-color var(--motion-duration-base) var(--motion-easing-standard),
    border-color var(--motion-duration-base) var(--motion-easing-standard),
    box-shadow var(--motion-duration-base) var(--motion-easing-standard),
    opacity var(--motion-duration-base) var(--motion-easing-standard),
    transform var(--motion-duration-smooth) var(--motion-easing-standard);
}

/* 个人资料展示头像保留刻意的悬停动效，触屏和减少动态效果时不启用。 */
@media (hover: hover) and (prefers-reduced-motion: no-preference) {
  .profile-avatar:hover {
    transform: scale(1.1) rotate(5deg);
    box-shadow: 0 8px 16px rgba(0, 0, 0, 0.15);
  }
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
  transition: color var(--motion-duration-base) ease;
}

.profile-form-card :deep(.el-divider__text) {
  background-color: var(--color-card-background);
  color: var(--color-heading);
  transition:
    background-color var(--motion-duration-base) ease,
    color var(--motion-duration-base) ease;
}

@media (max-width: 640px) {
  .profile-form :deep(.el-form-item) {
    display: block;
  }
  .profile-form :deep(.el-form-item__label) {
    width: 100% !important;
    height: auto;
    justify-content: flex-start;
    padding-bottom: var(--space-control-gap);
  }
  .profile-form :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
}
</style>
