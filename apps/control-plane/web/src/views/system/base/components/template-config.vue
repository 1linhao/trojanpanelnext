<template>
  <div class="app-container">
    <liquid-form
      ref="dataForm"
      :rules="updateRules"
      :model="systemConfig"
      label-position="top"
    >
      <div class="template-mode-switches">
        <liquid-tabs
          id-prefix="subscription-template-client"
          :tabs="clientTabs"
          v-model="activeClient"
          label="订阅模板客户端"
        />
      </div>

      <div :id="`subscription-template-client-panel-${activeClient}`" role="tabpanel" :aria-labelledby="`subscription-template-client-tab-${activeClient}`">
      <client-template-editor
        ref="templateEditor"
        :client-id="activeClient"
        :templates="activeTemplates"
        :active-template-id="activeTemplateId"
        :template-select-label="$t('config.templateSelect')"
        :template-name-label="$t('config.templateName')"
        :format-button-label="$t('config.formatTemplate')"
        @select-template="selectTemplate"
        @update-template="updateTemplate"
      />
      </div>

      <liquid-form-item class="actions">
        <liquid-button type="primary" icon="check" @click="updateData">
          {{ $t('table.confirm') }}
        </liquid-button>
      </liquid-form-item>
    </liquid-form>
  </div>
</template>

<script>
import { updateSystemById } from '@/api/system'
import ClientTemplateEditor from '@/components/ClientTemplateEditor'
import jsYaml from 'js-yaml'

export default {
  name: 'TemplateConfigForm',
  components: { ClientTemplateEditor },
  inject: {
    systemConfig: {
      from: 'systemConfigModel'
    }
  },
  data() {
    const nameRules = [
      {
        required: true,
        message: this.$t('valid.templateName'),
        trigger: ['change', 'blur']
      },
      {
        min: 1,
        max: 32,
        message: this.$t('valid.templateNameRange'),
        trigger: ['change', 'blur']
      }
    ]
    return {
      activeClient: 'clash-meta',
      clients: [
        { value: 'clash-meta', label: 'Clash.Meta' },
        { value: 'sing-box', label: 'sing-box' },
        { value: 'xray', label: 'Xray' }
      ],
      activeTemplateIds: {
        'clash-meta': 'default',
        'sing-box': 'tun',
        xray: 'default'
      },
      updateRules: {
        clashTemplateName: nameRules,
        singBoxTunTemplateName: nameRules,
        singBoxOutboundTemplateName: nameRules,
        xrayTemplateName: nameRules,
        clashRule: [
          {
            min: 0,
            max: 102400,
            message: this.$t('valid.clashRuleRange'),
            trigger: ['change', 'blur']
          }
        ],
        singBoxTun: [
          {
            min: 0,
            max: 102400,
            message: this.$t('valid.singBoxRuleRange'),
            trigger: ['change', 'blur']
          }
        ],
        singBoxOutbound: [
          {
            min: 0,
            max: 102400,
            message: this.$t('valid.singBoxRuleRange'),
            trigger: ['change', 'blur']
          }
        ],
        xrayTemplate: [
          {
            min: 0,
            max: 10240,
            message: this.$t('valid.xrayTemplateRange'),
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  computed: {
    templateCatalog() {
      return {
        'clash-meta': [
          {
            id: 'default',
            name: this.systemConfig.clashTemplateName,
            nameProp: 'clashTemplateName',
            content: this.systemConfig.clashRule,
            contentProp: 'clashRule',
            contentLabel: this.$t('config.clashRule'),
            languageLabel: 'YAML',
            format: 'yaml'
          }
        ],
        'sing-box': [
          {
            id: 'tun',
            name: this.systemConfig.singBoxTunTemplateName,
            nameProp: 'singBoxTunTemplateName',
            content: this.systemConfig.singBoxTunEntity,
            contentProp: 'singBoxTun',
            contentLabel: this.$t('config.singBoxTunTemplate'),
            languageLabel: 'JSON',
            format: 'json'
          },
          {
            id: 'outbound',
            name: this.systemConfig.singBoxOutboundTemplateName,
            nameProp: 'singBoxOutboundTemplateName',
            content: this.systemConfig.singBoxOutboundEntity,
            contentProp: 'singBoxOutbound',
            contentLabel: this.$t('config.singBoxOutboundTemplate'),
            languageLabel: 'JSON',
            format: 'json'
          }
        ],
        xray: [
          {
            id: 'default',
            name: this.systemConfig.xrayTemplateName,
            nameProp: 'xrayTemplateName',
            content: this.systemConfig.xrayTemplateEntity,
            contentProp: 'xrayTemplate',
            contentLabel: this.$t('config.xrayTemplate'),
            languageLabel: 'JSON',
            format: 'json'
          }
        ]
      }
    },
    clientTabs() {
      return this.clients
    },
    activeTemplates() {
      return this.templateCatalog[this.activeClient]
    },
    activeTemplateId() {
      return this.activeTemplateIds[this.activeClient]
    }
  },
  methods: {
    selectTemplate(templateId) {
      this.$set(this.activeTemplateIds, this.activeClient, templateId)
    },
    updateTemplate({ templateId, patch }) {
      const template = this.activeTemplates.find(
        (candidate) => candidate.id === templateId
      )
      if (!template) return
      if (Object.prototype.hasOwnProperty.call(patch, 'name')) {
        this.systemConfig[template.nameProp] = patch.name
      }
      if (Object.prototype.hasOwnProperty.call(patch, 'content')) {
        this.systemConfig[
          template.format === 'json'
            ? `${template.contentProp}Entity`
            : template.contentProp
        ] = patch.content
      }
    },
    serializeEditor(field) {
      const value = this.systemConfig[field]
      const entity = typeof value === 'object' ? value : JSON.parse(value)
      return JSON.stringify(entity)
    },
    updateData() {
      if (!this.$refs.templateEditor.validate()) return
      try {
        if (String(this.systemConfig.clashRule || '').trim()) {
          jsYaml.safeLoad(this.systemConfig.clashRule)
        }
      } catch (error) {
        this.$message.error(this.$t('valid.yamlFormat').toString())
        return
      }
      try {
        this.systemConfig.singBoxTun = this.serializeEditor(
          'singBoxTunEntity'
        )
        this.systemConfig.singBoxOutbound = this.serializeEditor(
          'singBoxOutboundEntity'
        )
        this.systemConfig.xrayTemplate = this.serializeEditor(
          'xrayTemplateEntity'
        )
      } catch (error) {
        this.$message.error(this.$t('valid.jsonFormat').toString())
        return
      }
      this.$refs.dataForm.validate((valid) => {
        if (!valid) return
        updateSystemById(Object.assign({}, this.systemConfig)).then(() => {
          this.$nextTick(() => {
            this.$refs.dataForm.clearValidate()
          })
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.modifySuccess'),
            type: 'success',
            duration: 2000
          })
        })
      })
    }
  }
}
</script>

<style scoped>
.template-mode-switches {
  display: grid;
  justify-items: start;
  margin-bottom: 20px;
}

.template-mode-switches > .liquid-tabs {
  margin-bottom: 0;
}

.actions {
  margin-top: 20px;
}

</style>
