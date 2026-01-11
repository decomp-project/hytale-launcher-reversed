<script lang="ts" setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { BrowserOpenURL } from '@wailsjs/runtime/runtime'
import * as App from '@wailsjs/go/app/App'
import Logo from '@/components/Logo.vue'
import HyButton from '@/components/HyButton.vue'
import LauncherVersion from '@/components/LauncherVersion.vue'

const router = useRouter()
const isLoading = ref(false)

const openInIcon = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABEAAAARCAYAAAA7bUf6AAAACXBIWXMAAAsTAAALEwEAmpwYAAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAC8SURBVHgBrZK9EQIhEIX5Cwgp4SJmCCnBCmzFDjxLsAM7sQTMSC3hKgB3Ax08gT3v7iXsDPDxeLs8xjiybz2dczcscE8IcWaERG8TYGNK6cIIqfJCCwSOWM9R10kJyjlfN0HAycA5P66GIAC+codyWAWpATDoedjqX8C7AWXYTSdSSgOLqQFQZfubEGvtA8I8QDnNAT8gnMrK1H4UQjCMkKIOeO8n6gES0pPW+oTromGjtAuE90Jdql2cvAClzFdGDZMFsAAAAABJRU5ErkJggg=='

async function signIn() {
  if (isLoading.value) return

  isLoading.value = true
  try {
    // Get OAuth URL from backend (starts loopback server)
    const loginUrl = await App.Login()
    // Open in browser
    BrowserOpenURL(loginUrl)
  } catch (error) {
    console.error('Login error:', error)
    router.push({ name: 'error', query: { error: String(error) } })
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="login">
    <div class="login__container text--center">
      <Logo class="login__signin-logo" />
      <div class="login__signin-container">
        <HyButton
          class="login__sign-in-button"
          @click="signIn"
          :disabled="isLoading"
          type="primary"
        >
          {{ $t('login.sign_in') }}
          <img :src="openInIcon" class="login__sign-in-img" :alt="$t('login.open_in_icon')" draggable="false" />
        </HyButton>
        <p class="login__sign-in-message">{{ $t('login.sign_in_message') }}</p>
      </div>
      <div class="login__signin-footer">
        <LauncherVersion class="login__version-text" />
      </div>
      <div class="login__background-container"></div>
    </div>
  </div>
</template>

<style scoped>
.login {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  flex-grow: 1;
  height: 100%;
}

.login__container {
  max-width: 395px;
  background-image: url('@/assets/images/signin-background.png');
  background-size: cover;
  background-position: center;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.login__sign-in-button {
  display: flex;
  flex-direction: row;
  justify-content: center;
  align-items: center;
  width: 100%;
}

.login__sign-in-img {
  height: 16px;
  margin-left: 6px;
}

.login__sign-in-message {
  font-weight: 300;
}

.login__signin-logo {
  margin-top: 88px;
  width: 254px;
}

.login__signin-logo :deep(img) {
  height: 133px;
}

.login__signin-container {
  margin-top: 70px;
  padding: 0 80px;
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  align-items: center;
  height: 100%;
}

.login__background-container {
  flex-grow: 1;
}

.login__signin-footer {
  width: 100%;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  padding-bottom: 20px;
}

.login__version-text {
  padding-right: 45px;
}
</style>
