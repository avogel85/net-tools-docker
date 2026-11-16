<script setup lang="ts">
import type { TlsAcme } from './acme-check.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { CAS, isValidCertName, verdict } from './acme-check.service'

const { t, te } = useI18n()
const name = useQueryParam({ name: 'name', defaultValue: '' })
const ca = useQueryParam({ name: 'ca', defaultValue: 'letsencrypt' })
const validation = useValidation({ source: name, rules: [{ validator: isValidCertName, message: t('tools.acme-check.invalidName') }] })
const { data, error, loading, run } = useApi<TlsAcme>('/v1/tls/acme')
const submit = () => name.value && validation.value.isValid && run({ name: name.value.trim(), ca: ca.value })
onMounted(() => {
  if (name.value)
    void submit()
})
const caOptions = computed(() => CAS.map(c => ({ value: c, label: t(`tools.acme-check.cas.${c}`) })))
const v = computed(() => (data.value ? verdict(data.value) : undefined))
const alertType = { ready: 'success', warnings: 'warning', blocked: 'error' } as const
function finding(code: string): string {
  const params = { ca: data.value ? t(`tools.acme-check.cas.${data.value.ca}`) : '', status: code.replace('http-status-', '') }
  return te(`tools.acme-check.findings.${code}`) ? t(`tools.acme-check.findings.${code}`, params) : t('tools.acme-check.findings.http-status', params)
}
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[2fr_1fr]">
        <CInputText v-model:value="name" :label="t('tools.acme-check.name')" placeholder="www.example.com" :validation="validation" monospace test-id="name" />
        <CSelect v-model:value="ca" :label="t('tools.acme-check.ca')" :options="caOptions" />
      </div>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!name || !validation.isValid" data-testid="run">
        {{ t('tools.acme-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data && v">
    <NAlert :type="alertType[v]" class="mb-4" :title="t(`tools.acme-check.verdict.${v}`)" data-testid="verdict">
      <ul v-if="data.problems.length || data.warnings.length" class="m-0 pl-5">
        <li v-for="p in data.problems" :key="p">
          {{ finding(p) }}
        </li>
        <li v-for="w in data.warnings" :key="w">
          {{ finding(w) }}
        </li>
      </ul>
    </NAlert>
    <CCard :title="t('tools.acme-check.details')" data-testid="result">
      <dl class="grid grid-cols-1 m-0 gap-x-4 gap-y-3 text-sm md:grid-cols-[12rem_1fr]">
        <dt class="font-semibold">
          {{ t('tools.acme-check.addresses') }}
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ [...data.a, ...data.aaaa].join(', ') || '—' }}
        </dd>
        <dt class="font-semibold">
          CAA
        </dt>
        <dd class="m-0">
          <NTag size="small" :type="data.caaAllowed ? 'success' : 'error'" :bordered="false">
            {{ data.caaAllowed ? t('tools.acme-check.allowed') : t('tools.acme-check.denied') }}
          </NTag>
          <span class="ml-2 break-all font-mono">{{ data.caaName ? `${data.caaName}: ${[...data.caaIssue.map(i => `issue ${i}`), ...data.caaWild.map(i => `issuewild ${i}`)].join(', ')}` : t('tools.acme-check.noCaa') }}</span>
        </dd>
        <dt class="font-semibold">
          HTTP-01
        </dt>
        <dd class="m-0">
          <template v-if="data.httpChecked">
            <div v-for="(h, i) in data.httpHops" :key="i" class="break-all font-mono">
              {{ h.url }} → {{ h.error || h.status }}
            </div>
          </template>
          <span v-else>{{ t('tools.acme-check.httpSkipped') }}</span>
        </dd>
        <dt class="font-semibold">
          DNS-01
        </dt>
        <dd class="m-0 break-all font-mono">
          <div>_acme-challenge.{{ data.name }}{{ data.challengeCname ? ` → CNAME ${data.challengeCname}` : '' }}</div>
          <div v-for="(txt, i) in data.challengeTxt" :key="i">
            TXT "{{ txt }}"
          </div>
        </dd>
      </dl>
      <div class="mt-3">
        <ResultActions :text="() => [...data!.problems, ...data!.warnings].map(finding).join('\n')" :json="data" filename="acme-check.json" />
      </div>
    </CCard>
  </template>
</template>
