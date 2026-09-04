<template>
  <div
    class="liquid-code-editor"
    :class="{ 'is-focused': focused, 'is-error': error }"
  >
    <div class="liquid-code-editor__toolbar">
      <span>{{ languageLabel }}</span>
      <button v-if="processor" type="button" @click="formatContent">
        {{ formatButtonLabel }}
      </button>
    </div>
    <textarea
      v-bind="controlAttrs"
      :value="text"
      :aria-label="$attrs['aria-label'] || `${languageLabel} 配置编辑器`"
      :aria-invalid="error ? 'true' : controlAttrs['aria-invalid']"
      :aria-describedby="describedBy"
      spellcheck="false"
      autocomplete="off"
      @input="handleInput"
      @focus="focused = true"
      @blur="handleBlur"
    />
    <small v-if="error" :id="localErrorId" role="alert">{{ error }}</small>
  </div>
</template>

<script>
import jsYaml from 'js-yaml'
import emitter from '@/mixins/liquid-control-emitter'
import formControl from '@/mixins/liquid-form-control'

function toText(value) {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}

function formatYamlText(text) {
  // js-yaml 3 does not expose a comment-preserving CST. Keep the user's YAML
  // representation (including comments, anchors, aliases and scalar styles)
  // and limit formatting to representation-safe whitespace normalization.
  return text.split(/\r?\n/).map((line) => line.replace(/[ \t]+$/, '')).join('\n').replace(/\n*$/, '\n')
}

// WEB-032: JSON and YAML are equal language capabilities on the same editor
// shell. Each processor owns parse, stringify and error wording for its
// language; neither is derived through the other.
const languageProcessors = {
  json: {
    parse(text) {
      try {
        return { value: JSON.parse(text), error: null }
      } catch (error) {
        return { value: null, error }
      }
    },
    stringify(value) {
      return JSON.stringify(value, null, 2)
    }
  },
  yaml: {
    parse(text) {
      try {
        return { value: jsYaml.safeLoad(text), error: null }
      } catch (error) {
        return { value: null, error }
      }
    },
    stringify(value, text) {
      return formatYamlText(text)
    }
  }
}

export default {
  name: 'LiquidCodeEditor',
  mixins: [formControl, emitter],
  inheritAttrs: false,
  props: {
    value: {
      type: [Object, Array, String],
      default: ''
    },
    languageLabel: {
      type: String,
      default: 'TEXT'
    },
    format: {
      type: String,
      default: ''
    },
    formatButtonLabel: {
      type: String,
      default: '格式化'
    },
    formatErrorPrefix: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      text: toText(this.value),
      focused: false,
      error: ''
    }
  },
  computed: {
    processor() {
      return languageProcessors[this.format] || null
    },
    errorPrefix() {
      if (this.formatErrorPrefix) return this.formatErrorPrefix
      return this.format === 'yaml' ? 'YAML 格式错误' : `${this.format.toUpperCase()} 格式错误`
    },
    localErrorId() {
      return `liquid-code-editor-error-${this._uid}`
    },
    describedBy() {
      return [this.controlAttrs['aria-describedby'], this.error ? this.localErrorId : '']
        .filter(Boolean).join(' ') || undefined
    }
  },
  watch: {
    value: {
      deep: true,
      handler(value) {
        const next = toText(value)
        if (!this.focused && next !== this.text) this.text = next
      }
    }
  },
  methods: {
    handleInput(event) {
      this.text = event.target.value
      this.error = ''
      this.$emit('input', this.text)
    },
    handleBlur() {
      this.focused = false
      this.validate()
      this.dispatch('LiquidFormItem', 'liquid.form.blur', [this.text])
    },
    validate() {
      if (!this.processor || !this.text.trim()) {
        this.error = ''
        return true
      }
      const { error } = this.processor.parse(this.text)
      this.error = error ? `${this.errorPrefix}：${error.message || error.reason}` : ''
      return !error
    },
    formatContent() {
      if (!this.text.trim()) return
      const { value, error } = this.processor.parse(this.text)
      if (error) {
        this.error = `${this.errorPrefix}：${error.message || error.reason}`
        return
      }
      // Preserve formatting of scalars; only structure is re-dumped.
      this.text = this.processor.stringify(value, this.text)
      this.error = ''
      this.$emit('input', this.text)
    }
  }
}
</script>

<style scoped>
.liquid-code-editor {
  width: 100%;
  overflow: hidden;
  border: 1px solid var(--control-border);
  border-radius: 16px;
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft);
  transition: border-color var(--ui-motion-fast) var(--ui-motion-easing-standard), box-shadow var(--ui-motion-fast) var(--ui-motion-easing-standard);
}

.liquid-code-editor.is-focused {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent),
    inset 0 1px 0 var(--spec-soft);
}

.liquid-code-editor.is-error {
  border-color: var(--bad-fg);
}

.liquid-code-editor__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 38px;
  padding: 0 10px 0 14px;
  border-bottom: 1px solid var(--hairline);
  color: var(--supporting-text-ink);
  background: var(--glass-soft);
  font-size: 12px;
  line-height: 1.2;
  letter-spacing: 0.08em;
}

.liquid-code-editor__toolbar button {
  min-height: 30px;
  padding: 0 11px;
  line-height: 1.2;
  letter-spacing: 0;
}

.liquid-code-editor textarea {
  display: block;
  width: 100%;
  min-height: var(--ui-editor-body-min-height, 320px);
  padding: 14px;
  border: 0;
  outline: 0;
  resize: none;
  color: var(--ink);
  background: transparent;
  font: 13px/1.6 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas,
    "Liberation Mono", monospace;
  tab-size: 2;
}

.liquid-code-editor small {
  display: block;
  padding: 0 14px 10px;
  color: var(--bad-fg);
  font-size: 12px;
}

@media (max-width: 720px) {
  .liquid-code-editor textarea {
    min-height: var(
      --ui-editor-mobile-body-min-height,
      var(--ui-editor-body-min-height, 260px)
    );
  }
}
</style>
