<script setup lang="ts">
import type { Guest } from './homelab-inventory.service'
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import { useSse } from '@/composables/useSse'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { useHomelab } from '../homelab-shared/useHomelab'
import { nameIndex, parseKnown, unknownHosts } from './homelab-inventory.service'

interface GuestRow extends Guest { cpus: number, memMb: number, tags?: string, uptime: number }
interface NodeStatus { node: string, ok: boolean, error?: string }
interface UniFiClient { name: string, ip: string, mac: string, type: string, connectedAt?: string }
interface UniFiDevice { name: string, ip: string, mac: string, model: string, state: string }
interface UniFi { ok: boolean, error?: string, clients: UniFiClient[], devices: UniFiDevice[] }
const { t } = useI18n()
const { loading, data, error, run } = useHomelab<{ guests: GuestRow[], nodes: NodeStatus[], unifi?: UniFi | null }>('inventory')
onMounted(() => void run())

const cidr = useQueryParam({ name: 'cidr', defaultValue: '192.168.20.0/24' })
const known = useQueryParam({ name: 'known', defaultValue: '' })
const arp = ref<{ ip: string, mac: string }[]>([])
const { status, error: arpError, start } = useSse('/v1/lan/arpscan/stream', { host: d => arp.value.push(d as { ip: string, mac: string }) })
function compare() {
  arp.value = []
  start({ cidr: cidr.value.trim() })
}
const scanned = computed(() => status.value === 'done')

const rows = computed(() => (data.value?.guests ?? []).map(g => ({
  key: `${g.node}-${g.vmid}`,
  node: g.node,
  vmid: g.vmid,
  type: g.type,
  name: g.name,
  status: g.status === 'running' ? '✔ running' : g.status,
  ip: g.ip ?? '—',
  cpus: g.cpus,
  mem: g.memMb,
  tags: g.tags ?? '',
})))
const names = computed(() => nameIndex(data.value?.unifi?.clients ?? [], data.value?.unifi?.devices ?? []))
const unknown = computed(() => unknownHosts(arp.value, data.value?.guests ?? [], parseKnown(known.value), names.value))
const unifiRows = computed(() => (data.value?.unifi?.clients ?? []).map(cl => ({
  mac: cl.mac,
  name: cl.name || '—',
  ip: cl.ip || '—',
  conn: cl.type === 'WIRED' ? t('tools.homelab-inventory.wired') : t('tools.homelab-inventory.wireless'),
  since: cl.connectedAt ? cl.connectedAt.slice(0, 10) : '',
})))
const badNodes = computed(() => (data.value?.nodes ?? []).filter(n => !n.ok))
const c = (k: string) => t(`tools.homelab-inventory.columns.${k}`)
</script>

<template>
  <CCard>
    <NButton type="primary" :loading="loading" data-testid="run" @click="run()">
      {{ t('tools.homelab-inventory.run') }}
    </NButton>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('tools.homelab-inventory.guests', { n: rows.length })" data-testid="result">
    <NAlert v-for="n in badNodes" :key="n.node" type="error" class="mb-3">
      {{ n.node }}: {{ n.error }}
    </NAlert>
    <ResultTable :columns="[{ key: 'node', title: c('node'), width: 90, sortable: true }, { key: 'vmid', title: c('vmid'), width: 70, sortable: true }, { key: 'type', title: c('type'), width: 70 }, { key: 'name', title: c('name'), width: 170, sortable: true }, { key: 'status', title: c('status'), width: 110 }, { key: 'ip', title: c('ip'), mono: true, width: 140 }, { key: 'cpus', title: c('cpus'), width: 60 }, { key: 'mem', title: c('mem'), width: 90 }, { key: 'tags', title: c('tags'), width: 120 }]" :rows="rows" row-key="key" filename="homelab-inventory.json" />
  </CCard>
  <CCard v-if="data" :title="t('tools.homelab-inventory.arpTitle')">
    <form class="contents" @submit.prevent="compare">
      <CInputText v-model:value="cidr" :label="t('tools.homelab-inventory.subnet')" monospace test-id="cidr" />
      <CInputText v-model:value="known" :label="t('tools.homelab-inventory.known')" placeholder="192.168.20.1, 192.168.20.20" monospace test-id="known" />
      <NButton attr-type="submit" :loading="status === 'running'" data-testid="compare">
        {{ t('tools.homelab-inventory.compare') }}
      </NButton>
      <p class="mb-0 text-xs nt-muted">
        {{ t('tools.homelab-inventory.arpNote') }}
      </p>
    </form>
    <ApiError v-if="arpError" :error="arpError" />
    <NAlert v-if="scanned" :type="unknown.length ? 'warning' : 'success'" class="mt-3">
      {{ t('tools.homelab-inventory.unknownResult', { n: unknown.length, total: arp.length }) }}
    </NAlert>
    <ResultTable v-if="scanned && unknown.length" class="mt-3" :columns="[{ key: 'ip', title: c('ip'), mono: true, width: 150 }, { key: 'mac', title: c('mac'), mono: true, width: 190 }, { key: 'name', title: c('name'), width: 240 }]" :rows="unknown" row-key="ip" :actions="false" />
  </CCard>
  <CCard v-if="data?.unifi" :title="t('tools.homelab-inventory.unifiTitle', { n: unifiRows.length })" data-testid="unifi">
    <NAlert v-if="!data.unifi.ok" type="error" class="mb-3">
      UniFi: {{ data.unifi.error }}
    </NAlert>
    <ResultTable v-else :columns="[{ key: 'name', title: c('name'), width: 240, sortable: true }, { key: 'ip', title: c('ip'), mono: true, width: 140, sortable: true }, { key: 'mac', title: c('mac'), mono: true, width: 170 }, { key: 'conn', title: c('conn'), width: 100, sortable: true }, { key: 'since', title: c('since'), width: 110, sortable: true }]" :rows="unifiRows" row-key="mac" filename="unifi-clients.json" />
  </CCard>
</template>
