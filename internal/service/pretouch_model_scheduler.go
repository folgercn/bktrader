package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultPretouchModelReloadInterval  = 30 * time.Second
	defaultPretouchModelRetrainInterval = 24 * time.Hour
)

type PretouchModelSchedulerConfig struct {
	HotReloadEnabled        bool
	ReloadInterval          time.Duration
	RetrainEnabled          bool
	RetrainInterval         time.Duration
	LeadRetrainEnabled      bool
	T3RetrainEnabled        bool
	LeadModelPath           string
	LeadEventsCSVPath       string
	LeadTimingLabelsCSVPath string
	LeadTimingLabelsMaxAge  time.Duration
	T3OverlayModelPath      string
	T3OverlayTradesCSVPath  string
}

type pretouchModelFileState struct {
	path    string
	size    int64
	modTime time.Time
	version string
}

func (p *Platform) StartPretouchModelScheduler(ctx context.Context, cfg PretouchModelSchedulerConfig) {
	engine := p.pretouchTimingEngine()
	if engine == nil {
		p.logger("service.pretouch_model_scheduler").Warn("pretouch model scheduler disabled because timing engine is not registered")
		return
	}
	cfg = normalizePretouchModelSchedulerConfig(cfg, engine)
	logger := p.logger("service.pretouch_model_scheduler")
	if !cfg.HotReloadEnabled && !cfg.RetrainEnabled {
		logger.Info("pretouch model scheduler disabled by configuration")
		return
	}
	logger.Info("pretouch model scheduler started",
		"hot_reload_enabled", cfg.HotReloadEnabled,
		"reload_interval", cfg.ReloadInterval.String(),
		"retrain_enabled", cfg.RetrainEnabled,
		"retrain_interval", cfg.RetrainInterval.String(),
		"lead_retrain_enabled", cfg.LeadRetrainEnabled,
		"t3_retrain_enabled", cfg.T3RetrainEnabled,
		"lead_model_path", cfg.LeadModelPath,
		"t3_overlay_model_path", cfg.T3OverlayModelPath,
	)
	go p.runPretouchModelScheduler(ctx, engine, cfg)
}

func (p *Platform) pretouchTimingEngine() *bkLiveEthPretouchTimingEngine {
	engine, ok := p.strategyEngines[normalizeStrategyEngineKey(bkLiveEthPretouchTimingEngineKey)]
	if !ok {
		return nil
	}
	pretouchEngine, _ := engine.(*bkLiveEthPretouchTimingEngine)
	return pretouchEngine
}

func normalizePretouchModelSchedulerConfig(cfg PretouchModelSchedulerConfig, engine *bkLiveEthPretouchTimingEngine) PretouchModelSchedulerConfig {
	if cfg.ReloadInterval <= 0 {
		cfg.ReloadInterval = defaultPretouchModelReloadInterval
	}
	if cfg.RetrainInterval <= 0 {
		cfg.RetrainInterval = defaultPretouchModelRetrainInterval
	}
	if strings.TrimSpace(cfg.LeadModelPath) == "" {
		cfg.LeadModelPath = firstNonEmpty(engine.modelPath, defaultPretouchModelPath)
	}
	if strings.TrimSpace(cfg.T3OverlayModelPath) == "" {
		cfg.T3OverlayModelPath = firstNonEmpty(engine.t3ModelPath, defaultPretouchT3OverlayModelPath)
	}
	if strings.TrimSpace(cfg.LeadEventsCSVPath) == "" {
		cfg.LeadEventsCSVPath = DefaultPretouchTrainerConfig().EventsCSVPath
	}
	if strings.TrimSpace(cfg.LeadTimingLabelsCSVPath) == "" {
		cfg.LeadTimingLabelsCSVPath = DefaultPretouchTrainerConfig().TimingLabelsCSVPath
	}
	if cfg.LeadTimingLabelsMaxAge < 0 {
		cfg.LeadTimingLabelsMaxAge = 0
	}
	if strings.TrimSpace(cfg.T3OverlayTradesCSVPath) == "" {
		cfg.T3OverlayTradesCSVPath = DefaultPretouchT3OverlayTrainerConfig().TradesCSVPath
	}
	return cfg
}

func (p *Platform) runPretouchModelScheduler(ctx context.Context, engine *bkLiveEthPretouchTimingEngine, cfg PretouchModelSchedulerConfig) {
	logger := p.logger("service.pretouch_model_scheduler")
	var leadState pretouchModelFileState
	var t3State pretouchModelFileState
	reloadAll := func(force bool) {
		if !cfg.HotReloadEnabled && !force {
			return
		}
		p.reloadPretouchModelIfChanged("lead", cfg.LeadModelPath, &leadState, force, engine.setLeadModel)
		p.reloadPretouchModelIfChanged("t3_overlay", cfg.T3OverlayModelPath, &t3State, force, engine.setT3OverlayModel)
	}

	reloadAll(true)

	reloadTicker := time.NewTicker(cfg.ReloadInterval)
	defer reloadTicker.Stop()
	var retrainTicker *time.Ticker
	var retrainC <-chan time.Time
	if cfg.RetrainEnabled && (cfg.LeadRetrainEnabled || cfg.T3RetrainEnabled) {
		retrainTicker = time.NewTicker(cfg.RetrainInterval)
		retrainC = retrainTicker.C
		defer retrainTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("pretouch model scheduler stopped")
			return
		case <-reloadTicker.C:
			reloadAll(false)
		case <-retrainC:
			p.retrainPretouchModels(cfg)
			reloadAll(true)
		}
	}
}

