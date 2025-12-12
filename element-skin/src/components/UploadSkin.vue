<template>
  <el-card>
    <h3>上传皮肤</h3>
    <el-form>
      <el-form-item label="文件">
        <input type="file" ref="fileInput" />
      </el-form-item>
      <el-form-item label="UUID">
        <el-input v-model="uuid" />
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
import { ref } from 'vue'
import axios from 'axios'

const fileInput = ref(null)
const uuid = ref('')
const type = ref('skin')

async function upload() {
  const file = fileInput.value?.files?.[0]
  if (!file) return alert('请选择文件')
  const accessToken = localStorage.getItem('accessToken') || ''
  if (!accessToken) return alert('请先登录')

  const form = new FormData()
  form.append('file', file)
  form.append('accessToken', accessToken)
  form.append('uuid', uuid.value)
  form.append('texture_type', type.value)

  try {
    await axios.post('/textures/upload', form, { headers: { 'Content-Type': 'multipart/form-data' } })
    alert('上传成功')
  } catch (e) {
    alert('上传失败: ' + (e.response?.data?.detail || e.message))
  }
}
</script>
