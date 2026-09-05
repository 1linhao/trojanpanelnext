# @tp-ui/material-frosted

[简体中文](README.md) | English

The production frosted-glass material for TrojanPanel Next. It implements the complete material variable contract and changes visuals through semantic surfaces without altering component DOM.

## Responsibilities

- Provides light and dark modes with blue, violet, emerald, and amber palettes.
- Styles canvas, panel, raised, overlay, control, and navigation surfaces.
- Owns color, transparency, blur, border highlights, shadows, state colors, and browser chrome metadata.
- Includes a fallback for browsers without `backdrop-filter`.

## Style entries

| Entry | Purpose |
| --- | --- |
| `material.css` | Standalone integration lab |
| `production.css` | Complete production application material |
| `overlay.css` | Dialog and overlay material variables |

## Verify

```bash
npm run check --workspace @tp-ui/material-frosted
```
