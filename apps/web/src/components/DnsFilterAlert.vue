<script setup lang="ts">
import type { components } from '@/api/schema'
import { NAlert } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCapabilitiesStore } from '@/stores/capabilities.store'

type DnsLookup = components['schemas']['DnsLookup']

/**
 * Warns when DNS answers look blocked by a DNS filter (Pi-hole, AdGuard Home): one result gets
 * the details, several (a report) get a summary. Renders nothing when no answer is filtered.
 */
const props = defineProps<{ results: (DnsLookup | undefined)[] }>()
const { t } = useI18n()
const caps = useCapabilitiesStore()

const filtered = computed(() => props.results.filter((r): r is DnsLookup & { filtered: NonNullable<DnsLookup['filtered']> } => !!r?.filtered))
const single = computed(() => (props.results.length === 1 ? filtered.value[0] : undefined))

const reason = computed(() => {
  const r = single.value
  if (!r)
    return ''
  const f = r.filtered
  if (f.reason === 'ede')
    return t('dnsFilter.ede', { resolver: r.resolver, code: f.ede, text: f.edeText ? ` "${f.edeText}"` : '' })
  if (f.reason === 'null-address')
    return t('dnsFilter.nullAddress', { resolver: r.resolver })
  return t('dnsFilter.missing', { resolver: r.resolver, reference: f.reference })
})
const reference = computed(() => {
  const f = single.value?.filtered
  if (!f?.reference)
    return ''
  return f.referenceData.length
    ? t('dnsFilter.reference', { reference: f.reference, data: f.referenceData.join(', ') })
    : t('dnsFilter.referenceNone', { reference: f.reference, rcode: f.referenceRcode })
})
const summary = computed(() => t('dnsFilter.some', { count: filtered.value.length, names: [...new Set(filtered.value.map(r => `${r.name} ${r.type}`))].join(', ') }))
// With DNS_RESOLVER set the filter was chosen on purpose ("local"); otherwise point to the fix.
const showHint = computed(() => !caps.capabilities?.mode.dnsUpstream)
</script>

<template>
  <NAlert v-if="filtered.length" type="warning" :title="t('dnsFilter.title')" class="mb-3" data-testid="dns-filtered">
    <template v-if="single">
      <p class="m-0">
        {{ reason }}
      </p>
      <p v-if="reference" class="m-0 mt-1 text-sm font-mono">
        {{ reference }}
      </p>
    </template>
    <p v-else class="m-0">
      {{ summary }}
    </p>
    <p v-if="showHint" class="m-0 mt-1 text-sm">
      {{ t('dnsFilter.hint') }}
    </p>
  </NAlert>
</template>
