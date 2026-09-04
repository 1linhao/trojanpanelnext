<template>
  <div class="profile-form">
    <liquid-form
      ref="dataForm"
      :rules="updateRules"
      :model="temp"
      label-position="left"
    >
      <liquid-form-item :label="$t('table.oldPass')" prop="oldPass">
        <liquid-input
          v-model="temp.oldPass"
          type="password"
          :placeholder="$t('table.oldPass')"
          clearable
        />
      </liquid-form-item>
      <liquid-form-item :label="$t('table.newPass')" prop="newPassOne">
        <liquid-input
          v-model="temp.newPassOne"
          type="password"
          :placeholder="$t('table.newPass')"
          clearable
        />
      </liquid-form-item>
      <liquid-form-item :label="$t('table.confirmNewPass')" prop="newPass">
        <liquid-input
          v-model="temp.newPass"
          type="password"
          :placeholder="$t('table.confirmNewPass')"
          clearable
        />
      </liquid-form-item>
      <liquid-form-item>
        <liquid-button type="primary" @click="updateData()"
          >{{ $t('table.confirm') }}
        </liquid-button>
      </liquid-form-item>
    </liquid-form>
  </div>
</template>

<script>
import { updateAccountPass } from '@/api/account'
import { setting } from '@/api/system'

export default {
  name: 'ModifyPass',
  data() {
    const validatePass = (rule, value, callback) => {
      if (this.temp.newPassOne !== this.temp.newPass) {
        callback(new Error(this.$t('valid.passNotSame')))
      } else {
        callback()
      }
    }
    const validateOldPassRequired = (rule, value, callback) => {
      if (!value) {
        callback(new Error(this.$t('table.oldPassRequired')))
      } else {
        callback()
      }
    }
    return {
      temp: {
        username: this.$store.getters.username,
        email: undefined,
        oldPass: '',
        newPassOne: '',
        newPass: ''
      },
      emailEnable: 0,
      updateRules: {
        email: [
          {
            min: 4,
            max: 64,
            message: this.$t('valid.emailRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern:
              /^([A-Za-z0-9_.-])+@(163.com|126.com|qq.com|gmail.com)$/,
            message: this.$t('valid.emailElement'),
            trigger: ['change', 'blur']
          }
        ],
        oldPass: [
          {
            required: true,
            message: this.$t('table.oldPassRequired'),
            validator: validateOldPassRequired,
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
        newPassOne: [
          {
            required: true,
            message: this.$t('valid.passNew'),
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
        newPass: [
          {
            required: true,
            message: this.$t('valid.passNew'),
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
        ]
      }
    }
  },
  created() {
    this.setting()
  },
  methods: {
    resetTemp() {
      this.temp = {
        username: this.$store.getters.username,
        email: undefined,
        oldPass: '',
        newPassOne: '',
        newPass: ''
      }
    },
    setting() {
      setting().then((response) => {
        const { data } = response
        this.emailEnable = data.emailEnable
      })
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = Object.assign({}, this.temp)
          updateAccountPass(tempData).then(() => {
            this.resetTemp()
            this.$nextTick(() => {
              this.$refs['dataForm'].clearValidate()
            })
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.updateAccountPass'),
              type: 'success',
              duration: 2000
            })
          })
        }
      })
    }
  }
}
</script>

<style scoped></style>
