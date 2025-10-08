package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "time"

    "claude-code-codex-companion/internal/config"
    "claude-code-codex-companion/internal/endpoint"
    "claude-code-codex-companion/internal/i18n"
    "claude-code-codex-companion/internal/logger"
    "claude-code-codex-companion/internal/tagging"
    "claude-code-codex-companion/internal/web"
)

func main() {
    cfgPath := flag.String("config", "config.yaml", "Config file path")
    flag.Parse()

    cfg, err := config.LoadConfig(*cfgPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
        os.Exit(1)
    }

    // Logger
    log, err := logger.NewLogger(logger.LogConfig{
        Level:           cfg.Logging.Level,
        LogRequestTypes: cfg.Logging.LogRequestTypes,
        LogRequestBody:  cfg.Logging.LogRequestBody,
        LogResponseBody: cfg.Logging.LogResponseBody,
        LogDirectory:    cfg.Logging.LogDirectory,
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
        os.Exit(1)
    }
    defer log.Close()

    // Endpoint manager
    em, err := endpoint.NewManager(cfg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to init endpoint manager: %v\n", err)
        os.Exit(1)
    }

    // Tagging
    tm := tagging.NewManager()
    if err := tm.Initialize(&cfg.Tagging); err != nil {
        fmt.Fprintf(os.Stderr, "failed to init tagging: %v\n", err)
        os.Exit(1)
    }

    // i18n
    i18nManager, _ := i18n.NewManager(i18n.DefaultConfig())

    admin := web.NewAdminServer(cfg, em, tm, log, *cfgPath, "probe", i18nManager)

    // Probe only enabled endpoints to keep it fast
    all := em.GetAllEndpoints()
    results := make([]*web.BatchTestResult, 0, len(all))
    for _, ep := range all {
        // Test all endpoints regardless of enabled flag
        r := admin.TestEndpoint(ep.Name)
        results = append(results, r)
        // Short delay between endpoints to avoid burst
        time.Sleep(200 * time.Millisecond)
    }

    out := map[string]interface{}{
        "timestamp": time.Now().Unix(),
        "results":   results,
    }
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    _ = enc.Encode(out)
}
