<script setup lang="ts">
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import CCard from '@/ui/c-card.vue'
import { useHomelab } from '../homelab-shared/useHomelab'

interface Row { host: string, notAfter?: string, daysLeft: number | null, issuer?: string, verified: boolean, verifyError?: string, error?: string }
const { t } = useI18n()
const { loading, data, error, run } = useHomelab<{ rows: Row[] }>('certs')
onMounted(() => void run())
function status(r: Row) {
  if (r.error)
    return `✖ ${r.error}`
  if (!r.verified)
    return `⚠ ${r.verifyError ?? t('tools.homelab-certs.unverified')}`
  return (r.daysLeft ?? 0) < 14 ? `⚠ ${t('tools.homelab-certs.soon')}` : `✔ ${t('tools.homelab-certs.valid')}`
}
const rows = computed(() => (data.value?.rows ?? []).map(r => ({
  host: r.host,
  days: r.daysLeft === null ? '—' : r.daysLeft,
  expires: r.notAfter ? r.notAfter.slice(0, 10) : '',
  issuer: r.issuer ?? '',
  status: status(r),
})))
const attention = computed(() => rows.value.filter(r => !r.status.startsWith('✔')).length)
const c = (k: string) => t(`tools.homelab-certs.columns.${k}`)
</script>

<template>
  <CCard>
    <NButton type="primary" :loading="loading" data-testid="run" @click="run()">
      {{ t('tools.homelab-certs.run') }}
    </NButton>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('common.result')" data-testid="result">
    <NAlert v-if="attention" type="warning" class="mb-3">
      {{ t('tools.homelab-certs.attention', { n: attention }) }}
    </NAlert>
    <ResultTable :columns="[{ key: 'host', title: c('host'), mono: true, width: 280 }, { key: 'days', title: c('days'), width: 90, sortable: true }, { key: 'expires', title: c('expires'), width: 110 }, { key: 'issuer', title: c('issuer'), width: 200 }, { key: 'status', title: c('status'), width: 300 }]" :rows="rows" row-key="host" filename="homelab-certs.json" />
  </CCard>
</template>
