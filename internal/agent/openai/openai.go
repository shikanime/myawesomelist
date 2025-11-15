package openai

import (
    "log/slog"
    "time"

    sdk "github.com/openai/openai-go/v3"
    "github.com/openai/openai-go/v3/option"
    "golang.org/x/time/rate"
    "myawesomelist.shikanime.studio/internal/config"
)

func NewClientForConfig(cfg *config.Config) *sdk.Client {
    c := sdk.NewClient(
        option.WithAPIKey(cfg.GetOpenAIAPIKey()),
        option.WithMaxRetries(3),
        option.WithBaseURL(cfg.GetOpenAIBaseURL()),
    )
    return &c
}

func NewOpenAIScalewayLimiter(identityVerified bool) *rate.Limiter {
    if identityVerified {
        l := rate.NewLimiter(rate.Every(time.Minute), 120)
        slog.Info("Created Scaleway OpenAI rate limiter", "rate", "120 requests/min", "burst", 10)
        return l
    }
    l := rate.NewLimiter(rate.Every(time.Minute), 60)
    slog.Info("Created Scaleway OpenAI rate limiter", "rate", "60 requests/min", "burst", 5)
    return l
}
