export default function accountImportRows(source) {
  const json = source.trim()
  const values = JSON.parse(json)
  if (!Array.isArray(values) || !values.length) throw new Error('Expected a non-empty array')

  // Keep the original JSON for retries so large integer fields retain their precision.
  const rows = []
  let start = 1
  let depth = 0
  let quoted = false
  for (let i = 1; i < json.length; i++) {
    const character = json[i]
    if (quoted && character === '\\') {
      i++
    } else if (character === '"') {
      quoted = !quoted
    } else if (!quoted) {
      if (depth === 0 && (character === ',' || character === ']')) {
        rows.push({ number: rows.length + 1, source: json.slice(start, i).trim() })
        start = i + 1
      } else if (character === '[' || character === '{') {
        depth++
      } else if (character === ']' || character === '}') {
        depth--
      }
    }
  }
  return rows
}
