<script setup lang="ts">
import { NAlert, NButton, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RemoteResults from '@/components/RemoteResults.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import { useRemote } from '@/composables/useRemote'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { fetchPublicIp } from '../what-is-my-ip/what-is-my-ip.service'
import { asPathOf, locationList } from './reverse-traceroute.service'

const { t } = useI18n()
const locations = useQueryParam({ name: 'locations', defaultValue: 'Europe, North America, Asia' })
const ips = ref<{ v4?: string, v6?: string }>({})
const family = ref<'v4' | 'v6'>('v4')
const detecting = ref(true)
onMounted(async () => {
  const [v4, v6] = await Promise.all([fetchPublicIp(4), fetchPublicIp(6)])
  ips.value = { v4, v6 }
  family.value = v4 ? 'v4' : 'v6'
  detecting.value = false
})
const target = computed(() => ips.value[family.value])
const remote = useRemote()
const running = computed(() => remote.status.value === 'running')
function start() {
  if (!target.value)
    return
  const locs = locationList(locations.value)
  // MTR rather than traceroute: Globalping only reports AS numbers per hop for MTR.
  remote.start({ type: 'mtr', target: target.value, protocol: 'ICMP', locations: locs.join(', ') || undefined, limit: Math.max(locs.length, 1) })
}
const paths = computed(() => remote.probes.value.map(p => ({ where: [p.probe.city, p.probe.country].filter(Boolean).join(', '), asn: p.probe.asn, path: asPathOf(p) })))
</script>

<template>
  <CCard>
    <p class="mt-0 text-sm">
      {{ t('tools.reverse-traceroute.intro') }}
    </p>
    <NAlert v-if="!detecting && !ips.v4 && !ips.v6" type="error" class="mb-3" data-testid="no-ip">
      {{ t('tools.reverse-traceroute.noIp') }}
    </NAlert>
    <div v-else class="mb-3 flex flex-wrap items-center gap-3">
      <span class="text-sm">{{ t('tools.reverse-traceroute.yourIp') }}</span>
      <NRadioGroup v-model:value="family" name="family" :disabled="detecting">
        <NRadioButton value="v4" :disabled="!ips.v4">
          <span class="font-mono">{{ ips.v4 ?? 'IPv4 —' }}</span>
        </NRadioButton>
        <NRadioButton value="v6" :disabled="!ips.v6">
          <span class="font-mono">{{ ips.v6 ?? 'IPv6 —' }}</span>
        </NRadioButton>
      </NRadioGroup>
    </div>
    <CInputText v-model:value="locations" :label="t('tools.reverse-traceroute.locations')" placeholder="Germany, US+AWS, AS13335" test-id="locations" />
    <NAlert type="info" class="mb-3">
      {{ t('tools.reverse-traceroute.privacy') }}
    </NAlert>
    <NButton type="primary" :loading="running" :disabled="!target || detecting" data-testid="run" @click="start">
      {{ t('tools.reverse-traceroute.run') }}
    </NButton>
  </CCard>
  <CCard v-if="paths.some(p => p.path.length)" :title="t('tools.reverse-traceroute.paths')" data-testid="paths">
    <p class="mt-0 text-sm">
      {{ t('tools.reverse-traceroute.pathsHint') }}
    </p>
    <div v-for="(p, i) in paths" :key="i" class="py-1 text-sm">
      <span class="font-semibold">{{ p.where }}</span> (AS{{ p.asn }}):
      <span class="break-all font-mono">{{ p.path.map(a => `AS${a}`).join(' → ') || '—' }}</span>
    </div>
  </CCard>
  <RemoteResults v-if="remote.meta.value || remote.error.value" :meta="remote.meta.value" :probes="remote.probes.value" :summary="remote.summary.value" :status="remote.status.value" :error="remote.error.value" />
</template>
