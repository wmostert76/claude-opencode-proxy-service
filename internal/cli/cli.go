package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wmostert76/claude-go/internal/config"
	"github.com/wmostert76/claude-go/internal/proxy"
)

var version = "2.0.0"

func SetVersion(v string) { version = v }

func Run(args []string) error {
	if len(args) == 0 {
		return runWrapper()
	}

	switch args[0] {
	case "--api":
		return setAPIKey(args)
	case "--model", "-m":
		return handleModel(args)
	case "--prompt":
		return setSystemPrompt(args)
	case "--prompt-clear":
		return clearSystemPrompt(args)
	case "--completion":
		return generateCompletion(args)
	case "--complete-models":
		return listModelIDs()
	case "--version":
		fmt.Println(version)
		return nil
	case "--help":
		printHelp()
		return nil
	case "setup":
		return runSetup()
	case "doctor":
		return runDoctor()
	case "status":
		return runStatus()
	case "logs":
		return runLogs(args)
	case "traces":
		return runTraces(args)
	case "trace":
		return runTrace(args)
	case "update":
		return runUpdate()
	case "install":
		return runInstall()
	case "uninstall":
		return runUninstall()
	case "models":
		return handleModels(args)
	default:
		return runWrapperArgs(args)
	}
}

func runWrapper() error {
	runWrapperArgs(nil)
	return nil
}

func runWrapperArgs(extra []string) error {
	cfg, _ := config.Load()
	run := func() error {
		proxy.ProxyInfo(version, cfg.Model)
		proxy.FetchModelsAtStartup(cfg.APIKey)
		return proxy.NewServer(cfg.APIKey, cfg.Model, proxy.Port()).Start()
	}

	_ = run // placeholder
	fmt.Println("Proxy mode not yet wired to Claude Code exec - building...")
	return nil
}

func setAPIKey(args []string) error {
	if len(args) < 2 || args[1] == "" {
		return fmt.Errorf("Usage: claude-go --api <opencode-go-api-key>")
	}
	cfg, _ := config.Load()
	cfg.APIKey = args[1]
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("Failed to save config: %w", err)
	}
	fmt.Printf("API key stored in ~/.config/claude-go/config.json\n")
	return nil
}

func handleModel(args []string) error {
	if len(args) < 2 || args[1] == "" {
		printModels()
		return nil
	}
	cfg, _ := config.Load()
	cfg.Model = proxy.NormalizeModel(args[1], cfg.Model)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("Failed to save config: %w", err)
	}
	fmt.Printf("Default model set to: %s\n", cfg.Model)
	return nil
}

func setSystemPrompt(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: claude-go --prompt <text>")
	}
	cfg, _ := config.Load()
	cfg.SystemPrompt = args[1]
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Println("System prompt stored.")
	return nil
}

func clearSystemPrompt(args []string) error {
	cfg, _ := config.Load()
	cfg.SystemPrompt = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Println("System prompt removed.")
	return nil
}

func printModels() {
	fmt.Println("OpenCode Go models (14):")
	fmt.Println()
	models := []struct {
		name string
		tier string
	}{
		{"deepseek-v4-pro", "Best"},
		{"glm-5.1", "Strong"},
		{"kimi-k2.6", "Strong"},
		{"minimax-m2.7", "Strong"},
		{"qwen3.6-plus", "Strong"},
		{"deepseek-v4-flash", "Fast/Cheap"},
		{"glm-5", "Good"},
		{"kimi-k2.5", "Good"},
		{"minimax-m2.5", "Good"},
		{"qwen3.5-plus", "Good"},
		{"mimo-v2.5-pro", "Specialized"},
		{"mimo-v2.5", "Specialized"},
		{"mimo-v2-pro", "Specialized"},
		{"mimo-v2-omni", "Specialized"},
	}
	for _, m := range models {
		fmt.Printf("  %-20s %s\n", m.name, m.tier)
	}
	fmt.Println("\nUse: claude-go --model <model>")
}

