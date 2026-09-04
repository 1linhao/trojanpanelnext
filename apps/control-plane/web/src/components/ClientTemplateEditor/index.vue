<template>
  <section class="client-template-editor">
    <div
      class="client-template-editor__identity"
      :class="{ 'has-template-select': hasMultipleTemplates }"
    >
      <liquid-form-item
        v-if="hasMultipleTemplates"
        class="client-template-editor__select-field"
        :label="templateSelectLabel"
      >
        <liquid-select
          class="template-config-editor__template-select"
          :value="activeTemplateId"
          @input="$emit('select-template', $event)"
        >
          <option
            v-for="template in templates"
            :key="template.id"
            :value="template.id"
          >
            {{ template.name }}
          </option>
        </liquid-select>
      </liquid-form-item>

      <liquid-form-item
        class="client-template-editor__name-field"
        :label="templateNameLabel"
        :prop="activeTemplate.nameProp"
      >
        <liquid-input
          :value="activeTemplate.name"
          clearable
          @input="updateTemplate({ name: $event })"
        />
      </liquid-form-item>

      <div
        v-if="canCreate || canDelete || $scopedSlots['management-actions']"
        class="client-template-editor__management"
      >
        <slot
          name="management-actions"
          :client-id="clientId"
          :active-template="activeTemplate"
        />
        <liquid-button
          v-if="canCreate"
          @click="$emit('create-template', { clientId })"
        >
          {{ createLabel }}
        </liquid-button>
        <liquid-button
          v-if="canDelete"
          type="danger"
          @click="$emit('delete-template', activeTemplate)"
        >
          {{ deleteLabel }}
        </liquid-button>
      </div>
    </div>

    <liquid-form-item
      :label="activeTemplate.contentLabel"
      :prop="activeTemplate.contentProp"
    >
      <liquid-code-editor
        ref="codeEditor"
        class="subscription-config-editor"
        :value="activeTemplate.content"
        :language-label="activeTemplate.languageLabel"
        :format="activeTemplate.format"
        :format-button-label="formatButtonLabel"
        :format-error-prefix="$t(activeTemplate.format === 'yaml' ? 'valid.yamlFormat' : 'valid.jsonFormat').toString()"
        @input="updateTemplate({ content: $event })"
      />
    </liquid-form-item>
  </section>
</template>

<script>
import LiquidCodeEditor from '@/components/LiquidCodeEditor'

export default {
  name: 'ClientTemplateEditor',
  components: { LiquidCodeEditor },
  props: {
    clientId: { type: String, required: true },
    templates: {
      type: Array,
      required: true,
      validator: (templates) => templates.length > 0
    },
    activeTemplateId: { type: String, required: true },
    templateSelectLabel: { type: String, default: '配置模板' },
    templateNameLabel: { type: String, default: '模板名称' },
    formatButtonLabel: { type: String, default: '格式化' },
    canCreate: { type: Boolean, default: false },
    canDelete: { type: Boolean, default: false },
    createLabel: { type: String, default: '新增模板' },
    deleteLabel: { type: String, default: '删除模板' }
  },
  computed: {
    hasMultipleTemplates() {
      return this.templates.length > 1
    },
    activeTemplate() {
      return (
        this.templates.find((template) => template.id === this.activeTemplateId) ||
        this.templates[0]
      )
    }
  },
  methods: {
    validate() {
      return !this.$refs.codeEditor || this.$refs.codeEditor.validate()
    },
    updateTemplate(patch) {
      this.$emit('update-template', {
        clientId: this.clientId,
        templateId: this.activeTemplate.id,
        patch
      })
    }
  }
}
</script>

<style scoped>
.client-template-editor__identity {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0 18px;
}

.client-template-editor__identity.has-template-select {
  grid-template-columns: minmax(180px, 0.72fr) minmax(240px, 1.28fr);
}

.client-template-editor__select-field,
.client-template-editor__name-field {
  min-width: 0;
}

.client-template-editor__management {
  display: flex;
  grid-column: 1 / -1;
  flex-wrap: wrap;
  gap: 8px;
  margin: -4px 0 18px;
}

.subscription-config-editor {
  --ui-editor-body-min-height: 320px;
  --ui-editor-mobile-body-min-height: 260px;
}

@media (max-width: 720px) {
  .client-template-editor__identity.has-template-select {
    grid-template-columns: 1fr;
  }
}
</style>
