package kernelconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"trojan-panel-core/internal/atomicfile"
	"trojan-panel-core/model/constant"
)

// SyncKernelCertificates rewrites certificate paths embedded in existing
// kernel configurations. The installer changes config.ini when tls_mode moves
// between acme and external, but restored kernel documents are otherwise kept.
func SyncKernelCertificates(crtPath, keyPath string) error {
	if strings.TrimSpace(crtPath) == "" || strings.TrimSpace(keyPath) == "" {
		return nil
	}
	return syncKernelCertificates(constant.CoreBasePath, crtPath, keyPath)
}

func syncKernelCertificates(basePath, crtPath, keyPath string) error {
	targets := []struct {
		dir    string
		accept func(string) bool
		mutate func(map[string]any, string, string) bool
	}{
		{constant.XrayPath, func(name string) bool { _, _, ok := ParseXrayConfigName(name); return ok }, syncXrayCertificate},
		{constant.NaiveProxyPath, func(name string) bool { _, ok := ParseSingleConfigName(name); return ok }, syncNaiveProxyCertificate},
		{constant.Hysteria2Path, func(name string) bool { _, ok := ParseSingleConfigName(name); return ok }, syncHysteria2Certificate},
	}

	var problems []string
	for _, target := range targets {
		dir := filepath.Join(basePath, target.dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				problems = append(problems, fmt.Sprintf("read %s: %v", dir, err))
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !target.accept(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if err := rewriteJSONFile(path, func(document map[string]any) bool {
				return target.mutate(document, crtPath, keyPath)
			}); err != nil {
				problems = append(problems, fmt.Sprintf("sync %s: %v", path, err))
			}
		}
	}
	if len(problems) != 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func rewriteJSONFile(path string, mutate func(map[string]any) bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		return err
	}
	if !mutate(document) {
		return nil
	}
	content, err = json.MarshalIndent(document, "", "    ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	return atomicfile.Write(path, content, 0644)
}

func syncXrayCertificate(document map[string]any, crtPath, keyPath string) bool {
	changed := false
	inbounds, _ := document["inbounds"].([]any)
	for _, value := range inbounds {
		inbound, _ := value.(map[string]any)
		stream, _ := inbound["streamSettings"].(map[string]any)
		if stream == nil || stream["security"] != "tls" {
			continue
		}
		tlsSettings, _ := stream["tlsSettings"].(map[string]any)
		if tlsSettings == nil {
			tlsSettings = make(map[string]any)
			stream["tlsSettings"] = tlsSettings
		}
		certificates, _ := tlsSettings["certificates"].([]any)
		if len(certificates) == 0 {
			tlsSettings["certificates"] = []any{map[string]any{"certificateFile": crtPath, "keyFile": keyPath}}
			changed = true
			continue
		}
		for _, certificateValue := range certificates {
			certificate, _ := certificateValue.(map[string]any)
			if certificate == nil {
				continue
			}
			changed = setString(certificate, "certificateFile", crtPath) || changed
			changed = setString(certificate, "keyFile", keyPath) || changed
		}
	}
	return changed
}

func syncNaiveProxyCertificate(document map[string]any, crtPath, keyPath string) bool {
	apps := childMap(document, "apps")
	tls := childMap(apps, "tls")
	certificates := childMap(tls, "certificates")
	loadFiles, _ := certificates["load_files"].([]any)
	if len(loadFiles) == 0 {
		certificates["load_files"] = []any{map[string]any{"certificate": crtPath, "key": keyPath}}
		return true
	}
	changed := false
	for _, value := range loadFiles {
		pair, _ := value.(map[string]any)
		if pair == nil {
			continue
		}
		changed = setString(pair, "certificate", crtPath) || changed
		changed = setString(pair, "key", keyPath) || changed
	}
	return changed
}

func syncHysteria2Certificate(document map[string]any, crtPath, keyPath string) bool {
	tls := childMap(document, "tls")
	changed := setString(tls, "cert", crtPath)
	return setString(tls, "key", keyPath) || changed
}

func childMap(parent map[string]any, key string) map[string]any {
	if child, ok := parent[key].(map[string]any); ok {
		return child
	}
	child := make(map[string]any)
	parent[key] = child
	return child
}

func setString(document map[string]any, key, value string) bool {
	if document[key] == value {
		return false
	}
	document[key] = value
	return true
}
