<script setup lang="ts">
import type { ProxyInput, Target } from './reverse-proxy-config.service'
import { NButton, NCheckbox, NFormItem, NRadioButton, NRadioGroup, NTabPane, NTabs } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { downloadText } from '@/utils/download'
import { splitList } from '@/utils/ip'
import { generate, TARGETS, validate } from './reverse-proxy-config.service'

const { t } = useI18n()
const hosts = useQueryParam({ name: 'hosts', defaultValue: 'app.example.com' })
const upstream = useQueryParam({ name: 'upstream', defaultValue: 'http://127.0.0.1:3000' })
const tls = useQueryParam({ name: 'tls', defaultValue: 'acme' })
const certPath = useQueryParam({ name: 'cert', defaultValue: '/etc/ssl/app/fullchain.pem' })
const keyPath = useQueryParam({ name: 'key', defaultValue: '/etc/ssl/app/privkey.pem' })
const redirectHttp = useQueryParam({ name: 'redirect', defaultValue: true })
const hsts = useQueryParam({ name: 'hsts', defaultValue: true })
const websockets = useQueryParam({ name: 'ws', defaultValue: true })
const compression = useQueryParam({ name: 'gzip', defaultValue: true })
const securityHeaders = useQueryParam({ name: 'headers', defaultValue: true })
const maxBody = useQueryParam({ name: 'body', defaultValue: '' })
const timeout = useQueryParam({ name: 'timeout', defaultValue: '' })

const input = computed<ProxyInput>(() => ({
  hosts: splitList(hosts.value).map(h => h.toLowerCase()),
  upstream: upstream.value,
  tls: tls.value as ProxyInput['tls'],
  certPath: certPath.value,
  keyPath: keyPath.value,
  redirectHttp: redirectHttp.value,
  hsts: hsts.value,
  websockets: websockets.value,
  compression: compression.value,
  securityHeaders: securityHeaders.value,
  maxBodyMb: Math.max(0, Math.min(10240, Number(maxBody.value) || 0)),
  timeoutS: Math.max(0, Math.min(86400, Number(timeout.value) || 0)),
}))
const errors = computed(() => validate(input.value))
const err = (code: string, msg: string) => (errors.value.includes(code) ? { isValid: false, status: 'error' as const, message: msg } : undefined)
const tab = ref<Target>('nginx')
const configs = computed(() => Object.fromEntries(TARGETS.map(x => [x, generate(input.value, x)])) as Record<Target, string>)
const files: Record<Target, string> = { 'nginx': 'nginx-site.conf', 'caddy': 'Caddyfile', 'traefik': 'traefik-dynamic.yml', 'traefik-labels': 'traefik-labels.yml', 'haproxy': 'haproxy.cfg' }
</script>

<template>
  <CCard>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="hosts" :label="t('tools.reverse-proxy-config.hosts')" placeholder="app.example.com, www.example.com" monospace test-id="hosts" :validation="err('hosts', t('tools.reverse-proxy-config.invalidHosts'))" />
      <CInputText v-model:value="upstream" :label="t('tools.reverse-proxy-config.upstream')" placeholder="http://127.0.0.1:3000" monospace test-id="upstream" :validation="err('upstream', t('tools.reverse-proxy-config.invalidUpstream'))" />
    </div>
    <NFormItem :label="t('tools.reverse-proxy-config.tls')">
      <NRadioGroup v-model:value="tls" name="tls">
        <NRadioButton v-for="m in ['acme', 'files', 'none']" :key="m" :value="m">
          {{ t(`tools.reverse-proxy-config.tlsModes.${m}`) }}
        </NRadioButton>
      </NRadioGroup>
    </NFormItem>
    <div v-if="tls === 'files'" class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="certPath" :label="t('tools.reverse-proxy-config.cert')" monospace test-id="cert" :validation="err('paths', t('tools.reverse-proxy-config.invalidPath'))" />
      <CInputText v-model:value="keyPath" :label="t('tools.reverse-proxy-config.key')" monospace test-id="key" :validation="err('paths', t('tools.reverse-proxy-config.invalidPath'))" />
    </div>
    <div class="mb-3 flex flex-wrap gap-x-6 gap-y-1">
      <NCheckbox v-model:checked="redirectHttp" :disabled="tls === 'none'">
        {{ t('tools.reverse-proxy-config.redirect') }}
      </NCheckbox>
      <NCheckbox v-model:checked="hsts" :disabled="tls === 'none'">
        HSTS
      </NCheckbox>
      <NCheckbox v-model:checked="websockets">
        WebSockets
      </NCheckbox>
      <NCheckbox v-model:checked="compression">
        {{ t('tools.reverse-proxy-config.compression') }}
      </NCheckbox>
      <NCheckbox v-model:checked="securityHeaders">
        {{ t('tools.reverse-proxy-config.securityHeaders') }}
      </NCheckbox>
    </div>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="maxBody" :label="t('tools.reverse-proxy-config.maxBody')" :placeholder="t('tools.reverse-proxy-config.serverDefault')" monospace test-id="body" />
      <CInputText v-model:value="timeout" :label="t('tools.reverse-proxy-config.timeout')" :placeholder="t('tools.reverse-proxy-config.serverDefault')" monospace test-id="timeout" />
    </div>
  </CCard>
  <CCard v-if="!errors.length" :title="t('tools.reverse-proxy-config.config')" data-testid="result">
    <NTabs v-model:value="tab" type="line" animated>
      <NTabPane v-for="x in TARGETS" :key="x" :name="x" :tab="t(`tools.reverse-proxy-config.targets.${x}`)">
        <pre class="m-0 mb-2 overflow-x-auto text-xs font-mono" :data-testid="`config-${x}`">{{ configs[x] }}</pre>
        <div class="flex flex-wrap gap-2">
          <CopyButton :text="configs[x]" />
          <NButton secondary size="small" @click="downloadText(files[x], configs[x], 'text/plain')">
            {{ t('tools.reverse-proxy-config.download', { file: files[x] }) }}
          </NButton>
        </div>
      </NTabPane>
    </NTabs>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.reverse-proxy-config.hint') }}
    </p>
  </CCard>
</template>
