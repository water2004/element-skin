<template>
  <div class="max-w-[1000px] mx-auto py-5 animate-fade-in">
    <PageHeader title="彩蛋列表" subtitle="配置服务端允许启用的节日彩蛋">
      <template #icon><MagicStick /></template>
      <template #actions>
        <el-button :icon="Refresh" @click="loadSettings" class="hover-lift"> 重新加载 </el-button>
        <el-button type="primary" :loading="saving" @click="saveSettings" class="hover-lift">
          保存
        </el-button>
      </template>
    </PageHeader>

    <el-alert
      class="mb-6"
      type="info"
      :closable="false"
      show-icon
      title="彩蛋启用规则"
      description="这里配置的是服务端允许启用的彩蛋。客户端还会结合本地日期和用户个人设置，三者都满足时才会 lazy import 对应效果。"
    />

    <div
      class="mt-6 grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] max-sm:grid-cols-1 gap-4"
    >
      <UiCard
        v-for="egg in easterEggOptions"
        :key="egg.id"
        class="easter-egg-card"
        :class="{ active: enabledIds.includes(egg.id) }"
        shadow="never"
      >
        <div class="flex items-start max-sm:items-center gap-3 p-5">
          <div class="easter-egg-icon">
            <el-icon><MagicStick /></el-icon>
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap mb-2">
              <h3 class="m-0 text-[17px] font-bold text-[var(--color-heading)]">{{ egg.name }}</h3>
              <el-tag v-if="enabledIds.includes(egg.id)" type="success" effect="light"
                >已启用</el-tag
              >
              <el-tag v-else type="info" effect="plain">未启用</el-tag>
            </div>
            <p class="m-0 mb-3 text-[var(--color-text-light)] leading-normal">
              {{ egg.description }}
            </p>
            <div class="font-mono text-xs text-[var(--color-text-light)]">ID: {{ egg.id }}</div>
          </div>
          <el-switch
            :model-value="enabledIds.includes(egg.id)"
            @change="toggleEasterEgg(egg.id, Boolean($event))"
          />
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { MagicStick, Refresh } from '@element-plus/icons-vue'
import { getAdminSettingsGroup, saveAdminSettingsGroup } from '@/api/admin/settings'
import PageHeader from '@/components/common/PageHeader.vue'
import { availableEasterEggs } from '@/easter-eggs'
import UiCard from '@/components/ui/UiCard.vue'

const easterEggOptions = availableEasterEggs()
const enabledIds = ref<string[]>([])
const saving = ref(false)

async function loadSettings() {
  try {
    const res = await getAdminSettingsGroup('easter_eggs')
    const enabled = res.data.easter_eggs_enabled
    enabledIds.value = Array.isArray(enabled)
      ? enabled.filter((item): item is string => typeof item === 'string')
      : []
  } catch {
    ElMessage.error('加载彩蛋设置失败')
  }
}

function toggleEasterEgg(id: string, enabled: boolean) {
  const exists = enabledIds.value.includes(id)
  if (enabled && !exists) {
    enabledIds.value = [...enabledIds.value, id]
  } else if (!enabled && exists) {
    enabledIds.value = enabledIds.value.filter((item) => item !== id)
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await saveAdminSettingsGroup('easter_eggs', {
      easter_eggs_enabled: enabledIds.value,
    })
    ElMessage.success('彩蛋设置已更新')
  } catch {
    ElMessage.error('保存彩蛋设置失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.easter-egg-card {
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.easter-egg-card.active {
  border-color: rgba(64, 158, 255, 0.45);
  box-shadow: 0 10px 26px rgba(64, 158, 255, 0.08);
}

.easter-egg-card:hover {
  transform: translateY(-2px);
}

.easter-egg-icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #409eff, #8553cf);
  flex-shrink: 0;
}
</style>
