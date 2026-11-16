<script setup lang="ts">
import type { LdapCheck } from './ldap-check.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { functionalLevel, rootDseRows } from './ldap-check.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const mode = useQueryParam({ name: 'mode', defaultValue: 'both' })
const { data, error, loading, run } = useApi<LdapCheck>('/v1/ldap/check')
const submit = () => host.value.trim() && run({ host: host.value.trim(), mode: mode.value })
onMounted(() => {
  if (host.value)
    void submit()
})
const modes = computed(() => ['both', 'ldap', 'ldaps'].map(v => ({ value: v, label: t(`tools.ldap-check.modes.${v}`) })))
const display = (key: string, v: string) => (/functionality$/i.test(key) ? functionalLevel(v) : v)
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_14rem]">
        <CInputText v-model:value="host" :label="t('tools.ldap-check.host')" placeholder="dc1.corp.example.com" monospace test-id="host" />
        <CSelect v-model:value="mode" :label="t('tools.ldap-check.mode')" :options="modes" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.ldap-check.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim()" data-testid="run">
        {{ t('tools.ldap-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <div v-if="data.warnings.length" class="mb-4 flex flex-col gap-2">
      <NAlert v-for="w in data.warnings" :key="w" type="warning">
        {{ t(`tools.ldap-check.warnings.${w}`) }}
      </NAlert>
    </div>
    <CCard v-for="ep in data.endpoints" :key="ep.mode" :title="`${ep.mode.toUpperCase()} — ${ep.address}`" data-testid="result">
      <div class="mb-3 flex flex-wrap gap-2">
        <NTag :type="ep.error ? 'error' : 'success'" :bordered="false" data-testid="status">
          {{ ep.error ? t(`tools.ldap-check.errors.${ep.error}`) : t('tools.ldap-check.answered') }}
        </NTag>
        <NTag v-if="ep.mode === 'ldap' && !ep.error" :type="ep.startTls ? 'success' : 'warning'" :bordered="false">
          {{ ep.startTls ? t('tools.ldap-check.startTls') : t('tools.ldap-check.noStartTls') }}
        </NTag>
        <NTag v-if="ep.tls" :type="ep.tls.verified ? 'success' : 'warning'" :bordered="false">
          {{ ep.tls.version }} · {{ ep.tls.verified ? t('tools.ldap-check.trusted') : t('tools.ldap-check.untrusted') }}
        </NTag>
      </div>
      <p v-if="ep.tls?.chain[0]" class="mt-0 break-all text-sm">
        {{ t('tools.ldap-check.cert') }}: <span class="font-mono">{{ ep.tls.chain[0].subject }}</span> · {{ t('tools.ldap-check.daysLeft', { n: ep.tls.chain[0].daysLeft }) }}
      </p>
      <p v-if="ep.resultMessage" class="mt-0 break-all text-sm font-mono">
        {{ ep.resultMessage }}
      </p>
      <dl v-if="Object.keys(ep.rootDse).length" class="grid grid-cols-1 m-0 gap-x-4 gap-y-1 text-sm md:grid-cols-[16rem_1fr]">
        <template v-for="row in rootDseRows(ep.rootDse)" :key="row.key">
          <dt class="font-semibold font-mono">
            {{ row.key }}
          </dt>
          <dd class="m-0 break-all font-mono">
            <div v-for="(v, i) in row.values" :key="i">
              {{ display(row.key, v) }}
            </div>
          </dd>
        </template>
      </dl>
    </CCard>
    <ResultActions :text="() => data!.endpoints.map(e => `${e.mode}\t${e.address}\t${e.error || 'ok'}`).join('\n')" :json="data" filename="ldap-check.json" />
  </template>
</template>
