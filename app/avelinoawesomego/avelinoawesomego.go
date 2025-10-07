package avelinoawesomego

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-github/v75/github"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Project struct {
	Name        string
	Description string
	URL         string
	Category    string
}

// ReadContent creates a reader for the README.md file of the Avelino/awesome-go repository
func ReadContent(ctx context.Context, client *github.Client) ([]byte, error) {
	file, _, _, err := client.Repositories.GetContents(
		ctx,
		"avelino",
		"awesome-go",
		"README.md",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	content, err := base64.StdEncoding.DecodeString(*file.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content: %v", err)
	}
	return content, nil
}

// DecodeProjects parses projects from the Avelino/awesome-go repository
func DecodeProjects(in []byte) ([]Project, error) {
	var projects []Project

	// Create a goldmark parser
	p := goldmark.New()
	root := p.Parser().Parse(text.NewReader(in))

	// Find the ActorModel section and start parsing from there
	var currentCategory string
	var foundActorModel bool

	// Walk through the AST
	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Heading:
			if n.Level == 2 { // Main category headings are level 2
				headingText, err := DecodeTextFromNode(n, in)
				if err != nil {
					return ast.WalkStop, fmt.Errorf("failed to decode heading text: %v", err)
				}

				// Check if we've reached the ActorModel section
				if strings.Contains(strings.ToLower(headingText), "actor model") {
					foundActorModel = true
					currentCategory = strings.TrimSpace(headingText)
				} else if foundActorModel {
					// Update current category for subsequent sections
					currentCategory = strings.TrimSpace(headingText)
				}
			}

		case *ast.List:
			if foundActorModel && currentCategory != "" {
				// Parse list items as projects
				for child := n.FirstChild(); child != nil; child = child.NextSibling() {
					if listItem, ok := child.(*ast.ListItem); ok {
						project, err := DecodeProjectsFromListItem(listItem, in, currentCategory)
						if err != nil {
							return ast.WalkStop, fmt.Errorf("failed to decode project: %v", err)
						}
						if project.Name != "" {
							projects = append(projects, project)
						}
					}
				}
			}
		}

		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk AST: %v", err)
	}

	if !foundActorModel {
		return nil, fmt.Errorf("ActorModel section not found in the document")
	}

	return projects, nil
}

// DecodeTextFromNode extracts text content from an AST node
func DecodeTextFromNode(node ast.Node, source []byte) (string, error) {
	var text strings.Builder
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if textNode, ok := n.(*ast.Text); ok {
			text.Write(textNode.Segment.Value(source))
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk node: %v", err)
	}
	return text.String(), nil
}

// DecodeProjectsFromListItem extracts project information from a list item
func DecodeProjectsFromListItem(listItem *ast.ListItem, source []byte, category string) (Project, error) {
	project := Project{Category: category}
	err := ast.Walk(listItem, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Link:
			// Extract project name and URL
			project.URL = string(n.Destination)
			name, err := DecodeTextFromNode(n, source)
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to decode project name: %v", err)
			}
			project.Name = name

		case *ast.Text:
			// Extract description (text after the link)
			text, err := DecodeTextFromNode(n, source)
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to decode project description: %v", err)
			}
			if project.Name != "" && strings.Contains(text, " - ") {
				// Split on " - " to get the description
				parts := strings.SplitN(text, " - ", 2)
				if len(parts) > 1 {
					project.Description = strings.TrimSpace(parts[1])
				}
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return Project{}, fmt.Errorf("failed to walk list item: %v", err)
	}
	return project, nil
}

// GetProjects parses projects from the Avelino/awesome-go repository
func GetProjects(ctx context.Context, client *github.Client) ([]Project, error) {
	content, err := ReadContent(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %v", err)
	}
	return DecodeProjects(content)
}
