<script setup lang="ts">
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import CCard from '@/ui/c-card.vue'
import { useHomelab } from '../homelab-shared/useHomelab'

interface Row { name: string, group: string, host: string, port: number, state: string, rttMs: number, detail?: string }
const { t } = useI18n()
const { loading, data, error, run } = useHomelab<{ rows: Row[], open: number, total: number }>('reach')
onMounted(() => void run())
const icon: Record<string, string> = { open: '✔ open', closed: '⚠ closed', filtered: '✖ filtered' }
const rows = computed(() => (data.value?.rows ?? []).map(r => ({
  key: `${r.host}:${r.port}`,
  name: r.name,
  group: r.group || '—',
  target: `${r.host}:${r.port}`,
  state: icon[r.state] ?? r.state,
  rtt: r.rttMs ? r.rttMs.toFixed(1) : '—',
  detail: r.detail ?? '',
})))
const c = (k: string) => t(`tools.homelab-reach.columns.${k}`)
</script>

<template>
  <CCard>
    <NButton type="primary" :loading="loading" data-testid="run" @click="run()">
      {{ t('tools.homelab-reach.run') }}
    </NButton>
    <p class="mb-0 mt-2 text-xs nt-muted">
      {{ t('tools.homelab-reach.note') }}
    </p>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('tools.homelab-reach.result', { open: data.open, total: data.total })" data-testid="result">
    <NAlert v-if="data.open < data.total" type="warning" class="mb-3">
      {{ t('tools.homelab-reach.unreachable', { n: data.total - data.open }) }}
    </NAlert>
    <ResultTable :columns="[{ key: 'name', title: c('name'), width: 150 }, { key: 'group', title: c('group'), width: 100 }, { key: 'target', title: c('target'), mono: true, width: 190 }, { key: 'state', title: c('state'), width: 120, sortable: true }, { key: 'rtt', title: c('rtt'), width: 90 }, { key: 'detail', title: c('detail'), width: 260 }]" :rows="rows" row-key="key" filename="homelab-reach.json" />
  </CCard>
</template>
