package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCorrectModelAlias(t *testing.T) {
	tests := []struct {
		name           string
		inputModel     string
		expectedModel  string
		expectCorrected bool
	}{
		{
			name:           "Exact match - claude-sonnet",
			inputModel:     "claude-sonnet",
			expectedModel:  "claude-sonnet-4-6",
			expectCorrected: true,
		},
		{
			name:           "Exact match - claude-opus",
			inputModel:     "claude-opus",
			expectedModel:  "claude-opus-4-6",
			expectCorrected: true,
		},
		{
			name:           "Exact match - gpt4o",
			inputModel:     "gpt4o",
			expectedModel:  "gpt-4o",
			expectCorrected: true,
		},
		{
			name:           "No correction needed - exact model",
			inputModel:     "claude-sonnet-4-6",
			expectedModel:  "claude-sonnet-4-6",
			expectCorrected: false,
		},
		{
			name:           "No correction needed - unknown model",
			inputModel:     "unknown-model",
			expectedModel:  "unknown-model",
			expectCorrected: false,
		},
		{
			name:           "Case insensitive match",
			inputModel:     "CLAUDE-SONNET",
			expectedModel:  "claude-sonnet-4-6",
			expectCorrected: true,
		},
		{
			name:           "Model with date suffix",
			inputModel:     "gpt-4o-2024-11-20",
			expectedModel:  "gpt-4o",
			expectCorrected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, corrected := correctModelAlias(tt.inputModel)
			assert.Equal(t, tt.expectCorrected, corrected)
			assert.Equal(t, tt.expectedModel, result)
		})
	}
}

func TestGetModelMaxTokens(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		expectedLimit uint
	}{
		{
			name:          "Claude Opus 4.6",
			modelName:     "claude-opus-4-6",
			expectedLimit: 32000,
		},
		{
			name:          "Claude Sonnet 4.6",
			modelName:     "claude-sonnet-4-6",
			expectedLimit: 16000,
		},
		{
			name:          "GPT-4o",
			modelName:     "gpt-4o",
			expectedLimit: 16384,
		},
		{
			name:          "Unknown model - should return default",
			modelName:     "unknown-model",
			expectedLimit: 4096,
		},
		{
			name:          "Model with date suffix",
			modelName:     "claude-sonnet-4-6-20250514",
			expectedLimit: 16000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := getModelMaxTokens(tt.modelName)
			assert.Equal(t, tt.expectedLimit, limit)
		})
	}
}

func TestDeprecatedModelsMapping(t *testing.T) {
	tests := []struct {
		name              string
		inputModel        string
		expectedReplacement string
		expectDeprecated  bool
	}{
		{
			name:              "Claude 2 -> Claude Sonnet 4.6",
			inputModel:        "claude-2",
			expectedReplacement: "claude-sonnet-4-6",
			expectDeprecated:  true,
		},
		{
			name:              "Claude 2.1 -> Claude Sonnet 4.6",
			inputModel:        "claude-2.1",
			expectedReplacement: "claude-sonnet-4-6",
			expectDeprecated:  true,
		},
		{
			name:              "GPT-4-0314 -> GPT-4-turbo",
			inputModel:        "gpt-4-0314",
			expectedReplacement: "gpt-4-turbo",
			expectDeprecated:  true,
		},
		{
			name:              "Current model - not deprecated",
			inputModel:        "claude-sonnet-4-6",
			expectedReplacement: "",
			expectDeprecated:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replacement, isDeprecated := DeprecatedModels[tt.inputModel]
			assert.Equal(t, tt.expectDeprecated, isDeprecated)
			if tt.expectDeprecated {
				assert.Equal(t, tt.expectedReplacement, replacement)
			}
		})
	}
}

func TestCorrectRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                   string
		request                dto.Request
		expectedModel          string
		expectedMaxTokens      uint
		expectAdjusted         bool
	}{
		{
			name: "Model alias correction",
			request: &dto.GeneralOpenAIRequest{
				Model: "claude-sonnet",
			},
			expectedModel:     "claude-sonnet-4-6",
			expectAdjusted:    true,
		},
		{
			name: "Max tokens correction",
			request: &dto.GeneralOpenAIRequest{
				Model:     "claude-sonnet-4-6",
				MaxTokens: ptr(uint(50000)),
			},
			expectedModel:     "claude-sonnet-4-6",
			expectedMaxTokens: 16000,
			expectAdjusted:    true,
		},
		{
			name: "Deprecated model forwarding",
			request: &dto.GeneralOpenAIRequest{
				Model: "claude-2",
			},
			expectedModel:     "claude-sonnet-4-6",
			expectAdjusted:    true,
		},
		{
			name: "No correction needed",
			request: &dto.GeneralOpenAIRequest{
				Model:     "claude-sonnet-4-6",
				MaxTokens: ptr(uint(8000)),
			},
			expectedModel:     "claude-sonnet-4-6",
			expectedMaxTokens: 8000,
			expectAdjusted:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			result := CorrectRequest(c, tt.request)

			assert.Equal(t, tt.expectAdjusted, result.WasAdjusted)
			if tt.expectedModel != "" {
				switch r := tt.request.(type) {
				case *dto.GeneralOpenAIRequest:
					assert.Equal(t, tt.expectedModel, r.Model)
					if tt.expectedMaxTokens > 0 && r.MaxTokens != nil {
						assert.Equal(t, tt.expectedMaxTokens, *r.MaxTokens)
					}
				}
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
