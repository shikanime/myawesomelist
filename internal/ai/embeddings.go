package ai

import (
	"context"

	sdk "github.com/openai/openai-go/v3"
	"myawesomelist.shikanime.studio/internal/ai/openai"
	"myawesomelist.shikanime.studio/internal/config"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// Embeddings generates vector embeddings for projects using an OpenAI client.
type Embeddings struct {
	c     *sdk.Client
	model string
}

// NewEmbeddingsForConfig constructs Embeddings with the provided OpenAI client and model from cfg.
func NewEmbeddingsForConfig(cfg *config.Config, c *sdk.Client) *Embeddings {
	return &Embeddings{c: c, model: cfg.GetEmbeddingModel()}
}

// NewEmbeddingsWithOpenAI constructs Embeddings by creating an OpenAI client from cfg.
func NewEmbeddingsWithOpenAI(cfg *config.Config) *Embeddings {
	return NewEmbeddingsForConfig(cfg, openai.NewClientForConfig(cfg))
}

// EmbedProjects returns embeddings for a slice of projects.
func (e *Embeddings) EmbedProjects(
	ctx context.Context,
	inputs []*myawesomelistv1.Project,
) ([][]float32, error) {
	out := make([][]float32, len(inputs))
	for i := range inputs {
		res, err := e.c.Embeddings.New(ctx, sdk.EmbeddingNewParams{
			Input: sdk.EmbeddingNewParamsInputUnion{
				OfString: sdk.String(inputs[i].Name + " " + inputs[i].Description),
			},
			Model: sdk.EmbeddingModel(e.model),
		})
		if err != nil {
			return nil, err
		}
		v := make([]float32, len(res.Data[0].Embedding))
		for j := range v {
			v[j] = float32(res.Data[0].Embedding[j])
		}
		out[i] = v
	}
	return out, nil
}
