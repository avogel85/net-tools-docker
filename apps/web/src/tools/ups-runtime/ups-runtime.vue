<script setup lang="ts">
import { NAlert, NInputNumber } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import { formatDuration, runtime } from './ups-runtime.service'

const { t } = useI18n()
function num(name: string, defaultValue: number) {
  const p = useQueryParam({ name, defaultValue: String(defaultValue) })
  return computed({ get: () => Number(p.value), set: (v: number | null) => (p.value = String(v ?? 0)) })
}
const wh = num('wh', 0)
const volts = num('v', 12)
const ah = num('ah', 100)
const efficiency = num('eff', 85)
const depth = num('dod', 80)
const watts = num('w', 0)
const va = num('va', 0)
const powerFactor = num('pf', 0.9)
const result = computed(() => runtime({ wh: wh.value, volts: volts.value, ah: ah.value, efficiency: efficiency.value, depth: depth.value, watts: watts.value, va: va.value, powerFactor: powerFactor.value }))
const fields = [
  ['wh', wh],
  ['volts', volts],
  ['ah', ah],
  ['efficiency', efficiency],
  ['depth', depth],
  ['watts', watts],
  ['va', va],
  ['powerFactor', powerFactor],
] as const
</script>

<template>
  <CCard>
    <div class="grid gap-3 sm:grid-cols-2">
      <label v-for="[k, model] in fields" :key="k" class="flex flex-col gap-1 text-sm">
        {{ t(`tools.ups-runtime.fields.${k}`) }}
        <NInputNumber v-model:value="model.value" :min="0" :data-testid="k" />
      </label>
    </div>
    <p class="mb-0 mt-3 text-xs nt-muted">
      {{ t('tools.ups-runtime.note') }}
    </p>
  </CCard>
  <CCard v-if="result" :title="t('common.result')" data-testid="result">
    <NAlert type="info">
      {{ t('tools.ups-runtime.result', { time: formatDuration(result.minutes), wh: Math.round(result.usableWh), w: Math.round(result.loadW) }) }}
    </NAlert>
  </CCard>
</template>
