import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const installer = readFileSync(`${root}deploy/installer/install.sh`, 'utf8');
const version = installer.match(/^INSTALLER_VERSION="([^"]+)"$/m)?.[1];
if (!version || !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error('INSTALLER_VERSION must be a literal product version without a v prefix');
}
const expected = process.argv[2]?.replace(/^v/, '');
if (expected && expected !== version) {
  throw new Error(`Release version ${expected} does not match installer version ${version}`);
}
if (!installer.includes('DEFAULT_CONFIG_REF="v${INSTALLER_VERSION}"')) {
  throw new Error('Configuration ref must be bound to the installer release tag');
}
for (const [file, images] of [
  ['web.yaml', { panel_image: 'trojanpanelnext-api', ui_image: 'trojanpanelnext-web' }],
  ['node-agent.yaml', { core_image: 'trojanpanelnext-node-agent' }],
]) {
  const yaml = readFileSync(`${root}deploy/installer/examples/${file}`, 'utf8');
  for (const [key, image] of Object.entries(images)) {
    const actual = yaml.match(new RegExp(`^  ${key}: ([^\\s]+)$`, 'm'))?.[1];
    const required = `ghcr.io/1linhao/${image}:${version}`;
    if (actual !== required) {
      throw new Error(`${file}: ${key} must be ${required}, got ${actual}`);
    }
    if (!installer.includes(`ghcr.io/1linhao/${image}:\${INSTALLER_VERSION}`)) {
      throw new Error(`Installer default for ${image} must use INSTALLER_VERSION`);
    }
  }
}
console.log(`Installer, configuration ref and product image tags agree: v${version}`);
