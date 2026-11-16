<script setup lang="ts">
import type { Band, Region } from './wifi-channels.service'
import { NAlert, NCheckbox, NFormItem, NInputNumber, NRadioButton, NRadioGroup, NTag } from 'naive-ui'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import { channels, nonOverlapping, plan, WIDTHS } from './wifi-channels.service'

const { t } = useI18n()
const band = useQueryParam({ name: 'band', defaultValue: '5' })
const region = useQueryParam({ name: 'region', defaultValue: 'eu' })
const width = useQueryParam({ name: 'width', defaultValue: 80 })
const aps = useQueryParam({ name: 'aps', defaultValue: 4 })
const avoidDfs = useQueryParam({ name: 'nodfs', defaultValue: false })
const b = computed(() => band.value as Band)
const r = computed(() => region.value as Region)
watch(b, (v) => {
  if (!WIDTHS[v].includes(width.value))
    width.value = v === '2.4' ? 20 : 80
}, { immediate: true })
const apsModel = computed({
  get: () => aps.value,
  set: (v: number | null) => {
    aps.value = v ?? 1
  },
})
const list = computed(() => channels(b.value, r.value, width.value))
const free = computed(() => nonOverlapping(b.value, r.value, width.value, avoidDfs.value))
const assignments = computed(() => plan(b.value, r.value, width.value, aps.value, avoidDfs.value))
const reuse = computed(() => assignments.value.some(a => a.reuse > 0))
const rows = computed(() => list.value.map(c => ({
  channel: c.width === 20 ? String(c.number) : `${c.number} (${c.covers.join(', ')})`,
  center: `${c.centerMhz} MHz`,
  flags: [c.dfs ? 'DFS' : '', c.weatherRadar ? t('tools.wifi-channels.weather') : ''].filter(Boolean).join(' · ') || '—',
})))
const columns = computed(() => [
  { key: 'channel', title: t('tools.wifi-channels.channel'), mono: true, width: 220 },
  { key: 'center', title: t('tools.wifi-channels.center'), width: 120 },
  { key: 'flags', title: t('tools.wifi-channels.flags'), width: 220 },
])
</script>

<template>
  <CCard>
    <div class="flex flex-wrap gap-x-8">
      <NFormItem :label="t('tools.wifi-channels.band')">
        <NRadioGroup v-model:value="band" name="band">
          <NRadioButton v-for="x in ['2.4', '5', '6']" :key="x" :value="x">
            {{ x }} GHz
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
      <NFormItem :label="t('tools.wifi-channels.region')">
        <NRadioGroup v-model:value="region" name="region">
          <NRadioButton v-for="x in ['eu', 'us']" :key="x" :value="x">
            {{ t(`tools.wifi-channels.regions.${x}`) }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
      <NFormItem :label="t('tools.wifi-channels.width')">
        <NRadioGroup v-model:value="width" name="width">
          <NRadioButton v-for="w in WIDTHS[b]" :key="w" :value="w">
            {{ w }} MHz
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
    </div>
    <div class="flex flex-wrap items-center gap-x-8">
      <NFormItem :label="t('tools.wifi-channels.aps')" class="w-40">
        <NInputNumber v-model:value="apsModel" :min="1" :max="200" :show-button="false" :input-props="{ 'aria-label': t('tools.wifi-channels.aps') }" />
      </NFormItem>
      <NCheckbox v-model:checked="avoidDfs" :disabled="b !== '5'">
        {{ t('tools.wifi-channels.avoidDfs') }}
      </NCheckbox>
    </div>
  </CCard>
  <NAlert v-if="b === '2.4' && width > 20" type="warning" class="mb-4">
    {{ t('tools.wifi-channels.wide24') }}
  </NAlert>
  <CCard :title="t('tools.wifi-channels.plan', { n: free.length })" data-testid="result">
    <div class="flex flex-wrap gap-2">
      <NTag v-for="a in assignments" :key="a.ap" :type="a.channel.dfs ? 'warning' : 'success'" :bordered="false" data-testid="assignment">
        AP {{ a.ap }}: {{ a.channel.number }}{{ a.channel.width > 20 && b !== '2.4' ? ` (${a.channel.width} MHz)` : '' }}
      </NTag>
    </div>
    <p v-if="!assignments.length" class="m-0 text-sm">
      {{ t('tools.wifi-channels.none') }}
    </p>
    <NAlert v-if="reuse" type="info" class="mt-3">
      {{ t('tools.wifi-channels.reuse') }}
    </NAlert>
    <p class="mb-0 mt-3 text-sm">
      {{ t(`tools.wifi-channels.hints.${b}`) }}
    </p>
  </CCard>
  <CCard :title="t('tools.wifi-channels.list', { n: list.length })">
    <ResultTable :columns="columns" :rows="rows" filename="wifi-channels.json" :label="t('tools.wifi-channels.list', { n: list.length })" />
  </CCard>
</template>
