<script setup lang="ts">
const auth = useAuthStore()
const route = useRoute()

const status = ref<'verifying' | 'error'>('verifying')

onMounted(async () => {
  const token = route.query.token

  if (typeof token !== 'string' || token === '') {
    status.value = 'error'
    return
  }

  try {
    await auth.verifyMagicLink(token)
    await navigateTo('/')
  } catch {
    status.value = 'error'
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-neutral-950 px-4 text-neutral-100">
    <div class="w-full max-w-sm space-y-8 text-center">
      <h1 class="text-2xl font-semibold">
        Face Value
      </h1>

      <p
        v-if="status === 'verifying'"
        class="text-sm text-neutral-300"
      >
        Signing you in…
      </p>

      <div
        v-else
        class="space-y-4"
      >
        <p class="text-sm text-red-400">
          This sign-in link is invalid or has expired.
        </p>
        <NuxtLink
          to="/login"
          class="inline-block rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-neutral-950 transition-colors hover:bg-emerald-400"
        >
          Back to sign in
        </NuxtLink>
      </div>
    </div>
  </div>
</template>
