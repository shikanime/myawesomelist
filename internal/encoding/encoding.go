package encoding

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type Project struct {
	Name        string
	Description string
	URL         string
	Language    string
}

type Category struct {
	Name     string
	Projects []Project
}

// UnmarshallCategories parses projects from a repository's README and groups them by category
func UnmarshallCategories(in []byte, startSection string) ([]Category, error) {
	var categories []Category
	categoryMap := make(map[string]*Category)

	// Create a goldmark parser
	p := goldmark.New()
	root := p.Parser().Parse(text.NewReader(in))

	// Find the specified start section and begin parsing from there
	var currentCategory string
	var foundStartSection bool

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

				// Check if we've reached the specified start section
				if strings.Contains(headingText, startSection) {
					foundStartSection = true
					currentCategory = strings.TrimSpace(headingText)
				} else if foundStartSection {
					// Update current category for subsequent sections
					currentCategory = strings.TrimSpace(headingText)
				}
			}

		case *ast.List:
			if foundStartSection && currentCategory != "" {
				// Ensure category exists in map
				if _, exists := categoryMap[currentCategory]; !exists {
					categoryMap[currentCategory] = &Category{
						Name:     currentCategory,
						Projects: []Project{},
					}
				}

				// Parse list items as projects
				for child := n.FirstChild(); child != nil; child = child.NextSibling() {
					if listItem, ok := child.(*ast.ListItem); ok {
						project, err := UnmarshallProjectFromListItem(listItem, in)
						if err != nil {
							return ast.WalkStop, fmt.Errorf("failed to decode project: %v", err)
						}
						if project.Name != "" {
							categoryMap[currentCategory].Projects = append(categoryMap[currentCategory].Projects, project)
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

	if !foundStartSection {
		return nil, fmt.Errorf("%s section not found in the document", startSection)
	}

	// Convert map to slice to maintain order
	for _, category := range categoryMap {
		categories = append(categories, *category)
	}

	return categories, nil
}

// UnmarshallProjectFromListItem extracts project information from a list item
func UnmarshallProjectFromListItem(listItem *ast.ListItem, source []byte) (Project, error) {
	project := Project{}

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

// DecodeTextFromNode extracts text content from an AST node
func DecodeTextFromNode(node ast.Node, source []byte) (string, error) {
	var text strings.Builder
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if textNode, ok := n.(*ast.Text); ok {
				text.Write(textNode.Segment.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", err
	}
	return text.String(), nil
}
