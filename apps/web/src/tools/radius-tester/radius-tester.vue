<script setup lang="ts">
import type { LanRadius } from './radius-tester.service'
import { NAlert, NButton, NFormItem, NInput, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError as ApiErr, apiPost } from '@/api/client'
import ApiError from '@/components/ApiError.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { verdict } from './radius-tester.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const port = useQueryParam({ name: 'port', defaultValue: '1812' })
const mode = useQueryParam({ name: 'mode', defaultValue: 'status' })
const username = useQueryParam({ name: 'user', defaultValue: '' })
// Secret and password: in memory only, sent in a POST body, never in the URL.
const secret = ref('')
const password = ref('')
const portOk = computed(() => /^\d{1,5}$/.test(port.value) && Number(port.value) >= 1 && Number(port.value) <= 65535)
const canRun = computed(() => host.value.trim() && secret.value && portOk.value && (mode.value === 'status' || username.value.trim()))
const data = shallowRef<LanRadius>()
const error = shallowRef<ApiErr>()
const loading = ref(false)
async function submit() {
  if (!canRun.value)
    return
  loading.value = true
  error.value = undefined
  try {
    data.value = await apiPost<LanRadius>('/v1/lan/radius', { host: host.value.trim(), port: Number(port.value), secret: secret.value, mode: mode.value, username: username.value.trim(), password: password.value })
  }
  catch (e) {
    data.value = undefined
    error.value = e instanceof ApiErr ? e : new ApiErr('NETWORK_ERROR', String(e))
  }
  finally {
    loading.value = false
  }
}
const v = computed(() => (data.value ? verdict(data.value) : undefined))
const alertType = { ok: 'success', rejected: 'warning', untrusted: 'error', silent: 'warning', closed: 'error' } as const
</script>

<template>
  <CCard>
    <form class="contents" autocomplete="off" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_8rem]">
        <CInputText v-model:value="host" :label="t('tools.radius-tester.host')" placeholder="192.168.1.10" monospace test-id="host" />
        <CInputText v-model:value="port" :label="t('tools.radius-tester.port')" placeholder="1812" monospace test-id="port" :validation="portOk ? undefined : { isValid: false, status: 'error', message: '1–65535' }" />
      </div>
      <NFormItem :label="t('tools.radius-tester.secret')">
        <NInput v-model:value="secret" type="password" show-password-on="click" :input-props="{ 'aria-label': t('tools.radius-tester.secret'), 'autocomplete': 'off' }" data-testid="secret" />
      </NFormItem>
      <NFormItem :label="t('tools.radius-tester.mode')">
        <NRadioGroup v-model:value="mode" name="mode">
          <NRadioButton value="status" data-testid="mode-status">
            {{ t('tools.radius-tester.modes.status') }}
          </NRadioButton>
          <NRadioButton value="auth" data-testid="mode-auth">
            {{ t('tools.radius-tester.modes.auth') }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
      <div v-if="mode === 'auth'" class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
        <CInputText v-model:value="username" :label="t('tools.radius-tester.username')" monospace test-id="username" />
        <NFormItem :label="t('tools.radius-tester.password')">
          <NInput v-model:value="password" type="password" show-password-on="click" :input-props="{ 'aria-label': t('tools.radius-tester.password'), 'autocomplete': 'off' }" data-testid="password" />
        </NFormItem>
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.radius-tester.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!canRun" data-testid="run">
        {{ t('tools.radius-tester.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <NAlert v-if="data && v" :type="alertType[v]" :title="t(`tools.radius-tester.verdict.${v}`)" class="mb-4" data-testid="result">
    <p class="m-0">
      {{ t(`tools.radius-tester.explain.${v}`, { result: t(`tools.radius-tester.results.${data.result}`), ms: data.rttMs.toFixed(1) }) }}
    </p>
    <p v-for="(m, i) in data.replyMessages" :key="i" class="m-0 mt-1 font-mono">
      Reply-Message: {{ m }}
    </p>
  </NAlert>
</template>
