<script setup>
import { ref } from 'vue'
import { ArrowRight, Eye, EyeOff, LoaderCircle } from 'lucide-vue-next'
import './admin-theme.css'

defineProps({ checking: Boolean, loading: Boolean, error: String, username: String, password: String })
defineEmits(['update:username', 'update:password', 'submit'])
const showPassword = ref(false)
</script>

<template>
  <main class="admin-login">
    <div class="login-container">
      <section v-if="checking" class="login-card login-checking" aria-live="polite" aria-busy="true">
        <LoaderCircle class="login-spinner" :size="25" /><p>正在验证登录状态…</p>
      </section>
      <form v-else class="login-card" aria-labelledby="admin-login-title" :aria-busy="loading" @submit.prevent="!loading && $emit('submit')">
        <header class="login-heading"><h1 id="admin-login-title">管理员登录</h1></header>
        <div class="login-field">
          <label for="admin-login-username">账号</label>
          <input id="admin-login-username" :value="username" type="text" autocomplete="username" autocapitalize="none" spellcheck="false" placeholder="请输入管理员账号" required :disabled="loading" @input="$emit('update:username', $event.target.value.trim())" />
        </div>
        <div class="login-field">
          <label for="admin-login-password">密码</label>
          <div class="login-password">
            <input id="admin-login-password" :value="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" placeholder="请输入密码" required :disabled="loading" @input="$emit('update:password', $event.target.value)" />
            <button type="button" class="login-password-toggle" :aria-label="showPassword ? '隐藏密码' : '显示密码'" :aria-pressed="showPassword" aria-controls="admin-login-password" :disabled="loading" @click="showPassword = !showPassword"><component :is="showPassword ? EyeOff : Eye" :size="18" /></button>
          </div>
        </div>
        <p v-if="error" class="login-error" role="alert">{{ error }}</p>
        <button class="login-submit" type="submit" :disabled="loading"><LoaderCircle v-if="loading" class="login-spinner" :size="18" /><span>{{ loading ? '正在登录…' : '登录控制台' }}</span><ArrowRight v-if="!loading" :size="18" /></button>
      </form>
    </div>
  </main>
</template>

<style scoped>
.admin-login { display: grid; place-items: center; min-height: 100dvh; padding: 40px 20px; box-sizing: border-box; background-image: radial-gradient(ellipse at 50% 15%, #e8edff80, transparent 65%); }
.login-container { width: min(100%, 420px); }
.login-card { display: grid; gap: 22px; margin: 0; padding: 32px; border: 1px solid var(--admin-border); border-radius: 14px; background: #fff; box-shadow: 0 12px 36px #24344808, 0 2px 6px #24344803; }
.login-heading { margin-bottom: 2px; }
.login-heading h1 { margin: 0; font-size: 24px; font-weight: 600; line-height: 1.4; letter-spacing: -.4px; color: var(--admin-text); }
.login-field { display: grid; gap: 8px; min-width: 0; }
.login-field label { font-size: 14px; font-weight: 500; color: #46556a; }
.login-field input { box-sizing: border-box; width: 100%; min-width: 0; height: 46px; padding: 0 13px; border: 1px solid #d6deea; border-radius: 8px; background: #fff; color: var(--admin-text); font: inherit; font-size: 15px; }
.login-field input::placeholder { color: #8996a9; }
.login-field input:focus { outline: none; border-color: var(--admin-accent); box-shadow: 0 0 0 3px var(--admin-accent-shadow); }
.login-password { display: grid; grid-template-columns: minmax(0, 1fr) 44px; border: 1px solid #d6deea; border-radius: 8px; overflow: hidden; background: #fff; }
.login-password:focus-within { border-color: var(--admin-accent); box-shadow: 0 0 0 3px var(--admin-accent-shadow); }
.login-password input { height: 44px; border: 0; border-radius: 0; }
.login-password input:focus { box-shadow: none; }
.login-password-toggle { display: grid; place-items: center; width: 44px; min-height: 44px; padding: 0; border: 0; background: transparent; color: #7b899b; cursor: pointer; }
.login-password-toggle:hover:not(:disabled) { background: var(--admin-accent-soft); color: var(--admin-accent); }
.login-password-toggle:focus-visible { outline: 2px solid var(--admin-accent); outline-offset: -5px; border-radius: 7px; }
.login-submit { display: flex; align-items: center; justify-content: center; gap: 10px; width: 100%; min-height: 46px; margin-top: 2px; padding: 0 16px; border: 1px solid var(--admin-accent); border-radius: 8px; background: var(--admin-accent); color: #fff; font: inherit; font-size: 15px; font-weight: 500; cursor: pointer; }
.login-submit:hover:not(:disabled) { background: var(--admin-accent-hover); border-color: var(--admin-accent-hover); }
.login-submit:focus-visible { outline: 2px solid var(--admin-accent); outline-offset: 3px; }
.login-submit:disabled { opacity: .7; cursor: wait; }
.login-field input:disabled, .login-password-toggle:disabled { background: #f5f7fa; cursor: wait; }
.login-error { margin: 0; padding: 10px 12px; border: 1px solid #f3d9d9; border-radius: 8px; background: #fff4f3; color: #b43f39; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.login-checking { place-items: center; padding: 44px 24px; color: var(--admin-muted); font-size: 14px; }
.login-checking p { margin: 0; }
.login-checking > svg { color: var(--admin-accent); }
.login-spinner { animation: login-rotate 1s linear infinite; }
@keyframes login-rotate { to { transform: rotate(360deg); } }
@media (max-width: 480px) { .admin-login { padding: 28px 20px; } .login-card { padding: 26px 22px; gap: 20px; } .login-field input { font-size: 16px; } }
@media (prefers-reduced-motion: reduce) { .login-spinner { animation: none; } }
</style>
