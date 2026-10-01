import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const library = readFileSync(`${root}scripts/deploy/common.sh`, 'utf8');
const entrypoint = readFileSync(`${root}scripts/tp.sh`, 'utf8');
const version = library.match(/^INSTALLER_VERSION="([^"]+)"$/m)?.[1];
if (!version || !/^[1-9]\d*\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error('INSTALLER_VERSION must be a maintained product version without a v prefix');
}
const expected = process.argv[2]?.replace(/^v/, '');
if (expected && expected !== version) {
  throw new Error(`Release version ${expected} does not match script library version ${version}`);
}
for (const [file, name] of [
  ['apps/control-plane/api/model/constant/system.go', 'TrojanPanelVersion'],
  ['apps/node-agent/model/constant/system.go', 'TrojanPanelCoreVersion'],
]) {
  const source = readFileSync(`${root}${file}`, 'utf8');
  if (!new RegExp(`${name}\\s*=\\s*"v${version.replaceAll('.', '\\.')}"`).test(source)) {
    throw new Error(`${file}: public service version must match v${version}`);
  }
}
if (readFileSync(`${root}apps/control-plane/web/public/version`, 'utf8').trim() !== `v${version}`) {
  throw new Error('Web public version must match the product release');
}
if (!library.includes('DEFAULT_CONFIG_REF="v${INSTALLER_VERSION}"') ||
    !library.includes('CONFIG_REF="${TP_RELEASE_REF:-${DEFAULT_CONFIG_REF}}"') ||
    !library.includes('"${CONFIG_REF}" != "${DEFAULT_CONFIG_REF}"')) {
  throw new Error('Configuration templates must use the selected script library release tag');
}
if (library.includes('TP_CONFIG_REF') || library.includes('TP_SCRIPT_REF')) {
  throw new Error('Script library must not accept unversioned ref overrides');
}
if (!entrypoint.includes(`DEFAULT_VERSION="${version}"`) ||
    !entrypoint.includes('/scripts/deploy/')) {
  throw new Error('Entrypoint must default to the current release and download its script library');
}
for (const file of ['config.sh', 'validate.sh', 'install.sh', 'update.sh', 'uninstall.sh', 'quick.sh', 'web.sh', 'node.sh', 'dependencies.sh']) {
  const script = readFileSync(`${root}scripts/deploy/${file}`, 'utf8');
  if (!script.includes(`SCRIPT_VERSION="${version}"`)) {
    throw new Error(`${file} must use script library version ${version}`);
  }
}
for (const [file, images] of [
  ['web.yaml', { panel_image: 'trojanpanelnext-api', ui_image: 'trojanpanelnext-web' }],
  ['node.yaml', { core_image: 'trojanpanelnext-node-agent' }],
]) {
  const yaml = readFileSync(`${root}scripts/deploy/templates/${file}`, 'utf8');
  if (!yaml.includes(`  release: "${version}"`)) {
    throw new Error(`${file}: release must be ${version}`);
  }
  for (const [key, image] of Object.entries(images)) {
    const actual = yaml.match(new RegExp(`^  ${key}: ([^\\s]+)$`, 'm'))?.[1];
    const required = `ghcr.io/1linhao/${image}:${version}`;
    if (actual !== required) {
      throw new Error(`${file}: ${key} must be ${required}, got ${actual}`);
    }
    if (!library.includes(`ghcr.io/1linhao/${image}:\${INSTALLER_VERSION}`)) {
      throw new Error(`Script default for ${image} must use INSTALLER_VERSION`);
    }
  }
}
console.log(`Entrypoint, script library, configuration release and product image tags agree: v${version}`);
