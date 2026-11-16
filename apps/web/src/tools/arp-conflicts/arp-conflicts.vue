<script setup lang="ts">
import type { ArpEntry } from './arp-conflicts.service'
import { NAlert, NButton } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import { useSse } from '@/composables/useSse'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { analyze } from './arp-conflicts.service'

const { t } = useI18n()
const cidr = useQueryParam({ name: 'cidr', defaultValue: '192.168.1.0/24' })
const key = computed(() => `nettools:arp-baseline:${cidr.value.trim()}`)
const hosts = ref<ArpEntry[]>([])
const baseline = ref<ArpEntry[]>()
const { status, error, start } = useSse('/v1/lan/arpscan/stream', { host: d => hosts.value.push(d as ArpEntry) })

function readBaseline(): ArpEntry[] | undefined {
  try {
    const raw = localStorage.getItem(key.value)
    return raw ? JSON.parse(raw) as ArpEntry[] : undefined
  }
  catch {
    return undefined
  }
}
function saveBaseline() {
  try {
    localStorage.setItem(key.value, JSON.stringify(hosts.value))
    baseline.value = [...hosts.value]
  }
  catch { /* storage unavailable: the baseline just does not persist */ }
}
function run() {
  hosts.value = []
  baseline.value = readBaseline()
  start({ cidr: cidr.value.trim() })
}
const done = computed(() => status.value === 'done')
const findings = computed(() => analyze(hosts.value, baseline.value))
const rows = computed(() => findings.value.map((f, i) => ({ id: i, kind: t(`tools.arp-conflicts.kinds.${f.kind}`), ip: f.ip, detail: f.detail })))
const real = computed(() => findings.value.filter(f => f.kind !== 'multi-ip' && f.kind !== 'gone-device').length)
const c = (k: string) => t(`tools.arp-conflicts.columns.${k}`)
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="run">
      <CInputText v-model:value="cidr" :label="t('tools.arp-conflicts.subnet')" placeholder="192.168.1.0/24" monospace test-id="cidr" />
      <div class="flex gap-2">
        <NButton type="primary" attr-type="submit" :loading="status === 'running'" data-testid="run">
          {{ t('tools.arp-conflicts.run') }}
        </NButton>
        <NButton :disabled="!done" data-testid="baseline" @click="saveBaseline">
          {{ t('tools.arp-conflicts.saveBaseline') }}
        </NButton>
      </div>
      <p class="mb-0 text-xs nt-muted">
        {{ t('tools.arp-conflicts.note') }}
      </p>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="done" :title="t('tools.arp-conflicts.result', { n: hosts.length })" data-testid="result">
    <NAlert :type="real ? 'warning' : 'success'" class="mb-3">
      {{ real ? t('tools.arp-conflicts.attention', { n: real }) : t(baseline ? 'tools.arp-conflicts.clean' : 'tools.arp-conflicts.cleanNoBaseline') }}
    </NAlert>
    <ResultTable v-if="rows.length" :columns="[{ key: 'kind', title: c('kind'), width: 190 }, { key: 'ip', title: c('ip'), mono: true, width: 200 }, { key: 'detail', title: c('detail'), mono: true, width: 330 }]" :rows="rows" row-key="id" :actions="false" />
  </CCard>
</template>
