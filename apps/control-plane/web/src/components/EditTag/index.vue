<template>
  <div class="edit-tag-list">
    <liquid-tag
      :key="tag"
      v-for="tag in tags"
      class="edit-tag-item"
      closable
      @close="handleClose(tag)"
    >
      {{ tag }}
    </liquid-tag>
    <liquid-input
      class="edit-tag-input"
      v-if="inputVisible"
      v-model="inputValue"
      ref="saveTagInput"
      compact
      @keyup.enter.native="handleInputConfirm"
      @blur="handleInputConfirm"
    />
    <liquid-button
      icon="plus"
      v-else
      class="edit-tag-add liquid-add-button"
      type="primary"
      aria-label="添加"
      title="添加"
      @click="showInput"
    />
  </div>
</template>

<script>
import { Message } from '@/utils/liquid-feedback'

export default {
  name: 'EditableTagList',
  data() {
    return {
      inputVisible: false,
      inputValue: '',
      tags: [...this.dynamicTagsProps]
    }
  },
  props: {
    dynamicTagsProps: {
      type: Array,
      require: true
    },
    valueCanEmpty: {
      type: Boolean,
      require: false,
      default: false
    }
  },
  watch: {
    dynamicTagsProps(value) {
      this.tags = [...value]
    }
  },
  methods: {
    updateTags(tags) {
      this.tags = tags
      this.$emit('update:dynamicTagsProps', tags)
    },
    handleClose(tag) {
      this.updateTags(this.tags.filter((item) => item !== tag))
    },

    showInput() {
      this.inputVisible = true
      this.$nextTick(() => {
        this.$refs.saveTagInput.focus()
      })
    },

    handleInputConfirm() {
      let inputValue = this.inputValue
      if ((this.valueCanEmpty && inputValue === '') || inputValue) {
        if (this.tags.indexOf(inputValue) === -1) {
          this.updateTags([...this.tags, inputValue])
        } else {
          Message({
            message: this.$t('confirm.alreadyExists').toString(),
            type: 'error',
            duration: 5 * 1000
          })
        }
      }
      this.inputVisible = false
      this.inputValue = ''
    }
  }
}
</script>

<style scoped>
.edit-tag-list {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.edit-tag-item,
.edit-tag-add {
  min-height: 34px;
  height: 34px;
}

.edit-tag-add {
  padding: 0;
}

.edit-tag-input {
  width: 160px;
}
</style>
