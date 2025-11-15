package agent

import (
    "context"

    sdk "github.com/openai/openai-go/v3"
    "golang.org/x/time/rate"
    "myawesomelist.shikanime.studio/internal/agent/openai"
    "myawesomelist.shikanime.studio/internal/config"
    myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
    "log/slog"
)

// Embeddings generates vector embeddings for projects using an OpenAI client.
type Embeddings struct {
    c     *sdk.Client
    model string
    l     *rate.Limiter
}

type EmbeddingsOptions struct{ limiter *rate.Limiter }
type EmbeddingsOption func(*EmbeddingsOptions)

func WithLimiter(l *rate.Limiter) EmbeddingsOption { return func(o *EmbeddingsOptions) { o.limiter = l } }

// NewEmbeddingsForConfig constructs Embeddings by using the provided OpenAI client.
func NewEmbeddingsForConfig(cfg *config.Config, opts ...EmbeddingsOption) *Embeddings {
    return NewEmbeddingsWithOpenAI(cfg, openai.NewClientForConfig(cfg), opts...)
}

// NewEmbeddingsWithOpenAI constructs Embeddings by using the provided OpenAI client.
func NewEmbeddingsWithOpenAI(cfg *config.Config, c *sdk.Client, opts ...EmbeddingsOption) *Embeddings {
    var o EmbeddingsOptions
    for _, opt := range opts {
        opt(&o)
    }
    e := &Embeddings{c: c, model: cfg.GetEmbeddingModel(), l: o.limiter}
    slog.Debug("embeddings configured", "model", e.model, "limiter", e.l != nil)
    return e
}

// EmbedProjects returns embeddings for a slice of projects.
func (e *Embeddings) EmbedProjects(
    ctx context.Context,
    inputs []*myawesomelistv1.Project,
) ([][]float32, error) {
    out := make([][]float32, len(inputs))
    for i := range inputs {
        if e.l != nil {
            if err := e.l.Wait(ctx); err != nil {
                return nil, err
            }
        }
        slog.DebugContext(ctx, "embedding request", "index", i, "model", e.model, "name_len", len(inputs[i].Name), "desc_len", len(inputs[i].Description))
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
        slog.DebugContext(ctx, "embedding response", "index", i, "dim", len(v))
    }
    return out, nil
}