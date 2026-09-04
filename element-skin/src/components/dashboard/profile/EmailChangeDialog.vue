<template>
  <UiDialog v-model="visible" title="重设邮箱" :close-on-click-modal="false" @closed="reset">
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
      <el-button type="primary" plain class="w-full" @click="loadPolicy">重新加载</el-button>
    </div>

    <el-form v-else ref="formRef" :model="form" :rules="rules" label-position="top" size="large">
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
            :disabled="countdown > 0"
            :loading="codeLoading"
            class="min-w-[120px]"
            @click="sendCode"
          >
            {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
          </el-button>
        </div>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button :disabled="loading" @click="visible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="loading"
        :disabled="policyLoading || policyError"
        @click="submit"
      >
        确认重设
      </el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
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
  policyLoading.value = true
  policyError.value = false
  try {
    const response = await getPublicSettings()
    if (!response.data.email_suffix_policy) throw new Error('email suffix policy is missing')
    emailSuffixPolicy.value = response.data.email_suffix_policy
  } catch (error) {
    console.error('Failed to load email suffix policy', error)
    policyError.value = true
  } finally {
    policyLoading.value = false
  }
}

async function sendCode() {
  try {
    if (!formRef.value) return
    await formRef.value.validateField('email')
  } catch {
    ElMessage.warning('请先输入有效的新邮箱地址')
    return
  }

  try {
    codeLoading.value = true
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
  if (!(await validateForm(formRef.value))) return

  loading.value = true
  try {
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
  countdown.value = seconds
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) stopCountdown()
  }, 1000)
}

function stopCountdown() {
  if (timer) clearInterval(timer)
  timer = null
}

function reset() {
  stopCountdown()
  countdown.value = 0
  form.email = ''
  form.code = ''
  formRef.value?.clearValidate()
}

onBeforeUnmount(stopCountdown)

watch(visible, (open) => {
  if (open) void loadPolicy()
})
</script>
