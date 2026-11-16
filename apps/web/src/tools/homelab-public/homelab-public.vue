<script setup lang="ts">
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import CCard from '@/ui/c-card.vue'
import { useHomelab } from '../homelab-shared/useHomelab'

interface Row { host: string, ips: string[], status: number, finalUrl?: string, redirects: number, httpMs: number, daysLeft: number | null, verified: boolean, state: string, error?: string }
const { t } = useI18n()
const { loading, data, error, run } = useHomelab<{ rows: Row[] }>('public-check')
onMounted(() => void run())
const icon: Record<string, string> = { ok: '✔ ok', degraded: '⚠ degraded', down: '✖ down' }
const rows = computed(() => (data.value?.rows ?? []).map(r => ({
  host: r.host,
  state: icon[r.state] ?? r.state,
  http: r.status || '—',
  ms: r.httpMs ? Math.round(r.httpMs) : '—',
  ips: r.ips.join(', ') || '—',
  cert: r.daysLeft === null ? '—' : `${r.daysLeft} d${r.verified ? '' : ' ⚠'}`,
  detail: r.error ?? (r.redirects ? `→ ${r.finalUrl}` : ''),
})))
const bad = computed(() => (data.value?.rows ?? []).filter(r => r.state !== 'ok').length)
const c = (k: string) => t(`tools.homelab-public.columns.${k}`)
</script>

<template>
  <CCard>
    <NButton type="primary" :loading="loading" data-testid="run" @click="run()">
      {{ t('tools.homelab-public.run') }}
    </NButton>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('common.result')" data-testid="result">
    <NAlert v-if="bad" type="warning" class="mb-3">
      {{ t('tools.homelab-public.attention', { n: bad }) }}
    </NAlert>
    <ResultTable :columns="[{ key: 'host', title: c('host'), mono: true, width: 230 }, { key: 'state', title: c('state'), width: 120, sortable: true }, { key: 'http', title: c('http'), width: 70 }, { key: 'ms', title: c('ms'), width: 70 }, { key: 'cert', title: c('cert'), width: 90 }, { key: 'ips', title: c('ips'), mono: true, width: 220 }, { key: 'detail', title: c('detail'), width: 280 }]" :rows="rows" row-key="host" filename="homelab-public.json" />
  </CCard>
</template>