func (p *Platform) reloadPretouchModelIfChanged(kind, path string, state *pretouchModelFileState, force bool, apply func(*PretouchModelBundle)) bool {
	logger := p.logger("service.pretouch_model_scheduler", "model_kind", kind)
	path = strings.TrimSpace(path)
	if path == "" {
		logger.Warn("skip pretouch model reload because model path is empty")
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		logger.Warn("skip pretouch model reload because model file is unavailable", "path", path, "error", err)
		return false
	}
	if info.IsDir() || info.Size() <= 0 {
		logger.Warn("skip pretouch model reload because model file is not a non-empty regular file", "path", path, "size", info.Size())
		return false
	}
	if !force && state.path == path && state.size == info.Size() && state.modTime.Equal(info.ModTime()) {
		return false
	}
	bundle, err := LoadModelBundle(path)
	if err != nil {
		logger.Warn("skip pretouch model reload because model bundle validation failed", "path", path, "error", err)
		return false
	}
	apply(bundle)
	*state = pretouchModelFileState{
		path:    path,
		size:    info.Size(),
		modTime: info.ModTime(),
		version: bundle.Version,
	}
	logger.Info("pretouch model reloaded",
		"path", path,
		"version", bundle.Version,
		"trained_at", bundle.TrainedAt,
		"rf_accuracy", bundle.RFAccuracy,
	)
	return true
}

func (p *Platform) retrainPretouchModels(cfg PretouchModelSchedulerConfig) {
	logger := p.logger("service.pretouch_model_scheduler")
	if cfg.LeadRetrainEnabled {
		if err := p.retrainPretouchLeadModel(cfg); err != nil {
			logger.Warn("pretouch lead model retrain skipped or failed", "error", err)
		} else {
			logger.Info("pretouch lead model retrained", "model_path", cfg.LeadModelPath)
		}
	}
	if cfg.T3RetrainEnabled {
		if err := p.retrainPretouchT3OverlayModel(cfg); err != nil {
			logger.Warn("pretouch T3 overlay model retrain skipped or failed", "error", err)
		} else {
			logger.Info("pretouch T3 overlay model retrained", "model_path", cfg.T3OverlayModelPath)
		}
	}
}

func (p *Platform) retrainPretouchLeadModel(cfg PretouchModelSchedulerConfig) error {
	if err := ensureReadableNonEmptyFile(cfg.LeadEventsCSVPath); err != nil {
		return err
	}
	if err := ensureReadableNonEmptyFreshFile(cfg.LeadTimingLabelsCSVPath, cfg.LeadTimingLabelsMaxAge); err != nil {
		return err
	}
	trainCfg := DefaultPretouchTrainerConfig()
	trainCfg.EventsCSVPath = cfg.LeadEventsCSVPath
	trainCfg.TimingLabelsCSVPath = cfg.LeadTimingLabelsCSVPath
	trainCfg.ModelOutPath = cfg.LeadModelPath
	return TrainPretouchModel(trainCfg)
}

func (p *Platform) retrainPretouchT3OverlayModel(cfg PretouchModelSchedulerConfig) error {
	if err := ensureReadableNonEmptyFile(cfg.T3OverlayTradesCSVPath); err != nil {
		return err
	}
	trainCfg := DefaultPretouchT3OverlayTrainerConfig()
	trainCfg.TradesCSVPath = cfg.T3OverlayTradesCSVPath
	trainCfg.ModelOutPath = cfg.T3OverlayModelPath
	return TrainPretouchT3OverlayModel(trainCfg)
}

func ensureReadableNonEmptyFile(path string) error {
	_, err := statReadableNonEmptyFile(path)
	return err
}

func ensureReadableNonEmptyFreshFile(path string, maxAge time.Duration) error {
	info, err := statReadableNonEmptyFile(path)
	if err != nil {
		return err
	}
	if maxAge > 0 && time.Since(info.ModTime()) > maxAge {
		return fmt.Errorf("model retrain source %s is stale: mod_time=%s max_age=%s", path, info.ModTime().UTC().Format(time.RFC3339), maxAge)
	}
	return nil
}

func statReadableNonEmptyFile(path string) (os.FileInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("model retrain source path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("model retrain source %s unavailable: %w", path, err)
	}
	if info.IsDir() || info.Size() <= 0 {
		return nil, fmt.Errorf("model retrain source %s is not a non-empty file", path)
	}
	return info, nil
}
