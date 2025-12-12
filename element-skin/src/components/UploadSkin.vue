<template>
  <el-card>
    <h3>上传皮肤</h3>
    <el-form>
      <el-form-item label="文件">
        <input type="file" ref="fileInput" />
      </el-form-item>

      <el-form-item label="Profile">
        <el-select v-model="selectedProfile" placeholder="选择 Profile">
          <el-option v-for="p in profiles" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
      </el-form-item>

      <el-form-item label="类型">
        <el-select v-model="type" placeholder="选择类型">
          <el-option label="skin" value="skin" />
          <el-option label="cape" value="cape" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="upload">上传</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

const fileInput = ref(null)
const profiles = ref([])
const selectedProfile = ref('')
const type = ref('skin')

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

async function fetchProfiles() {
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    profiles.value = res.data.profiles || []
    if (profiles.value.length) selectedProfile.value = profiles.value[0].id
  } catch (e) {
    console.error('failed to fetch profiles', e)
  }
}

onMounted(fetchProfiles)

async function upload() {
  const file = fileInput.value?.files?.[0]
  if (!file) return alert('请选择文件')
  const accessToken = localStorage.getItem('accessToken') || ''
  if (!accessToken) return alert('请先登录')
  if (!selectedProfile.value) return alert('请选择 Profile')

  const form = new FormData()
  form.append('file', file)
  form.append('accessToken', accessToken)
  form.append('uuid', selectedProfile.value)
  form.append('texture_type', type.value)

  try {
    await axios.post('/textures/upload', form, { headers: { 'Content-Type': 'multipart/form-data' } })
    alert('上传成功')
  } catch (e) {
    alert('上传失败: ' + (e.response?.data?.detail || e.response?.data || e.message))
  }
}
</script>
