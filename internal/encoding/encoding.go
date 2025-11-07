package encoding

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	myawesomelistv1 "myawesomelist.shikanime.studio/pkgs/proto/myawesomelist/v1"
)

// options represents configuration options for parsing
type options struct {
	startSection         string
	endSection           string
	subsectionAsCategory bool
}

// Option is a function that configures options
type Option func(*options)

// WithStartSection sets the start section for parsing categories
func WithStartSection(section string) Option {
	return func(o *options) {
		o.startSection = section
	}
}

// WithEndSection sets the end section to stop parsing categories
func WithEndSection(section string) Option {
	return func(o *options) {
		o.endSection = section
	}
}

// New: treat H3 headings as separate categories under the current H2
func WithSubsectionAsCategory() Option {
	return func(o *options) {
		o.subsectionAsCategory = true
	}
}

// UnmarshallCollection parses projects from a repository's README and groups them by category
func UnmarshallCollection(in []byte, opts ...Option) (*myawesomelistv1.Collection, error) {
	options := &options{}
	for _, opt := range opts {
		opt(options)
	}

	// Create a goldmark parser
	root := goldmark.New().Parser().Parse(text.NewReader(in))

	// Find the specified start section and begin parsing from there
	var lang string
	var category string
	var foundStartSection bool
	var reachedEndSection bool
	var foundAwesomeHeader bool
	var currMainCat string
	categoriesMap := make(map[string]*myawesomelistv1.Category)

	// If no start section specified, start parsing immediately
	if options.startSection == "" {
		foundStartSection = true
	}

	// Walk through the AST
	err := ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		// Stop parsing if we've reached the end section
		if reachedEndSection {
			return ast.WalkStop, nil
		}

		switch n := node.(type) {
		case *ast.Heading:
			headingText, err := DecodeTextFromNode(n, in)
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to decode heading text: %v", err)
			}

			// Extract language from first heading
			if n.Level == 1 && !foundAwesomeHeader && strings.HasPrefix(strings.ToLower(headingText), "awesome ") {
				// Extract language from "Awesome {language}" format
				parts := strings.Fields(headingText)
				if len(parts) >= 2 {
					lang = strings.Join(parts[1:], " ")
				}
				foundAwesomeHeader = true
			}

			// Main category headings are level 2
			if n.Level == 2 {
				// Check if we've reached the end section
				if options.endSection != "" && foundStartSection && strings.Contains(headingText, options.endSection) {
					reachedEndSection = true
					return ast.WalkStop, nil
				}
				// Check if we've reached the specified start section
				if options.startSection != "" && strings.Contains(headingText, options.startSection) {
					foundStartSection = true
					currMainCat = strings.TrimSpace(headingText)
					category = currMainCat
				} else if foundStartSection {
					currMainCat = strings.TrimSpace(headingText)
					category = currMainCat
				}
			} else if n.Level == 3 && options.subsectionAsCategory && foundStartSection && currMainCat != "" {
				// Flatten subsections under the current main category
				sub := strings.TrimSpace(headingText)
				category = currMainCat + " - " + sub
			}

		case *ast.List:
			if foundStartSection && !reachedEndSection && category != "" {
				// Ensure category exists in map
				if _, exists := categoriesMap[category]; !exists {
					categoriesMap[category] = &myawesomelistv1.Category{
						Name:     category,
						Projects: []*myawesomelistv1.Project{},
					}
				}

				// Parse list items as projects
				for child := n.FirstChild(); child != nil; child = child.NextSibling() {
					if listItem, ok := child.(*ast.ListItem); ok {
						project, err := UnmarshallProjectFromListItem(listItem, in)
						if err != nil {
							return ast.WalkStop, fmt.Errorf("failed to decode project: %v", err)
						}
						if project.GetName() != "" {
							categoriesMap[category].Projects = append(categoriesMap[category].Projects, project)
						}
					}
				}
			}
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}

	if options.startSection != "" && !foundStartSection {
		return nil, fmt.Errorf("%s section not found in the document", options.startSection)
	}

	// Convert map to slice to maintain order
	var cats []*myawesomelistv1.Category
	for _, category := range categoriesMap {
		cats = append(cats, category)
	}

	return &myawesomelistv1.Collection{
		Language:   lang,
		Categories: cats,
	}, nil
}

// UnmarshallProjectFromListItem extracts project information from a list item
func UnmarshallProjectFromListItem(
	listItem *ast.ListItem,
	src []byte,
) (*myawesomelistv1.Project, error) {
	project := &myawesomelistv1.Project{
		Repo: &myawesomelistv1.Repository{},
	}

	err := ast.Walk(listItem, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Link:
			// Extract project name and URL
			urlValue, err := url.Parse(string(n.Destination))
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to parse project URL: %v", err)
			}
			owner := ""
			repo := ""
			path := strings.Trim(urlValue.Path, "/")
			parts := strings.Split(path, "/")
			if len(parts) >= 2 {
				owner = parts[0]
				repo = parts[1]
			} else if len(parts) == 1 {
				repo = parts[0]
			}

			hostname := urlValue.Hostname()
			if hostname == "" && len(parts) >= 2 {
				hostname = "github.com"
			}

			project.Repo.Hostname = hostname
			project.Repo.Owner = owner
			project.Repo.Repo = repo

			name, err := DecodeTextFromNode(n, src)
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to decode project name: %v", err)
			}
			project.Name = name

		case *ast.Text:
			// Extract description (text after the link)
			textValue, err := DecodeTextFromNode(n, src)
			if err != nil {
				return ast.WalkStop, fmt.Errorf("failed to decode project description: %v", err)
			}
			if project.Name != "" && strings.Contains(textValue, " - ") {
				// Split on " - " to get the description
				parts := strings.SplitN(textValue, " - ", 2)
				if len(parts) > 1 {
					project.Description = strings.TrimSpace(parts[1])
				}
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	return project, nil
}

// DecodeTextFromNode extracts text content from an AST node
func DecodeTextFromNode(node ast.Node, src []byte) (string, error) {
	var text strings.Builder
	err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if textNode, ok := n.(*ast.Text); ok {
				text.Write(textNode.Segment.Value(src))
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return "", err
	}
	return text.String(), nil
}
