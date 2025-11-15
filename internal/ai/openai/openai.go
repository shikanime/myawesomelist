package openai

import (
	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"myawesomelist.shikanime.studio/internal/config"
)

func NewClientForConfig(cfg *config.Config) *sdk.Client {
	c := sdk.NewClient(option.WithAPIKey(cfg.GetOpenAIAPIKey()))
	return &c
}
