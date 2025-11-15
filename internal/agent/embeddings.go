package agent

import (
	"context"

	"log/slog"

	sdk "github.com/openai/openai-go/v3"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
	"myawesomelist.shikanime.studio/internal/agent/openai"
	"myawesomelist.shikanime.studio/internal/config"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Embeddings generates vector embeddings for projects using an OpenAI client.
type Embeddings struct {
	c     *sdk.Client
	model string
	l     *rate.Limiter
}

type EmbeddingsOptions struct{ limiter *rate.Limiter }
type EmbeddingsOption func(*EmbeddingsOptions)

func WithLimiter(l *rate.Limiter) EmbeddingsOption {
	return func(o *EmbeddingsOptions) { o.limiter = l }
}

// NewEmbeddingsForConfig constructs Embeddings by using the provided OpenAI client.
func NewEmbeddingsForConfig(cfg *config.Config, opts ...EmbeddingsOption) *Embeddings {
	return NewEmbeddingsWithOpenAI(cfg, openai.NewClientForConfig(cfg), opts...)
}

// NewEmbeddingsWithOpenAI constructs Embeddings by using the provided OpenAI client.
func NewEmbeddingsWithOpenAI(
	cfg *config.Config,
	c *sdk.Client,
	opts ...EmbeddingsOption,
) *Embeddings {
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
	tracer := otel.Tracer("myawesomelist/agent")
	ctx, span := tracer.Start(ctx, "Embeddings.EmbedProjects")
	defer span.End()

	out := make([][]float32, len(inputs))
	g, gctx := errgroup.WithContext(ctx)
	for i := range inputs {
		i := i
		g.Go(func() error {
			igctx, cspan := tracer.Start(gctx, "Embeddings.EmbedProject")
			cspan.SetAttributes(
				attribute.Int("index", i),
				attribute.String("model", e.model),
				attribute.Int("name_len", len(inputs[i].Name)),
				attribute.Int("desc_len", len(inputs[i].Description)),
			)
			if e.l != nil {
				if err := e.l.Wait(igctx); err != nil {
					cspan.RecordError(err)
					cspan.SetStatus(codes.Error, err.Error())
					cspan.End()
					return err
				}
			}
			slog.DebugContext(
				igctx,
				"embedding request",
				"index",
				i,
				"model",
				e.model,
				"name_len",
				len(inputs[i].Name),
				"desc_len",
				len(inputs[i].Description),
			)
			res, err := e.c.Embeddings.New(igctx, sdk.EmbeddingNewParams{
				Input: sdk.EmbeddingNewParamsInputUnion{
					OfString: sdk.String(inputs[i].Name + " " + inputs[i].Description),
				},
				Model: sdk.EmbeddingModel(e.model),
			})
			if err != nil {
				cspan.RecordError(err)
				cspan.SetStatus(codes.Error, err.Error())
				cspan.End()
				return err
			}
			v := make([]float32, len(res.Data[0].Embedding))
			for j := range v {
				v[j] = float32(res.Data[0].Embedding[j])
			}
			out[i] = v
			cspan.SetAttributes(attribute.Int("dim", len(v)))
			slog.DebugContext(igctx, "embedding response", "index", i, "dim", len(v))
			cspan.End()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}
