package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wmostert76/claude-go/internal/cli"
	"github.com/wmostert76/claude-go/internal/config"
	"github.com/wmostert76/claude-go/internal/proxy"
	"github.com/wmostert76/claude-go/internal/trace"
)

var version = "0.1.0"

func main() {
	cli.SetVersion(version)

	// No args = start Claude Code
	if len(os.Args) < 2 {
		runClaudeCode(nil)
		return
	}

	var args = os.Args[1:]

	// Handle subcommands
	switch {
	case len(args) > 0 && args[0] == "serve":
		// Start proxy only (for testing direct API access)
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		proxy.ProxyInfo(version)
		proxy.FetchModelsAtStartup(cfg.APIKey)
		srv := proxy.NewServer(cfg.APIKey, cfg.Model, proxy.Port())
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Proxy error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle --api and other --flags before subcommand dispatch
	if len(args) > 0 && args[0] == "--api" {
		if err := cli.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	switch {
	case len(args) > 0 && (args[0] == "--model" || args[0] == "-m"),
		len(args) > 0 && args[0] == "--prompt",
		len(args) > 0 && args[0] == "--prompt-clear",
		len(args) > 0 && args[0] == "--completion",
		len(args) > 0 && args[0] == "--complete-models",
		len(args) > 0 && args[0] == "--version",
		len(args) > 0 && args[0] == "--help",
		len(args) > 0 && (args[0] == "setup" || args[0] == "doctor" || args[0] == "status" ||
			args[0] == "logs" || args[0] == "traces" || args[0] == "trace" ||
			args[0] == "update" || args[0] == "install" || args[0] == "uninstall" ||
			args[0] == "models"):
		if err := cli.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Default: start proxy + Claude Code
	runClaudeCode(args)
}

func runClaudeCode(extraArgs []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\nRun: claude-go --api <key>\n", err)
		os.Exit(1)
	}

	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "API key not set. Run: claude-go --api <key>")
		os.Exit(1)
	}

	// Sync Claude Code version (fast check)
	syncClaudeCode()

	proxyURL := fmt.Sprintf("http://127.0.0.1:%d", proxy.Port())

	// Start proxy in background
	srv := proxy.NewServer(cfg.APIKey, cfg.Model, proxy.Port())
	stopCh := make(chan error, 1)
	go func() {
		proxy.ProxyInfo(version)
		proxy.FetchModelsAtStartup(cfg.APIKey)
		stopCh <- srv.Start()
	}()

	// Wait for proxy health
	healthURL := fmt.Sprintf("%s/health", proxyURL)
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				break
			}
		}
	}

	// Check proxy didn't crash
	select {
	case err := <-stopCh:
		fmt.Fprintf(os.Stderr, "Proxy exited: %v\n", err)
		os.Exit(1)
	default:
	}

	// Build Claude Code command
	claudeBin := claudeCodePath()
	if _, err := os.Stat(claudeBin); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Claude Code not installed. Run: claude-go install")
		os.Exit(1)
	}

	sessionPrompt := fmt.Sprintf(
		"You are Claude Go v%s running through OpenCode Go. "+
			"You are a direct, expert software engineering assistant with full access to all standard Claude Code tools. "+
			"Do not claim to be using Anthropic-hosted models directly. "+
			"IMPORTANT: Never use the WebSearch tool — it does not work through this proxy. "+
			"Instead, use WebFetch with a search URL (e.g. Google, DuckDuckGo) to look up information online. "+
			"For example: WebFetch(\"https://www.google.com/search?q=your+query\") or WebFetch(\"https://html.duckduckgo.com/html/?q=your+query\").",
		version)

	claudeArgs := []string{
		"--dangerously-skip-permissions",
		"--append-system-prompt", sessionPrompt,
	}

	if cfg.SystemPrompt != "" {
		claudeArgs = append(claudeArgs, "--append-system-prompt", cfg.SystemPrompt)
	}
	claudeArgs = append(claudeArgs, extraArgs...)

	cmd := exec.Command(claudeBin, claudeArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_BASE_URL="+proxyURL,
		"ANTHROPIC_API_KEY=claude-go-local-proxy",
	)

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status := exitErr.Sys().(syscall.WaitStatus)
			os.Exit(status.ExitStatus())
		}
		os.Exit(1)
	}

	trace.Close()
}

func claudeCodePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "claude-go", "node_modules",
		"@anthropic-ai", "claude-code", "bin", "claude.exe")
}

func syncClaudeCode() {
	home, _ := os.UserHomeDir()
	globalPkg := filepath.Join(home, ".local", "share", "npm-global", "lib",
		"node_modules", "@anthropic-ai", "claude-code", "package.json")
	localPkg := filepath.Join(home, ".local", "share", "claude-go",
		"node_modules", "@anthropic-ai", "claude-code", "package.json")

	globalVer := readPkgVersion(globalPkg)
	localVer := readPkgVersion(localPkg)

	if globalVer != "" && globalVer != localVer && localVer != "" {
		srcDir := filepath.Dir(globalPkg)
		dstDir := filepath.Dir(localPkg)
		os.RemoveAll(dstDir)
		copyDir(srcDir, dstDir)
	}
}

func readPkgVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"version":`) {
			v := strings.TrimPrefix(line, `"version":`)
			v = strings.Trim(v, ` ",\n\r\t`)
			return v
		}
	}
	return ""
}

func copyDir(src, dst string) {
	os.MkdirAll(dst, 0o755)
	entries, _ := os.ReadDir(src)
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyDir(srcPath, dstPath)
		} else {
			data, _ := os.ReadFile(srcPath)
			os.WriteFile(dstPath, data, 0o644)
		}
	}
}
