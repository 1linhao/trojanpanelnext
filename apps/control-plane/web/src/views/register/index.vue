<template>
  <div class="auth">
    <liquid-theme-toggle class="auth-theme-toggle" />
    <ui-panel variant="auth" motion-role="shared" motion-key="auth-primary">
      <div class="auth-brand">
        <panel-logo />
        <h1>创建账号</h1>
        <p class="ui-supporting-text">{{ branding.systemName }}</p>
      </div>
      <liquid-form
        ref="registerForm"
        :model="registerForm"
        :rules="registerRules"
        class="auth-form"
        auto-complete="on"
        @submit.native.prevent
      >
        <liquid-form-item prop="username" label="用户名">
          <div class="fld"><input
              ref="username"
              v-model="registerForm.username"
              placeholder="6–20 位字母或数字"
              autocomplete="username" /></div>
        </liquid-form-item>
        <liquid-form-item prop="passOne" label="密码">
          <div class="fld">
            <span class="password-field"
              ><input
                ref="pass"
                v-model="registerForm.passOne"
                :type="passwordType"
                placeholder="6–20 位字母或数字"
                autocomplete="new-password" /><button
                type="button"
                class="field-icon"
                :aria-label="passwordType === 'password' ? '显示密码' : '隐藏密码'"
                :aria-pressed="String(passwordType !== 'password')"
                @click="showPwd"
              >
                <app-icon :name="passwordType === 'password' ? 'eye-off' : 'eye'"
                /></button></span></div>
        </liquid-form-item>
        <liquid-form-item prop="pass" label="确认密码">
            <div class="fld"><input
              v-model="registerForm.pass"
              :type="passwordType"
              placeholder="再次输入密码"
              autocomplete="new-password"
              @keyup.enter="handleRegister" /></div>
        </liquid-form-item>
        <div v-if="captchaEnable" class="captcha-row">
          <liquid-form-item prop="captchaCode" label="验证码">
              <div class="fld"><input
                v-model="registerForm.captchaCode"
                placeholder="输入右侧字符"
                @keyup.enter="handleRegister" /></div></liquid-form-item
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
          @click="handleRegister"
        >
          {{ loading ? '提交中…' : '提交注册' }}
        </button>
        <div class="auth-divider">已有账号？</div>
        <button type="button" class="cap auth-submit" @click="goLogin">
          返回登录
        </button>
      </liquid-form>
    </ui-panel>
  </div>
</template>

<script>
import { generateCaptcha } from '@/api/account'
import { panelBranding, loadPanelSettings } from '@/utils/panel-branding'
import PanelLogo from '@/components/PanelLogo'
import LiquidThemeToggle from '@/components/LiquidThemeToggle'

export default {
  name: 'RegisterPage',
  components: { LiquidThemeToggle, PanelLogo },
  data() {
    const validatePass = (rule, value, callback) => {
      if (this.registerForm.passOne !== this.registerForm.pass) {
        callback(new Error(this.$t('valid.passNotSame')))
      } else {
        callback()
      }
    }
    const validateUsername = (rule, value, callback) => {
      if (this.registerForm.username.trim().indexOf('admin') >= 0) {
        callback(new Error(this.$t('valid.usernameNotExistAdmin')))
      } else {
        callback()
      }
    }
    return {
      registerForm: {
        username: '',
        passOne: '',
        pass: '',
        captchaId: '',
        captchaCode: ''
      },
      branding: panelBranding,
      captchaImg: '',
      captchaEnable: 0,
      registerRules: {
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
          },
          {
            validator: validateUsername,
            trigger: ['change', 'blur']
          }
        ],
        passOne: [
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
          },
          {
            validator: validatePass,
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
      passwordType: 'password'
    }
  },
  created() {
    this.handleCaptchaGenerate()
    this.setting()
  },
  methods: {
    handleCaptchaGenerate() {
      generateCaptcha().then((response) => {
        this.registerForm.captchaId = response.data.captchaId
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
    handleRegister() {
      this.$refs.registerForm.validate((valid) => {
        if (valid) {
          this.loading = true
          this.$store
            .dispatch('account/register', this.registerForm)
            .then(() => {
              this.$router.push({ path: '/login' })
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
    goLogin() {
      this.$router.push('/login').catch(() => true)
    },
    setting() {
      loadPanelSettings().then((response) => {
        const { data } = response
        this.captchaEnable = data.captchaEnable
      }).catch(() => {})
    }
  }
}
</script>
