package awesome

import (
	"context"

	"github.com/openai/openai-go/v3"
)

type Embeddings struct {
    c     *openai.Client
    model string
}

func NewOpenAIEmbeddings(c *openai.Client, model string) *Embeddings {
	return &Embeddings{c: c, model: model}
}

func (e *Embeddings) EmbedProject(
    ctx context.Context,
    name string,
    description string,
) ([]float32, error) {
	res, err := e.c.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(name + " " + description),
		},
		Model: openai.EmbeddingModel(e.model),
	})
	if err != nil {
		return nil, err
	}
	v := make([]float32, len(res.Data[0].Embedding))
	for i := range v {
		v[i] = float32(res.Data[0].Embedding[i])
	}
	return v, nil
}

type ProjectInput struct {
    Name        string
    Description string
}

func (e *Embeddings) EmbedProjects(
    ctx context.Context,
    inputs []ProjectInput,
) ([][]float32, error) {
    out := make([][]float32, len(inputs))
    for i := range inputs {
        v, err := e.EmbedProject(ctx, inputs[i].Name, inputs[i].Description)
        if err != nil {
            return nil, err
        }
        out[i] = v
    }
    return out, nil
}
