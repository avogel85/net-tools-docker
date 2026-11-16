<script setup lang="ts">
import type { DnsTakeover, Status } from './takeover-check.service'
import { NAlert, NButton, NCheckbox, NTag } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { counts, sortChecks } from './takeover-check.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const names = useQueryParam({ name: 'names', defaultValue: '' })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const { data, error, loading, run } = useApi<DnsTakeover>('/v1/dns/takeover')
const submit = () => domain.value && validation.value.isValid && run({ domain: domain.value, names: names.value || undefined })
onMounted(() => {
  if (domain.value)
    void submit()
})
const showMissing = ref(false)
const rows = computed(() => (data.value ? sortChecks(data.value.checks, showMissing.value) : []))
const summary = computed(() => (data.value ? counts(data.value.checks) : {}))
const tagType: Record<Status, 'error' | 'warning' | 'success' | 'default' | 'info'> = { 'vulnerable': 'error', 'dangling': 'warning', 'error': 'warning', 'ok': 'success', 'no-cname': 'default', 'nxdomain': 'default' }
const evidence = (e: string) => (e === 'http-skipped' ? t('tools.takeover-check.httpSkipped') : e)
const report = () => rows.value.map(r => `${r.status}\t${r.name}\t${[r.name, ...r.chain].join(' → ')}\t${r.service}\t${evidence(r.evidence)}`).join('\n')
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <CInputText v-model:value="names" :label="t('tools.takeover-check.names')" placeholder="www, blog, shop.example.com" monospace test-id="names" />
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.takeover-check.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!domain || !validation.isValid" data-testid="run">
        {{ t('tools.takeover-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <NAlert v-if="summary.vulnerable" type="error" class="mb-4" data-testid="verdict">
      {{ t('tools.takeover-check.vulnerable', { n: summary.vulnerable }) }}
    </NAlert>
    <NAlert v-else-if="summary.dangling" type="warning" class="mb-4" data-testid="verdict">
      {{ t('tools.takeover-check.dangling', { n: summary.dangling }) }}
    </NAlert>
    <NAlert v-else type="success" class="mb-4" data-testid="verdict">
      {{ t('tools.takeover-check.clean', { n: data.checks.length - (summary.nxdomain ?? 0) }) }}
    </NAlert>
    <CCard :title="t('common.result')" data-testid="result">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <NTag v-for="(n, s) in summary" :key="s" size="small" :type="tagType[s as Status]" :bordered="false">
          {{ t(`tools.takeover-check.status.${s}`) }}: {{ n }}
        </NTag>
        <NCheckbox v-model:checked="showMissing" class="ml-2">
          {{ t('tools.takeover-check.showMissing') }}
        </NCheckbox>
      </div>
      <ul class="m-0 list-none p-0 text-sm">
        <li v-for="r in rows" :key="r.name" class="border-b nt-border border-b-solid py-2 last:border-b-0">
          <div class="flex flex-wrap items-center gap-2">
            <NTag size="small" :type="tagType[r.status]" :bordered="false">
              {{ t(`tools.takeover-check.status.${r.status}`) }}
            </NTag>
            <span class="break-all font-mono">{{ r.name }}</span>
            <span v-if="r.service" class="opacity-75">{{ r.service }}</span>
          </div>
          <div v-if="r.chain.length" class="mt-1 break-all font-mono opacity-80">
            → {{ r.chain.join(' → ') }}
          </div>
          <div v-if="r.evidence" class="mt-1">
            {{ evidence(r.evidence) }}
          </div>
        </li>
      </ul>
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.takeover-check.via', { resolver: data.resolver }) }}
      </p>
      <div class="mt-3">
        <ResultActions :text="report" :json="data" filename="takeover-check.json" />
      </div>
    </CCard>
  </template>
</template>
