package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplate(t *testing.T) {
	t.Run("Should_get_existing_template", func(t *testing.T) {
		template, err := GetTemplate("project_overview")
		require.NoError(t, err)
		assert.Equal(t, "Project Overview", template.Name)
		assert.Equal(t, "overview", template.Category)
		assert.Contains(t, template.Parameters, "project_id")
	})

	t.Run("Should_return_error_for_nonexistent_template", func(t *testing.T) {
		template, err := GetTemplate("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, template)
		assert.Contains(t, err.Error(), "template 'nonexistent' not found")
	})
}

func TestListTemplates(t *testing.T) {
	t.Run("Should_return_templates_grouped_by_category", func(t *testing.T) {
		categories := ListTemplates()
		assert.NotEmpty(t, categories)
		assert.Contains(t, categories, "overview")
		assert.Contains(t, categories, "functions")
		assert.Contains(t, categories, "dependencies")
	})
}

func TestGetTemplatesByCategory(t *testing.T) {
	t.Run("Should_return_templates_for_valid_category", func(t *testing.T) {
		templates := GetTemplatesByCategory("overview")
		assert.NotEmpty(t, templates)
		for _, template := range templates {
			assert.Equal(t, "overview", template.Category)
		}
	})

	t.Run("Should_return_empty_for_invalid_category", func(t *testing.T) {
		templates := GetTemplatesByCategory("invalid")
		assert.Empty(t, templates)
	})
}

func TestTemplate_ValidateParameters(t *testing.T) {
	template := &Template{
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
			"name":       "string - The name to search for",
		},
	}

	t.Run("Should_pass_with_all_required_parameters", func(t *testing.T) {
		params := map[string]any{
			"project_id": "test-project",
			"name":       "test-name",
		}
		err := template.ValidateParameters(params)
		assert.NoError(t, err)
	})

	t.Run("Should_fail_with_missing_parameters", func(t *testing.T) {
		params := map[string]any{
			"project_id": "test-project",
			// missing "name"
		}
		err := template.ValidateParameters(params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required parameter: name")
	})
}

func TestTemplate_BuildQuery(t *testing.T) {
	template := &Template{
		Query: "MATCH (n) WHERE n.project_id = $project_id RETURN n",
		Parameters: map[string]string{
			"project_id": "string - The project identifier",
		},
	}

	t.Run("Should_build_query_with_valid_parameters", func(t *testing.T) {
		params := map[string]any{
			"project_id": "test-project",
		}
		query, err := template.BuildQuery(params)
		require.NoError(t, err)
		assert.Equal(t, template.Query, query)
	})

	t.Run("Should_fail_with_missing_parameters", func(t *testing.T) {
		params := map[string]any{}
		query, err := template.BuildQuery(params)
		assert.Error(t, err)
		assert.Empty(t, query)
	})
}
