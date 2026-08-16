<script setup lang="ts">
const auth = useAuthStore()

const email = ref('')
const status = ref<'idle' | 'sending' | 'sent' | 'error'>('idle')

async function onSubmit() {
  status.value = 'sending'
  try {
    await auth.requestMagicLink(email.value.trim())
    status.value = 'sent'
  } catch {
    status.value = 'error'
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-neutral-950 px-4 text-neutral-100">
    <div class="w-full max-w-sm space-y-8">
      <div class="space-y-2 text-center">
        <h1 class="text-2xl font-semibold">
          Face Value
        </h1>
        <p class="text-sm text-neutral-400">
          Sign in with your email
        </p>
      </div>

      <form
        v-if="status !== 'sent'"
        class="space-y-3"
        @submit.prevent="onSubmit"
      >
        <div>
          <label
            for="email"
            class="block text-sm text-neutral-400"
          >Email</label>
          <input
            id="email"
            v-model="email"
            type="email"
            required
            placeholder="you@example.com"
            autocomplete="email"
            class="mt-1 w-full rounded-md bg-neutral-800 px-3 py-2 text-sm text-neutral-100 placeholder:text-neutral-500"
          >
        </div>

        <button
          type="submit"
          :disabled="status === 'sending'"
          class="w-full rounded-md bg-emerald-500 px-4 py-2 text-sm font-medium text-neutral-950 transition-colors hover:bg-emerald-400 disabled:opacity-50"
        >
          {{ status === 'sending' ? 'Sending…' : 'Send sign-in link' }}
        </button>

        <p
          v-if="status === 'error'"
          class="text-sm text-red-400"
        >
          Something went wrong. Please try again.
        </p>
      </form>

      <p
        v-else
        class="text-center text-sm text-neutral-300"
      >
        If that email is allowed to sign in, a link has been sent. Check your
        inbox and click the link to continue.
      </p>
    </div>
  </div>
</template>
