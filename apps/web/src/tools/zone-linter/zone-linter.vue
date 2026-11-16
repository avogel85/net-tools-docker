<script setup lang="ts">
import type { Severity } from './zone-linter.service'
import { refDebounced } from '@vueuse/core'
import { NAlert, NButton, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ResultActions from '@/components/ResultActions.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { lintZone } from './zone-linter.service'

const { t } = useI18n()
const origin = useQueryParam({ name: 'origin', defaultValue: '' })
// Zone text is not put in the URL: zones can be large and may be private.
const text = ref('')
const debounced = refDebounced(text, 250)
const result = computed(() => (debounced.value.trim() ? lintZone(debounced.value, origin.value.trim()) : undefined))
const counts = computed(() => {
  const c: Record<Severity, number> = { error: 0, warning: 0, info: 0 }
  for (const f of result.value?.findings ?? []) c[f.severity]++
  return c
})
const tagType = { error: 'error', warning: 'warning', info: 'info' } as const
const summary = computed(() => {
  const by = new Map<string, number>()
  for (const r of result.value?.records ?? []) by.set(r.type, (by.get(r.type) ?? 0) + 1)
  return [...by].sort((a, b) => b[1] - a[1]).map(([type, count]) => ({ type, count }))
})
const message = (code: string, params?: Record<string, string | number>) => t(`tools.zone-linter.findings.${code}`, params ?? {})
const report = () => (result.value?.findings ?? []).map(f => `${f.line ? `line ${f.line}: ` : ''}[${f.severity}] ${message(f.code, f.params)}`).join('\n')

const EXAMPLE = `$ORIGIN example.com.
$TTL 3600
@       IN SOA  ns1 hostmaster ( 2026111601 900 3600 604800 86400 )
        IN NS   ns1
        IN MX   10 mail.example.com
        IN TXT  "v=spf1 mx -all"
        IN TXT  "v=spf1 include:_spf.example.net ~all"
ns1     IN A    192.0.2.1
www     IN CNAME web
www     IN A    192.0.2.80
web     IN A    192.0.2.80
web 60  IN A    192.0.2.81
`
</script>

<template>
  <CCard>
    <CInputText v-model:value="origin" :label="t('tools.zone-linter.origin')" placeholder="example.com" monospace test-id="origin" />
    <NFormItem :label="t('tools.zone-linter.zone')">
      <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 10, maxRows: 30 }" class="font-mono" :placeholder="t('tools.zone-linter.placeholder')" :input-props="{ 'aria-label': t('tools.zone-linter.zone'), 'spellcheck': false }" data-testid="zone" />
    </NFormItem>
    <NButton secondary size="small" data-testid="example" @click="text = EXAMPLE">
      {{ t('tools.zone-linter.example') }}
    </NButton>
  </CCard>

  <template v-if="result">
    <CCard :title="t('tools.zone-linter.findingsTitle')" data-testid="result">
      <div class="mb-3 flex flex-wrap gap-2">
        <NTag :type="counts.error ? 'error' : 'success'" :bordered="false" data-testid="count-error">
          {{ t('tools.zone-linter.errors', { n: counts.error }) }}
        </NTag>
        <NTag :type="counts.warning ? 'warning' : 'default'" :bordered="false">
          {{ t('tools.zone-linter.warnings', { n: counts.warning }) }}
        </NTag>
        <NTag :bordered="false">
          {{ t('tools.zone-linter.records', { n: result.records.length }) }}
        </NTag>
      </div>
      <NAlert v-if="!result.findings.length" type="success">
        {{ t('tools.zone-linter.clean') }}
      </NAlert>
      <ul v-else class="m-0 list-none p-0 text-sm" data-testid="findings">
        <li v-for="(f, i) in result.findings" :key="i" class="flex items-start gap-2 py-1">
          <NTag size="small" :type="tagType[f.severity]" :bordered="false" class="shrink-0">
            {{ t(`tools.zone-linter.severity.${f.severity}`) }}
          </NTag>
          <span v-if="f.line" class="shrink-0 font-mono opacity-75">{{ t('tools.zone-linter.line', { n: f.line }) }}</span>
          <span class="break-words">{{ message(f.code, f.params) }}</span>
        </li>
      </ul>
      <div class="mt-3">
        <ResultActions :text="report" :json="result" filename="zone-lint.json" />
      </div>
    </CCard>
    <CCard v-if="summary.length" :title="t('tools.zone-linter.summary')">
      <ResultTable :columns="[{ key: 'type', title: t('tools.zone-linter.type'), width: 120 }, { key: 'count', title: t('tools.zone-linter.count'), width: 100 }]" :rows="summary" :actions="false" :label="t('tools.zone-linter.summary')" />
    </CCard>
  </template>
</template>
