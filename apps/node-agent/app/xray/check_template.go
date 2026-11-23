package xray

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
	"trojan-panel-core/util"
)

var ErrTemplateCheckUnavailable = errors.New("Xray template checking unavailable")

var templateCheckSlots = make(chan struct{}, 2)

// CheckTemplates compiles the same effective JSON used for a new instance. The kernel's
// test mode reads Node-local assets and exits before calling Server.Start.
func CheckTemplates(ctx context.Context, templates []string) error {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	select {
	case templateCheckSlots <- struct{}{}:
		defer func() { <-templateCheckSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	binary, err := util.GetBinaryFile(constant.Xray)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTemplateCheckUnavailable, err)
	}
	if len(templates) == 0 || len(templates) > 17 {
		return fmt.Errorf("expected 1–17 Xray templates")
	}
	for i, template := range templates {
		if len(template) == 0 || len(template) > 10240 {
			return fmt.Errorf("Xray template #%d must be 1–10240 bytes", i+1)
		}
		data, err := renderXrayConfig(dto.XrayConfigDto{Template: template, ApiPort: 40001, Port: 10001, Protocol: "socks", Settings: "{}", Tag: "panel-template-check"})
		if err != nil {
			return fmt.Errorf("Xray template #%d: %w", i+1, err)
		}
		cmd := exec.CommandContext(ctx, binary, "run", "-test", "-format", "json", "-config", "stdin:")
		cmd.Stdin = bytes.NewReader(data)
		cmd.Env = []string{"LANG=C"}
		cmd.WaitDelay = time.Second
		output := new(checkOutput)
		cmd.Stdout, cmd.Stderr = output, output
		if err := startRestrictedCheck(cmd); err != nil {
			return fmt.Errorf("%w: %v", ErrTemplateCheckUnavailable, err)
		}
		if err := cmd.Wait(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Diagnostics may contain certificate bytes or template secrets.
			detail := "kernel rejected the configuration"
			for _, hint := range []string{"failed to parse certificate", "failed to build inbound", "failed to build outbound", "failed to build routing", "cannot unmarshal", "permission denied", "no such file"} {
				if strings.Contains(strings.ToLower(output.String()), hint) {
					detail = hint
					break
				}
			}
			return fmt.Errorf("Xray template #%d: %s (%v)", i+1, detail, err)
		}
		if !strings.Contains(output.String(), "Configuration OK.") {
			return errors.New("Xray configuration test did not confirm success")
		}
	}
	return nil
}

type checkOutput struct{ buf bytes.Buffer }

func (b *checkOutput) String() string { return b.buf.String() }

func (b *checkOutput) Write(p []byte) (int, error) {
	const limit = 4096
	n := len(p)
	if keep := limit - b.buf.Len(); len(p) > keep {
		p = p[:keep]
	}
	_, err := b.buf.Write(p)
	return n, err
}
