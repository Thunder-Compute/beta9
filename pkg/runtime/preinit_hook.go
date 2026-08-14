package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const preInitShellName = "beta9-preinit"
const preInitDefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

type PreInitHook struct {
	Name    string
	Script  string
	Timeout time.Duration
}

func InjectPreInitHooks(spec *specs.Spec, hooks ...PreInitHook) error {
	if spec == nil || spec.Process == nil {
		return fmt.Errorf("container spec is required for pre-init hooks")
	}
	if len(spec.Process.Args) == 0 {
		return fmt.Errorf("container process args are required for pre-init hooks")
	}

	script := preInitScript(hooks)
	if script == "" {
		return nil
	}

	if !envHasKey(spec.Process.Env, "PATH") {
		spec.Process.Env = append(spec.Process.Env, "PATH="+preInitDefaultPath)
	}

	originalArgs := append([]string(nil), spec.Process.Args...)
	spec.Process.Args = append([]string{
		"bash",
		"-o",
		"pipefail",
		"-c",
		script,
		preInitShellName,
	}, originalArgs...)
	return nil
}

func preInitScript(hooks []PreInitHook) string {
	var builder strings.Builder
	builder.WriteString("set -e\n")
	wroteHook := false
	for _, hook := range hooks {
		script := strings.TrimSpace(hook.Script)
		if script == "" {
			continue
		}
		if hook.Name != "" {
			builder.WriteString("# ")
			builder.WriteString(hook.Name)
			builder.WriteByte('\n')
		}
		if hook.Timeout > 0 {
			builder.WriteString(wrapPreInitScriptWithTimeout(script, hook.Timeout))
		} else {
			builder.WriteString(script)
		}
		builder.WriteByte('\n')
		wroteHook = true
	}
	if !wroteHook {
		return ""
	}
	builder.WriteString("exec \"$@\"")
	return builder.String()
}

func wrapPreInitScriptWithTimeout(script string, timeout time.Duration) string {
	seconds := int((timeout + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	// Integer seconds (no unit suffix) is accepted by GNU and BusyBox timeout.
	return fmt.Sprintf("timeout %d bash -o pipefail -c %s", seconds, shellSingleQuote(script))
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
