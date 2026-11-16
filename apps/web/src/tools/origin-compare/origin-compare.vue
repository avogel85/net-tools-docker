<script setup lang="ts">
import type { HttpOriginCompare } from './origin-compare.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { normalizeUrl } from '../social-preview/social-preview.service'
import { isValidIp, sortDiffs } from './origin-compare.service'

const { t } = useI18n()
const url = useQueryParam({ name: 'url', defaultValue: '' })
const origin = useQueryParam({ name: 'origin', defaultValue: '' })
const originOk = computed(() => !origin.value || isValidIp(origin.value))
const { data, error, loading, run } = useApi<HttpOriginCompare>('/v1/http/origin-compare')
const submit = () => url.value.trim() && origin.value && originOk.value && run({ url: normalizeUrl(url.value), origin: origin.value.trim() })
onMounted(() => {
  if (url.value && origin.value)
    void submit()
})
const severity: Record<string, 'error' | 'warning' | 'info'> = { 'origin-exposed': 'warning', 'origin-cert-invalid': 'warning', 'content-differs': 'info', 'origin-unreachable': 'info', 'no-cdn-headers': 'info' }
const rows = computed(() => (data.value ? sortDiffs(data.value.diffs) : []).map(d => ({ field: d.field, same: d.same ? '=' : '≠', cdn: d.cdn || '—', origin: d.origin || '—' })))
const columns = computed(() => [
  { key: 'same', title: '', width: 40 },
  { key: 'field', title: t('tools.origin-compare.field'), width: 200 },
  { key: 'cdn', title: t('tools.origin-compare.viaCdn'), mono: true, width: 320 },
  { key: 'origin', title: t('tools.origin-compare.direct'), mono: true, width: 320 },
])
const sides = computed(() => (data.value ? [['viaCdn', data.value.cdn], ['direct', data.value.origin]] as const : []))
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[2fr_1fr]">
        <CInputText v-model:value="url" :label="t('tools.origin-compare.url')" placeholder="https://www.example.com/" monospace test-id="url" />
        <CInputText v-model:value="origin" :label="t('tools.origin-compare.origin')" placeholder="203.0.113.10" monospace test-id="origin" :validation="originOk ? undefined : { isValid: false, status: 'error', message: t('tools.origin-compare.invalidOrigin') }" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.origin-compare.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!url.trim() || !origin || !originOk" data-testid="run">
        {{ t('tools.origin-compare.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <div class="mb-4 flex flex-col gap-2" data-testid="findings">
      <NAlert v-for="f in data.findings" :key="f" :type="severity[f] ?? 'info'">
        {{ t(`tools.origin-compare.findings.${f}`) }}
      </NAlert>
    </div>
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
      <CCard v-for="[label, s] in sides" :key="label" :title="t(`tools.origin-compare.${label}`)">
        <p v-if="s.error" class="m-0 text-sm">
          {{ s.error }}
        </p>
        <template v-else>
          <div class="mb-2 flex flex-wrap items-center gap-2 text-sm">
            <NTag size="small" :type="s.status < 400 ? 'success' : 'warning'" :bordered="false">
              HTTP {{ s.status }}
            </NTag>
            <span class="font-mono">{{ s.address }}</span>
            <span class="opacity-75">{{ t('tools.origin-compare.timing', { ttfb: s.ttfbMs.toFixed(0), total: s.totalMs.toFixed(0) }) }}</span>
          </div>
          <div v-for="(v, k) in s.cdnHeaders" :key="k" class="break-all text-sm font-mono">
            {{ k }}: {{ v }}
          </div>
        </template>
      </CCard>
    </div>
    <CCard :title="t('tools.origin-compare.comparison')" data-testid="result">
      <ResultTable :columns="columns" :rows="rows" filename="origin-compare.json" :label="t('tools.origin-compare.comparison')" />
    </CCard>
  </template>
</template>
