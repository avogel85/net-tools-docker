<script setup lang="ts">
import { NAlert, NFormItem, NInputNumber } from 'naive-ui'
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultActions from '@/components/ResultActions.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CSelect from '@/ui/c-select.vue'
import { calculate, FIBERS, OPTICS } from './fiber-budget.service'

const { t } = useI18n()
const fiber = useQueryParam({ name: 'fiber', defaultValue: 'os2-1310' })
const optic = useQueryParam({ name: 'optic', defaultValue: '10gbase-lr' })
const lengthM = useQueryParam({ name: 'length', defaultValue: 2000 })
const connectors = useQueryParam({ name: 'connectors', defaultValue: 2 })
const connectorDb = useQueryParam({ name: 'conndb', defaultValue: 0.5 })
const splices = useQueryParam({ name: 'splices', defaultValue: 2 })
const spliceDb = useQueryParam({ name: 'splicedb', defaultValue: 0.1 })
const marginDb = useQueryParam({ name: 'margin', defaultValue: 3 })
const tx = useQueryParam({ name: 'tx', defaultValue: -8.2 })
const rx = useQueryParam({ name: 'rx', defaultValue: -14.4 })
const overload = useQueryParam({ name: 'overload', defaultValue: 0.5 })

// Picking an optic fills in its typical values; they stay editable.
watch(optic, (id) => {
  const o = OPTICS.find(x => x.id === id)
  if (!o)
    return
  tx.value = o.tx
  rx.value = o.rx
  overload.value = o.overload
  if (o.reach[fiber.value] === undefined)
    fiber.value = Object.keys(o.reach)[0]!
})

const fiberOptions = computed(() => FIBERS.map(f => ({ value: f.id, label: t(`tools.fiber-budget.fibers.${f.id}`, { db: f.dbPerKm }) })))
const opticOptions = computed(() => [...OPTICS.map(o => ({ value: o.id, label: o.id.toUpperCase() })), { value: 'custom', label: t('tools.fiber-budget.custom') }])
const result = computed(() => calculate({ fiber: fiber.value, lengthM: lengthM.value, connectors: connectors.value, connectorDb: connectorDb.value, splices: splices.value, spliceDb: spliceDb.value, marginDb: marginDb.value, tx: tx.value, rx: rx.value, overload: overload.value, optic: optic.value }))
function num(r: { value: number }) {
  return computed({
    get: () => r.value,
    set: (v: number | null) => {
      r.value = v ?? 0
    },
  })
}
const fields = [
  { key: 'length', model: num(lengthM), min: 0, max: 200000, step: 100 },
  { key: 'connectors', model: num(connectors), min: 0, max: 50, step: 1 },
  { key: 'connectorDb', model: num(connectorDb), min: 0, max: 3, step: 0.05 },
  { key: 'splices', model: num(splices), min: 0, max: 200, step: 1 },
  { key: 'spliceDb', model: num(spliceDb), min: 0, max: 1, step: 0.05 },
  { key: 'margin', model: num(marginDb), min: 0, max: 10, step: 0.5 },
  { key: 'tx', model: num(tx), min: -40, max: 10, step: 0.1 },
  { key: 'rx', model: num(rx), min: -45, max: 0, step: 0.1 },
  { key: 'overload', model: num(overload), min: -30, max: 10, step: 0.1 },
]
const alertType = { ok: 'success', tight: 'warning', fail: 'error' } as const
const rows = computed(() => [
  ['fiberLoss', result.value.fiberLoss],
  ['connectorLoss', result.value.connectorLoss],
  ['spliceLoss', result.value.spliceLoss],
  ['totalLoss', result.value.totalLoss],
  ['powerBudget', result.value.powerBudget],
  ['margin', result.value.margin],
  ['rxPower', result.value.rxPower],
] as const)
</script>

<template>
  <CCard>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CSelect v-model:value="optic" :label="t('tools.fiber-budget.optic')" :options="opticOptions" />
      <CSelect v-model:value="fiber" :label="t('tools.fiber-budget.fiber')" :options="fiberOptions" />
    </div>
    <div class="grid grid-cols-2 gap-x-4 md:grid-cols-3">
      <NFormItem v-for="f in fields" :key="f.key" :label="t(`tools.fiber-budget.fields.${f.key}`)">
        <NInputNumber v-model:value="f.model.value" :min="f.min" :max="f.max" :step="f.step" :show-button="false" :input-props="{ 'aria-label': t(`tools.fiber-budget.fields.${f.key}`) }" :data-testid="`f-${f.key}`" />
      </NFormItem>
    </div>
    <p class="mb-0 mt-0 text-sm">
      {{ t('tools.fiber-budget.hint') }}
    </p>
  </CCard>
  <NAlert :type="alertType[result.verdict]" class="mb-4" :title="t(`tools.fiber-budget.verdict.${result.verdict}`)" data-testid="verdict">
    <ul v-if="result.warnings.length" class="m-0 pl-5">
      <li v-for="w in result.warnings" :key="w">
        {{ t(`tools.fiber-budget.warnings.${w}`) }}
      </li>
    </ul>
  </NAlert>
  <CCard :title="t('common.result')" data-testid="result">
    <dl class="grid grid-cols-[1fr_auto] m-0 max-w-md gap-x-6 gap-y-1 text-sm">
      <template v-for="[k, v] in rows" :key="k">
        <dt :class="k === 'totalLoss' || k === 'margin' ? 'font-semibold' : ''">
          {{ t(`tools.fiber-budget.rows.${k}`) }}
        </dt>
        <dd class="m-0 text-right font-mono" :data-testid="`r-${k}`">
          {{ v.toFixed(2) }} {{ k === 'rxPower' ? 'dBm' : 'dB' }}
        </dd>
      </template>
    </dl>
    <div class="mt-3">
      <ResultActions :text="() => rows.map(([k, v]) => `${t(`tools.fiber-budget.rows.${k}`)}: ${v}`).join('\n')" :json="result" filename="fiber-budget.json" />
    </div>
  </CCard>
</template>