func runSetup() error {
	fmt.Printf("Claude Go setup v%s\n\n", version)
	fmt.Print("OpenCode Go API key: ")
	var key string
	fmt.Scanln(&key)
	if key == "" {
		return fmt.Errorf("API key is required")
	}
	cfg, _ := config.Load()
	cfg.APIKey = key
	config.Save(cfg)
	fmt.Println()
	printModels()
	fmt.Print("\nDefault model [deepseek-v4-pro]: ")
	var model string
	fmt.Scanln(&model)
	if model == "" {
		model = "deepseek-v4-pro"
	}
	cfg.Model = proxy.NormalizeModel(model, cfg.Model)
	config.Save(cfg)
	fmt.Println()
	return runDoctor()
}

func runDoctor() error {
	fmt.Printf("Claude Go doctor v%s\n\n", version)
	ok := func(msg string) { fmt.Printf("  OK   %s\n", msg)  }
	fail := func(msg string) { fmt.Printf("  FAIL %s\n", msg) }

	errors := 0

	for _, cmd := range []string{"node", "npm", "python3", "curl"} {
		if _, err := exec.LookPath(cmd); err == nil {
			ok(fmt.Sprintf("%s found", cmd))
		} else {
			fail(fmt.Sprintf("%s missing", cmd))
			errors++
		}
	}

	claudeBin := claudeCodePath()
	if _, err := os.Stat(claudeBin); err == nil {
		ok("Claude Code found")
	} else {
		fail(fmt.Sprintf("Claude Code not found: %s", claudeBin))
		errors++
	}

	cfg, _ := config.Load()
	if cfg.APIKey != "" {
		ok("API key configured")
	} else {
		fail("API key missing. Run: claude-go --api <key>")
		errors++
	}

	if cfg.Model != "" {
		ok(fmt.Sprintf("Default model: %s", cfg.Model))
	}

	if errors == 0 {
		fmt.Println("\nReady.")
	} else {
		fmt.Println("\nAction needed.")
	}
	return nil
}

func runStatus() error {
	cfg, _ := config.Load()
	home, _ := os.UserHomeDir()
	fmt.Println("Claude Go Status")
	fmt.Println("─────────────────────────────")
	fmt.Printf("  Release    v%s\n", version)
	fmt.Printf("  Proxy      http://127.0.0.1:%d\n", proxy.Port())
	fmt.Printf("  Provider   OpenCode Go\n")
	fmt.Printf("  Model      %s\n", cfg.Model)
	fmt.Printf("  Config     %s\n", filepath.Join(home, ".config", "claude-go", "config.json"))
	fmt.Printf("  Log        %s\n", filepath.Join(home, ".cache", "claude-go", "proxy.log"))
	fmt.Printf("  Claude     %s\n", claudeCodePath())
	return nil
}

