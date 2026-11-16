import type { ApiError as ApiErrorType } from '@/api/client'
import { ref, shallowRef } from 'vue'
import { ApiError, apiGet } from '@/api/client'

/** Runs one /v1/homelab/* endpoint; the server reads its targets from the local config file. */
export function useHomelab<T>(path: string) {
  const loading = ref(false)
  const data = shallowRef<T>()
  const error = shallowRef<ApiErrorType>()
  async function run(query = '') {
    loading.value = true
    error.value = undefined
    try {
      data.value = await apiGet<T>(`/v1/homelab/${path}${query}`)
    }
    catch (e) {
      error.value = e instanceof ApiError ? e : new ApiError('BAD_RESPONSE', String(e))
    }
    finally {
      loading.value = false
    }
  }
  return { loading, data, error, run }
}
