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
      <p v-if="activeClient !== 'sing-box'" class="template-help">
        {{ $t(activeClient === 'xray' ? 'config.xrayTemplateUse' : 'config.clashTemplateUse') }}
      </p>
      <liquid-form-item v-if="activeClient === 'xray'" :label="$t('config.activeXray')">
        <liquid-select v-model="library.activeXray">
          <option v-for="item in activeTemplates" :key="item.id" :value="item.id">{{ item.name }}</option>
        </liquid-select>
      </liquid-form-item>
      <client-template-editor
        ref="templateEditor"
        :key="activeClient + '/' + activeTemplateId"
        :can-create="activeClient !== 'sing-box' && activeTemplates.length < 17"
        :can-delete="activeClient !== 'sing-box' && activeTemplateId !== 'default' && !(activeClient === 'xray' && library.activeXray === activeTemplateId)"
        :create-label="$t('config.createTemplate')"
        :delete-label="$t('config.deleteTemplate')"
        @create-template="createTemplate"
        @delete-template="deleteTemplate"
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
    savedConfig: {
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
    const saved = this.savedConfig.templateLibrary || {}
    return {
      systemConfig: { ...this.savedConfig },
      library: {
        clash: (saved.clash || []).map((item) => ({ ...item })),
        xray: (saved.xray || []).map((item) => ({ ...item })),
        activeXray: saved.activeXray || 'default'
      },
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
          },
          ...this.namedEditors('clash')
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
          },
          ...this.namedEditors('xray')
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
    namedEditors(group) {
      const yaml = group === 'clash'
      return this.library[group].map((entry) => ({
        ...entry,
        entry,
        format: yaml ? 'yaml' : 'json',
        languageLabel: yaml ? 'YAML' : 'JSON',
        contentLabel: this.$t(yaml ? 'config.clashRule' : 'config.xrayTemplate')
      }))
    },
    createTemplate() {
      const group = this.activeClient === 'xray' ? 'xray' : 'clash'
      const source = this.activeTemplates.find((entry) => entry.id === this.activeTemplateId)
      const bytes = crypto.getRandomValues(new Uint8Array(12))
      const id = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
      const content = typeof source.content === 'string' ? source.content.trim() || '{}' : JSON.stringify(source.content, null, 2)
      this.library[group].push({ id, name: this.$t('config.newTemplate'), content })
      this.selectTemplate(id)
    },
    deleteTemplate(template) {
      const entries = this.activeClient === 'xray' ? this.library.xray : this.library.clash
      const index = entries.findIndex((t) => t.id === template.id)
      if (index < 0) return
      entries.splice(index, 1)
      this.selectTemplate('default')
    },
    selectTemplate(templateId) {
      this.$set(this.activeTemplateIds, this.activeClient, templateId)
    },
    updateTemplate({ templateId, patch }) {
      const template = this.activeTemplates.find(
        (candidate) => candidate.id === templateId
      )
      if (!template) return
      if (template.entry) {
        Object.assign(template.entry, patch)
        return
      }
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
      for (const client of ['clash-meta', 'xray']) {
        for (const template of this.templateCatalog[client]) {
          let error
          if (!template.name || !template.name.trim() || Array.from(template.name).length > 32) {
            error = this.$t('valid.templateNameRange')
          } else if (client === 'xray' && template.entry && new Blob([template.content]).size > 10240) {
            error = this.$t('valid.namedXraySize', { name: template.name })
          } else if (template.entry) {
            try {
              const value = client === 'xray' ? JSON.parse(template.content) : jsYaml.safeLoad(template.content)
              if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error()
            } catch {
              error = this.$t(client === 'xray' ? 'valid.jsonFormat' : 'valid.yamlFormat')
            }
          }
          if (error) {
            this.activeClient = client
            this.selectTemplate(template.id)
            this.$message.error(error)
            return
          }
        }
      }
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
        const templateLibrary = JSON.parse(JSON.stringify(this.library))
        const data = { ...this.systemConfig, templateLibrary }
        // Avoid rewriting legacy template files when only the catalog changes.
        if (data.clashRule === this.savedConfig.clashRule) delete data.clashRule
        for (const key of ['singBoxTun', 'singBoxOutbound', 'xrayTemplate']) {
          if (data[key] === JSON.stringify(JSON.parse(this.savedConfig[key]))) delete data[key]
        }
        updateSystemById(data, true).then(() => {
          Object.assign(this.savedConfig, data)
          this.$nextTick(() => {
            this.$refs.dataForm.clearValidate()
          })
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.modifySuccess'),
            type: 'success',
            duration: 2000
          })
        }).catch(() => {
          // Request feedback already shows the server or network error. Keep the editor open.
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

.template-help {
  color: var(--supporting-text-ink);
  margin: 0 0 16px;
}

.actions {
  margin-top: 20px;
}

</style>