func runLogs(args []string) error {
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".cache", "claude-go", "proxy.log")

	if len(args) > 1 && (args[1] == "--follow" || args[1] == "-f") {
		cmd := exec.Command("tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	cmd := exec.Command("tail", "-n", "80", logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTraces(args []string) error {
	home, _ := os.UserHomeDir()
	tracePath := filepath.Join(home, ".cache", "claude-go", "traces.jsonl")

	f, err := os.Open(tracePath)
	if err != nil {
		return fmt.Errorf("Trace log not found: %s", tracePath)
	}
	defer f.Close()

	var rows []map[string]any
	dec := json.NewDecoder(f)
	for dec.More() {
		var row map[string]any
		if err := dec.Decode(&row); err == nil {
			rows = append(rows, row)
		}
	}

	mode := "recent"
	limit := 20
	if len(args) > 1 {
		switch args[1] {
		case "--errors", "--slow", "--cost":
			mode = strings.TrimPrefix(args[1], "--")
			if len(args) > 2 {
				fmt.Sscanf(args[2], "%d", &limit)
			}
		default:
			fmt.Sscanf(args[1], "%d", &limit)
		}
	}

	// Filter/sort
	switch mode {
	case "errors":
		filtered := []map[string]any{}
		for _, r := range rows {
			if s, _ := r["status"].(string); s != "ok" {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
		if len(rows) > limit {
			rows = rows[len(rows)-limit:]
		}
	case "slow":
		// simple sort would need sort.Slice - skip for now
		if len(rows) > limit {
			rows = rows[:limit]
		}
	case "cost":
		if len(rows) > limit {
			rows = rows[:limit]
		}
	default:
		if len(rows) > limit {
			rows = rows[len(rows)-limit:]
		}
	}

	headers := []string{"Trace", "Status", "Model", "Final", "ms", "Retries", "Tokens", "Cost"}
	widths := []int{8, 6, 20, 20, 6, 7, 8, 6}
	fmt.Println(strings.Join(padSlice(headers, widths), "  "))
	fmt.Println(strings.Repeat("-", 80))
	for _, r := range rows {
		id := ""
		if v, ok := r["id"].(string); ok && len(v) >= 8 {
			id = v[:8]
		}
		status := ""
		if v, ok := r["status"].(string); ok {
			status = v
		}
		model := ""
		if v, ok := r["model"].(string); ok {
			model = truncate(v, 20)
		}
		final := ""
		if v, ok := r["finalModel"].(string); ok {
			final = truncate(v, 20)
		}
		lat := ""
		if v, ok := r["latencyMs"].(float64); ok {
			lat = fmt.Sprintf("%.0f", v)
		}
		retries := ""
		if v, ok := r["retries"].(float64); ok {
			retries = fmt.Sprintf("%.0f", v)
		}
		tokens := ""
		if u, ok := r["usage"].(map[string]any); ok {
			if v, ok := u["totalTokens"].(float64); ok {
				tokens = fmt.Sprintf("%.0f", v)
			} else if v, ok := u["inputTokens"].(float64); ok {
				tokens = fmt.Sprintf("%.0f", v)
			}
		}
		cost := ""
		if u, ok := r["usage"].(map[string]any); ok {
			if v, ok := u["cost"].(float64); ok {
				cost = fmt.Sprintf("%.4f", v)
			}
		}
		row := []string{id, status, model, final, lat, retries, tokens, cost}
		fmt.Println(strings.Join(padSlice(row, widths), "  "))
	}
	return nil
}

func runTrace(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: claude-go trace <trace-id>")
	}
	wanted := args[1]
	home, _ := os.UserHomeDir()
	tracePath := filepath.Join(home, ".cache", "claude-go", "traces.jsonl")

	f, err := os.Open(tracePath)
	if err != nil {
		return fmt.Errorf("Trace log not found: %s", tracePath)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	for dec.More() {
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			continue
		}
		if id, ok := row["id"].(string); ok && strings.HasPrefix(id, wanted) {
			b, _ := json.MarshalIndent(row, "", "  ")
			fmt.Println(string(b))
			return nil
		}
	}
	return fmt.Errorf("Trace not found: %s", wanted)
}

func runUpdate() error {
	repoURL := "https://github.com/wmostert76/claude-go/releases/latest/download"
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	}

	url := fmt.Sprintf("%s/claude-go-%s-%s", repoURL, osName, arch)
	fmt.Printf("Downloading %s...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("no release found — first release is built by CI on push to main")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Find current binary path
	binPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Download to temp file in same dir (avoids cross-device + text-file-busy)
	dir := filepath.Dir(binPath)
	tmp, err := os.CreateTemp(dir, ".claude-go-update-*")
	if err != nil {
		return fmt.Errorf("temp file failed: %v", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("download failed: %v", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	// Verify it runs
	cmd := exec.Command(tmpName, "--version")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("downloaded binary verification failed: %v", err)
	}
	newVer := strings.TrimSpace(string(out))

	// Atomically replace current binary (same device -> rename works)
	if err := os.Rename(tmpName, binPath); err != nil {
		return fmt.Errorf("replace failed: %v", err)
	}

	fmt.Printf("\nUpdated to %s\n", newVer)
	return nil
}

func runInstall() error {
	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "share", "claude-go")

	pkgJSON := filepath.Join(installDir, "package.json")
	if _, err := os.Stat(pkgJSON); os.IsNotExist(err) {
		os.MkdirAll(installDir, 0o755)
		pkg := `{"name":"claude-go-runtime","private":true,"dependencies":{"@anthropic-ai/claude-code":"*"}}`
		os.WriteFile(pkgJSON, []byte(pkg), 0o644)
		fmt.Println("Downloading Claude Code...")
		cmd := exec.Command("npm", "install", "--no-audit", "--no-fund")
		cmd.Dir = installDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		fmt.Println("Setting up shell completions...")
		installCompletions()
		return nil
	}

	fmt.Println("Updating Claude Code...")
	cmd := exec.Command("npm", "install", "--no-audit", "--no-fund")
	cmd.Dir = installDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	fmt.Println("Setting up shell completions...")
	installCompletions()
	return nil
}

func runUninstall() error {
	home, _ := os.UserHomeDir()
	wrapper := filepath.Join(home, ".local", "bin", "claude-go")
	if err := os.Remove(wrapper); err != nil {
		return fmt.Errorf("Claude Go not found: %s", wrapper)
	}
	fmt.Println("Claude Go uninstalled.")
	return nil
}

func handleModels(args []string) error {
	if len(args) > 1 && args[1] == "--test" {
		return testModels()
	}
	printModels()
	return nil
}

func testModels() error {
	cfg, _ := config.Load()
	if cfg.APIKey == "" {
		return fmt.Errorf("API key not set. Run: claude-go --api <key>")
	}

	models := []string{
		"deepseek-v4-pro", "glm-5.1", "kimi-k2.6", "minimax-m2.7", "qwen3.6-plus",
		"deepseek-v4-flash", "glm-5", "kimi-k2.5", "minimax-m2.5", "qwen3.5-plus",
		"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni",
	}

	fmt.Printf("Testing %d OpenCode Go models...\n\n", len(models))
	passed, failed := 0, 0

	type result struct {
		model string
		ok    bool
		ms    int64
		err   string
	}
	results := make([]result, len(models))
	done := make(chan int, len(models))

	for i, model := range models {
		go func(idx int, m string) {
			start := time.Now()
			ok, err := pingModel(cfg.APIKey, m)
			r := result{model: m, ok: ok, ms: time.Since(start).Milliseconds()}
			if !ok {
				r.err = err
			}
			results[idx] = r
			done <- idx
		}(i, model)
	}

	for range models {
		<-done
	}

	for _, r := range results {
		if r.ok {
			passed++
			fmt.Printf("  OK   opencode-go/%-25s %7dms\n", r.model, r.ms)
		} else {
			failed++
			errMsg := r.err
			if len(errMsg) > 40 {
				errMsg = errMsg[:40]
			}
			fmt.Printf("  FAIL opencode-go/%-25s %7dms  %s\n", r.model, r.ms, errMsg)
		}
	}

	fmt.Printf("\nResult: %d passed, %d failed, %d total\n", passed, failed, passed+failed)
	return nil
}

func pingModel(apiKey, model string) (bool, string) {
	body := fmt.Sprintf(`{"model":"%s","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`, model)
	req, _ := http.NewRequest("POST", proxy.Target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return false, string(b)
	}

	var data struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&data)
	if data.Error.Message != "" {
		return false, data.Error.Message
	}
	if len(data.Choices) > 0 {
		c := data.Choices[0].Message
		if c.Content != "" || c.ReasoningContent != "" || c.Reasoning != "" {
			return true, ""
		}
	}
	return false, "empty response"
}

func listModelIDs() error {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	f, err := os.Open(cfgPath)
	if err != nil {
		// Fall back to known models
		models := []string{
			"deepseek-v4-pro", "deepseek-v4-flash",
			"glm-5.1", "glm-5",
			"kimi-k2.6", "kimi-k2.5",
			"minimax-m2.7", "minimax-m2.5",
			"qwen3.6-plus", "qwen3.5-plus",
			"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni",
		}
		for _, m := range models {
			fmt.Println(m)
		}
		return nil
	}
	defer f.Close()

	var cfg map[string]any
	json.NewDecoder(f).Decode(&cfg)
	provider, _ := cfg["provider"].(map[string]any)
	oc, _ := provider["opencode-go"].(map[string]any)
	models, _ := oc["models"].(map[string]any)
	for id := range models {
		fmt.Println(id)
	}
	return nil
}

const fishCompletionScript = `function __claude_opencode_go_models
    command claude-go --complete-models 2>/dev/null
end

complete -c claude-go -l model -x -a '(__claude_opencode_go_models)' -d 'Set default model'
complete -c claude-go -s m -x -a '(__claude_opencode_go_models)' -d 'Set default model'
complete -c claude-go -f -a 'setup doctor status logs traces trace update models' -d 'Claude Go command'
`

const bashCompletionScript = `_claude_opencode_completion() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  if [[ "$prev" == "--model" || "$prev" == "-m" ]]; then
    mapfile -t COMPREPLY < <(compgen -W "$(command claude-go --complete-models 2>/dev/null)" -- "$cur")
    return 0
  fi
  if [[ "$cur" == --* ]]; then
    COMPREPLY=( $(compgen -W "--api --model --help --version setup doctor status logs traces trace update models" -- "$cur") )
  fi
}
complete -F _claude_opencode_completion claude-go
`

func installCompletions() {
	home, _ := os.UserHomeDir()

	// Fish
	fishDir := filepath.Join(home, ".config", "fish", "completions")
	os.MkdirAll(fishDir, 0o755)
	os.WriteFile(filepath.Join(fishDir, "claude-go.fish"), []byte(fishCompletionScript), 0o644)

	// Bash
	bashDir := filepath.Join(home, ".local", "share", "bash-completion", "completions")
	os.MkdirAll(bashDir, 0o755)
	os.WriteFile(filepath.Join(bashDir, "claude-go"), []byte(bashCompletionScript), 0o644)
}

func generateCompletion(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Usage: claude-go --completion <bash|fish>")
	}
	switch args[1] {
	case "fish":
		fmt.Print(fishCompletionScript)
	case "bash":
		fmt.Print(bashCompletionScript)
	}
	return nil
}

func printHelp() {
	fmt.Println(`Claude Go - Use Claude Code with OpenCode Go models

Usage:
  claude-go                           Start Claude Code through the proxy
  claude-go --api <key>               Store OpenCode Go API key
  claude-go --model                   List available models
  claude-go --model <name>            Set default model
  claude-go --prompt <text>           Set persistent system prompt
  claude-go --prompt-clear            Remove system prompt
  claude-go setup                     Interactive setup wizard
  claude-go doctor                    Check dependencies and config
  claude-go status                    Show proxy and config status
  claude-go logs [--follow]           View proxy logs
  claude-go traces [--errors|--slow|--cost] [n]  Show traces
  claude-go trace <id>                View single trace as JSON
  claude-go update                    Update Claude Go
  claude-go install                  Install Claude Code locally
  claude-go uninstall                Remove Claude Go
  claude-go models [--test]          List or test models
  claude-go --completion <bash|fish>  Generate shell completion
  claude-go --version                 Print version
  claude-go --help                    Show this help`)
}

func claudeCodePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "claude-go", "node_modules", "@anthropic-ai", "claude-code", "bin", "claude.exe")
}

func padSlice(items []string, widths []int) []string {
	for i := range items {
		items[i] = padRight(items[i], widths[i])
	}
	return items
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
