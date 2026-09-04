<template>
  <div class="auth">
    <liquid-theme-toggle class="auth-theme-toggle" />
    <ui-panel variant="auth" motion-role="shared" motion-key="auth-primary">
      <div class="auth-brand">
        <panel-logo />
        <h1>{{ branding.systemName }}</h1>
      </div>
      <liquid-form
        ref="loginForm"
        :model="loginForm"
        :rules="loginRules"
        class="auth-form"
        auto-complete="on"
        @submit.native.prevent
      >
        <liquid-form-item prop="username" label="用户名">
          <div class="fld"><input
              ref="username"
              v-model="loginForm.username"
              placeholder="6–20 位字母或数字"
              autocomplete="username" /></div>
        </liquid-form-item>
        <liquid-form-item prop="pass" label="密码">
          <div class="fld">
            <span class="password-field"
              ><input
                ref="pass"
                v-model="loginForm.pass"
                :type="passwordType"
                placeholder="6–20 位字母或数字"
                autocomplete="current-password"
                @keyup.enter="handleLogin" /><button
                type="button"
                class="field-icon"
                :aria-label="passwordType === 'password' ? '显示密码' : '隐藏密码'"
                :aria-pressed="String(passwordType !== 'password')"
                @click="showPwd"
              >
                <app-icon :name="passwordType === 'password' ? 'eye-off' : 'eye'"
                /></button></span></div>
        </liquid-form-item>
        <div v-if="captchaEnable" class="captcha-row">
          <liquid-form-item prop="captchaCode" label="验证码">
              <div class="fld"><input
                v-model="loginForm.captchaCode"
                placeholder="输入右侧字符"
                @keyup.enter="handleLogin" /></div></liquid-form-item
          ><button
            type="button"
            class="captcha-img"
            title="点击刷新"
            @click="handleCaptchaGenerate"
          >
            <img alt="captcha" :src="captchaImg" />
          </button>
        </div>
        <button
          type="button"
          class="cap primary auth-submit"
          :disabled="loading"
          @click="handleLogin"
        >
          {{ loading ? '登录中…' : '登 录' }}
        </button>
        <template v-if="registerEnable === 1"
          ><div class="auth-divider">没有账号？</div>
          <button type="button" class="cap auth-submit" @click="goRegister">
            注册新账号
          </button></template
        >
      </liquid-form>
    </ui-panel>
  </div>
</template>

<script>
import { panelBranding, loadPanelSettings } from '@/utils/panel-branding'
import PanelLogo from '@/components/PanelLogo'
import { generateCaptcha } from '@/api/account'
import LiquidThemeToggle from '@/components/LiquidThemeToggle'

export default {
  name: 'LoginPage',
  components: { LiquidThemeToggle, PanelLogo },
  data() {
    return {
      loginForm: {
        username: '',
        pass: '',
        captchaId: '',
        captchaCode: ''
      },
      captchaImg: '',
      loginRules: {
        username: [
          {
            required: true,
            message: this.$t('valid.username'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.usernameRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.usernameElement'),
            trigger: ['change', 'blur']
          }
        ],
        pass: [
          {
            required: true,
            message: this.$t('valid.pass'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.passRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.passElement'),
            trigger: ['change', 'blur']
          }
        ],
        captchaCode: [
          {
            required: true,
            message: this.$t('valid.code'),
            trigger: ['change', 'blur']
          }
        ]
      },
      loading: false,
      passwordType: 'password',
      redirect: undefined,
      registerEnable: 0,
      branding: panelBranding,
      captchaEnable: 0
    }
  },
  watch: {
    $route: {
      handler: function (route) {
        this.redirect = route.query && route.query.redirect
      },
      immediate: true
    }
  },
  created() {
    this.setting()
    this.handleCaptchaGenerate()
  },
  methods: {
    handleCaptchaGenerate() {
      generateCaptcha().then((response) => {
        this.loginForm.captchaId = response.data.captchaId
        this.captchaImg = response.data.captchaImg
      })
    },
    showPwd() {
      if (this.passwordType === 'password') {
        this.passwordType = ''
      } else {
        this.passwordType = 'password'
      }
      this.$nextTick(() => {
        this.$refs.pass.focus()
      })
    },
    handleLogin() {
      this.$refs.loginForm.validate((valid) => {
        if (valid) {
          this.loading = true
          this.$store
            .dispatch('account/login', this.loginForm)
            .then(() => {
              this.$router
                .push({ path: this.redirect || '/' })
                .catch(() => true)
              this.loading = false
            })
            .catch(() => {
              this.loading = false
              this.handleCaptchaGenerate()
            })
        } else {
          // console.log('error submit!!')
          return false
        }
      })
    },
    goRegister() {
      this.$router.push('/register').catch(() => true)
    },
    setting() {
      loadPanelSettings().then((response) => {
        const { data } = response
        this.registerEnable = data.registerEnable
        this.captchaEnable = data.captchaEnable
      }).catch(() => {})
    }
  }
}
</script>
