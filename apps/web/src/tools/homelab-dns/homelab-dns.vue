<script setup lang="ts">
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { useHomelab } from '../homelab-shared/useHomelab'

interface Row { resolver: string, addr: string, rcode: string, answers: string[], minTtl: number, ad: boolean, rttMs: number, error?: string, differs: boolean }
const { t } = useI18n()
const name = useQueryParam({ name: 'name', defaultValue: '' })
const type = useQueryParam({ name: 'type', defaultValue: 'A' })
const { loading, data, error, run } = useHomelab<{ name: string, type: string, rows: Row[], differing: number }>('dns-compare')
const go = () => run(`?name=${encodeURIComponent(name.value.trim())}&type=${encodeURIComponent(type.value.trim() || 'A')}`)
onMounted(() => void go())
const rows = computed(() => (data.value?.rows ?? []).map(r => ({
  resolver: `${r.resolver} (${r.addr})`,
  rcode: r.error ? '✖' : r.rcode,
  answers: r.error ?? (r.answers.join('\n') || '—'),
  ttl: r.minTtl || '—',
  ad: r.ad ? '✔' : '—',
  rtt: r.rttMs ? r.rttMs.toFixed(1) : '—',
  diff: r.differs ? '⚠' : '',
})))
const c = (k: string) => t(`tools.homelab-dns.columns.${k}`)
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="go">
      <CInputText v-model:value="name" :label="t('tools.homelab-dns.name')" :placeholder="t('tools.homelab-dns.namePlaceholder')" monospace test-id="name" />
      <CInputText v-model:value="type" :label="t('tools.homelab-dns.type')" placeholder="A" monospace test-id="type" />
      <NButton type="primary" attr-type="submit" :loading="loading" data-testid="run">
        {{ t('tools.homelab-dns.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="`${data.name} ${data.type}`" data-testid="result">
    <NAlert :type="data.differing ? 'warning' : 'success'" class="mb-3">
      {{ data.differing ? t('tools.homelab-dns.differs', { n: data.differing }) : t('tools.homelab-dns.same') }}
    </NAlert>
    <ResultTable :columns="[{ key: 'resolver', title: c('resolver'), width: 220 }, { key: 'rcode', title: c('rcode'), width: 100 }, { key: 'answers', title: c('answers'), mono: true, width: 300 }, { key: 'ttl', title: c('ttl'), width: 70 }, { key: 'ad', title: c('ad'), width: 60 }, { key: 'rtt', title: c('rtt'), width: 80 }, { key: 'diff', title: c('diff'), width: 60 }]" :rows="rows" row-key="resolver" :actions="false" />
  </CCard>
</template>
