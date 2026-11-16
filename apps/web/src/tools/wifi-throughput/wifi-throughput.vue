<script setup lang="ts">
import { NFormItem, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CSelect from '@/ui/c-select.vue'
import { phyRate, STANDARDS, table } from './wifi-throughput.service'

const { t } = useI18n()
const stdId = useQueryParam({ name: 'std', defaultValue: 'ax' })
const width = useQueryParam({ name: 'width', defaultValue: 80 })
const streams = useQueryParam({ name: 'ss', defaultValue: 2 })
const gi = useQueryParam({ name: 'gi', defaultValue: 0.8 })
const mcs = useQueryParam({ name: 'mcs', defaultValue: 11 })
const std = computed(() => STANDARDS.find(s => s.id === stdId.value) ?? STANDARDS[2]!)

// Keep the other choices valid when the standard changes.
watch(std, (s) => {
  if (!s.widths.includes(width.value))
    width.value = s.widths.at(-1)!
  if (streams.value > s.maxStreams)
    streams.value = s.maxStreams
  if (!s.gis.includes(gi.value))
    gi.value = s.gis[0]!
  if (mcs.value > s.maxMcs)
    mcs.value = s.maxMcs
}, { immediate: true })

const rate = computed(() => phyRate(std.value, width.value, mcs.value, streams.value, gi.value))
const real = computed(() => (rate.value ? Math.round(rate.value * std.value.efficiency) : undefined))
const mcsOptions = computed(() => table(std.value, width.value, 1, std.value.gis[0]!).map(r => ({ value: r.mcs, label: `MCS ${r.mcs} · ${r.mod} ${r.coding}` })))
const streamOptions = computed(() => Array.from({ length: std.value.maxStreams }, (_, i) => ({ value: i + 1, label: String(i + 1) })))
const rows = computed(() => table(std.value, width.value, streams.value, gi.value).map(r => ({
  mcs: r.mcs === mcs.value ? `▶ ${r.mcs}` : String(r.mcs),
  mod: r.mod,
  coding: r.coding,
  rate: r.rate === undefined ? t('tools.wifi-throughput.notDefined') : `${r.rate} Mbit/s`,
  real: r.rate === undefined ? '—' : `≈ ${Math.round(r.rate * std.value.efficiency)} Mbit/s`,
})))
const columns = computed(() => [
  { key: 'mcs', title: 'MCS', width: 70 },
  { key: 'mod', title: t('tools.wifi-throughput.modulation'), width: 120 },
  { key: 'coding', title: t('tools.wifi-throughput.coding'), width: 90 },
  { key: 'rate', title: t('tools.wifi-throughput.phy'), width: 150 },
  { key: 'real', title: t('tools.wifi-throughput.real'), width: 150 },
])
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.wifi-throughput.standard')">
      <NRadioGroup v-model:value="stdId" name="std">
        <NRadioButton v-for="s in STANDARDS" :key="s.id" :value="s.id">
          {{ s.name }}
        </NRadioButton>
      </NRadioGroup>
    </NFormItem>
    <NFormItem :label="t('tools.wifi-throughput.width')">
      <NRadioGroup v-model:value="width" name="width">
        <NRadioButton v-for="w in std.widths" :key="w" :value="w">
          {{ w }} MHz
        </NRadioButton>
      </NRadioGroup>
    </NFormItem>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-3">
      <CSelect v-model:value="streams" :label="t('tools.wifi-throughput.streams')" :options="streamOptions" />
      <CSelect v-model:value="gi" :label="t('tools.wifi-throughput.gi')" :options="std.gis.map(g => ({ value: g, label: `${g} µs` }))" />
      <CSelect v-model:value="mcs" label="MCS" :options="mcsOptions" />
    </div>
  </CCard>
  <CCard :title="t('common.result')" data-testid="result">
    <div class="flex flex-wrap gap-8">
      <div>
        <div class="text-sm opacity-75">
          {{ t('tools.wifi-throughput.phy') }}
        </div>
        <div class="text-3xl font-semibold font-mono" data-testid="phy">
          {{ rate === undefined ? t('tools.wifi-throughput.notDefined') : `${rate} Mbit/s` }}
        </div>
      </div>
      <div v-if="real">
        <div class="text-sm opacity-75">
          {{ t('tools.wifi-throughput.real') }}
        </div>
        <div class="text-3xl font-semibold font-mono">
          ≈ {{ real }} Mbit/s
        </div>
      </div>
    </div>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.wifi-throughput.hint', { pct: Math.round(std.efficiency * 100) }) }}
    </p>
  </CCard>
  <CCard :title="t('tools.wifi-throughput.table', { width, streams, gi })">
    <ResultTable :columns="columns" :rows="rows" filename="mcs-table.json" :label="t('tools.wifi-throughput.table', { width, streams, gi })" />
  </CCard>
</template>
