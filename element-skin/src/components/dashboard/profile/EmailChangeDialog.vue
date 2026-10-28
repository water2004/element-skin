<template>
  <UiDialog
    v-model="visible"
    title="重设邮箱"
    :busy="loading || codeLoading"
    :dirty="!!form.email || !!form.code"
    @closed="resetForm"
  >
    <p class="mt-0 mb-6 text-sm leading-6 text-[var(--color-text-light)]">
      验证码将发送至新邮箱，验证成功后将立即替换当前账号邮箱。
    </p>

    <el-skeleton v-if="policyLoading" :rows="2" animated />

    <div v-else-if="policyError" class="space-y-3">
      <el-alert
        type="error"
        title="无法加载邮箱后缀策略"
        description="策略加载成功前不能发送验证码或修改邮箱。"
        :closable="false"
        show-icon
      />
      <UiButton
        loading-mode="icon"
        :icon="Refresh"
        :loading="policyLoading"
        tone="primary"
        class="w-full"
        @click="loadPolicy"
        >重新加载</UiButton
      >
    </div>

    <el-form
      v-else
      ref="formRef"
      :model="form"
      :rules="rules"
      :disabled="loading || codeLoading"
      label-position="top"
      size="large"
    >
      <el-form-item label="新邮箱地址" prop="email">
        <EmailSuffixInput
          v-model="form.email"
          :policy="emailSuffixPolicy"
          placeholder="请输入新邮箱地址"
        />
      </el-form-item>

      <el-form-item label="验证码" prop="code">
        <div class="flex w-full gap-3">
          <el-input
            v-model="form.code"
            :maxlength="8"
            placeholder="请输入验证码"
            :prefix-icon="Ticket"
          />
          <el-button
            type="primary"
            plain
            :disabled="countdown > 0 || loading"
            :loading="codeLoading"
            class="min-w-[120px]"
            @click="sendCode"
          >
            {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
          </el-button>
        </div>
      </el-form-item>
    </el-form>

    <template #footer="{ requestClose }">
      <el-button :disabled="loading || codeLoading" @click="requestClose">取消</el-button>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="policyLoading || policyError || codeLoading"
        @click="submit"
      >
        确认重设
      </el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiButton from '@/components/ui/UiButton.vue'
import { Refresh } from '@element-plus/icons-vue'
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Ticket } from '@element-plus/icons-vue'
import { changeEmail, sendEmailChangeCode } from '@/api/me'
import { getPublicSettings } from '@/api/public'
import type { PublicEmailSuffixPolicy } from '@/api/types'
import EmailSuffixInput from '@/components/common/EmailSuffixInput.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import { getErrorMessage } from '@/utils/error'
import { validateForm } from '@/utils/formValidation'
import { disabledEmailSuffixPolicy, emailSuffixPolicyError } from '@/utils/emailSuffixPolicy'

const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{
  changed: []
}>()

const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
const policyLoading = ref(false)
const policyError = ref(false)
const emailSuffixPolicy = ref<PublicEmailSuffixPolicy>({ ...disabledEmailSuffixPolicy })
let timer: ReturnType<typeof setInterval> | null = null
let countdownEndsAt = 0
let policyRequest = 0

const form = reactive({
  email: '',
  code: '',
})

const rules: FormRules = {
  email: [
    { required: true, message: '请输入新邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        const message = emailSuffixPolicyError(String(value || ''), emailSuffixPolicy.value)
        if (message) {
          callback(new Error(message))
          return
        }
        callback()
      },
      trigger: ['blur', 'change'],
    },
  ],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

async function loadPolicy() {
  const request = ++policyRequest
  policyLoading.value = true
  policyError.value = false
  try {
    const response = await getPublicSettings()
    if (request !== policyRequest || !visible.value) return
    if (!response.data.email_suffix_policy) throw new Error('email suffix policy is missing')
    emailSuffixPolicy.value = response.data.email_suffix_policy
  } catch (error) {
    if (request !== policyRequest || !visible.value) return
    console.error('Failed to load email suffix policy', error)
    policyError.value = true
  } finally {
    if (request === policyRequest) policyLoading.value = false
  }
}

async function sendCode() {
  if (
    codeLoading.value ||
    loading.value ||
    countdown.value > 0 ||
    policyLoading.value ||
    policyError.value
  )
    return
  if (!formRef.value) return
  codeLoading.value = true
  try {
    try {
      await formRef.value.validateField('email')
    } catch {
      ElMessage.warning('请先输入有效的新邮箱地址')
      return
    }
    const response = await sendEmailChangeCode({ email: form.email })
    ElMessage.success('验证码已发送到新邮箱')
    startCountdown(Math.min(response.data.ttl, 60))
  } catch (error: unknown) {
    ElMessage.error('发送失败: ' + getErrorMessage(error, '请稍后再试'))
  } finally {
    codeLoading.value = false
  }
}

async function submit() {
  if (loading.value || codeLoading.value || policyLoading.value || policyError.value) return
  loading.value = true
  try {
    if (!(await validateForm(formRef.value))) return
    await changeEmail({ email: form.email, code: form.code })
    ElMessage.success('邮箱重设成功')
    visible.value = false
    emit('changed')
  } catch (error: unknown) {
    ElMessage.error('重设失败: ' + getErrorMessage(error, '请稍后再试'))
  } finally {
    loading.value = false
  }
}

function startCountdown(seconds: number) {
  stopCountdown()
  countdownEndsAt = Date.now() + Math.max(0, seconds) * 1000
  updateCountdown()
  if (countdown.value > 0) timer = setInterval(updateCountdown, 1000)
}

function updateCountdown() {
  countdown.value = Math.max(0, Math.ceil((countdownEndsAt - Date.now()) / 1000))
  if (countdown.value === 0) stopCountdown()
}

function stopCountdown() {
  if (timer) clearInterval(timer)
  timer = null
}

function resetForm() {
  form.email = ''
  form.code = ''
  formRef.value?.clearValidate()
}

onBeforeUnmount(() => {
  policyRequest++
  stopCountdown()
})

watch(
  visible,
  (open) => {
    if (open) {
      resetForm()
      void loadPolicy()
    } else {
      policyRequest++
    }
  },
  { immediate: true },
)
</script>
