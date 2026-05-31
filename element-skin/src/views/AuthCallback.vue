<template>
  <div class="auth-callback">
    <el-result icon="info" title="正在处理授权...">
      <template #sub-title>
        <p>请稍候，正在完成微软账户登录...</p>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

onMounted(async () => {
  // 获取URL参数
  const code = route.query.code
  const state = route.query.state
  const error = route.query.error

  // 如果有错误，重定向到dashboard并显示错误
  if (error) {
    await router.push({
      path: '/dashboard/roles',
      query: { error: error }
    })
    return
  }

  // 这个页面实际上不会被显示，因为后端会直接重定向
  // 但如果用户手动访问这个页面，给出提示
  if (!code) {
    await router.push('/dashboard/roles')
  }
})
</script>

<style scoped>
.auth-callback {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}
</style>
