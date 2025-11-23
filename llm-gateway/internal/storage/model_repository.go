package storage

import (
	"context"
	"database/sql"
	"fmt"
	
	"github.com/google/uuid"
	
	"gateway/internal/models"
)

// ModelRepository handles model database operations with caching
type ModelRepository struct {
	db    *DB
	cache *LRUCache
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *DB) *ModelRepository {
	return &ModelRepository{
		db:    db,
		cache: db.GetModelCache(),
	}
}

// GetByName retrieves a model by name (with caching)
func (r *ModelRepository) GetByName(ctx context.Context, name string) (*models.Model, error) {
	// Check cache first
	if cached, found := r.cache.Get(name); found {
		return cached.(*models.Model), nil
	}
	
	// Query database
	var model models.Model
	query := `
		SELECT id, provider_id, name, input_cost_per_token, output_cost_per_token,
		       max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
		       input_cost_per_audio_per_second, input_cost_per_video_per_second,
		       output_cost_per_image, output_cost_per_audio_per_second,
		       output_cost_per_video_per_second, output_vector_size, litellm_provider,
		       mode, supports_function_calling, supports_parallel_function_calling,
		       supports_vision, supports_tool_choice, supports_response_schema,
		       supports_prompt_caching, supports_audio_input, supports_audio_output,
		       supports_pdf_input, supports_video_input, supports_image_input,
		       supports_system_messages, source, metadata, created_at, updated_at
		FROM models
		WHERE name = $1
	`
	
	err := r.db.conn.GetContext(ctx, &model, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Try to find by alias
			return r.getByAlias(ctx, name)
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	
	// Cache the result
	r.cache.Set(name, &model)
	
	return &model, nil
}

// getByAlias retrieves a model by alias
func (r *ModelRepository) getByAlias(ctx context.Context, alias string) (*models.Model, error) {
	query := `
		SELECT m.id, m.provider_id, m.name, m.input_cost_per_token, m.output_cost_per_token,
		       m.max_tokens, m.max_input_tokens, m.max_output_tokens, m.input_cost_per_image,
		       m.input_cost_per_audio_per_second, m.input_cost_per_video_per_second,
		       m.output_cost_per_image, m.output_cost_per_audio_per_second,
		       m.output_cost_per_video_per_second, m.output_vector_size, m.litellm_provider,
		       m.mode, m.supports_function_calling, m.supports_parallel_function_calling,
		       m.supports_vision, m.supports_tool_choice, m.supports_response_schema,
		       m.supports_prompt_caching, m.supports_audio_input, m.supports_audio_output,
		       m.supports_pdf_input, m.supports_video_input, m.supports_image_input,
		       m.supports_system_messages, m.source, m.metadata, m.created_at, m.updated_at
		FROM models m
		INNER JOIN model_aliases ma ON m.id = ma.model_id
		WHERE ma.alias = $1
	`
	
	var model models.Model
	err := r.db.conn.GetContext(ctx, &model, query, alias)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("failed to get model by alias: %w", err)
	}
	
	// Cache by both alias and actual name
	r.cache.Set(alias, &model)
	r.cache.Set(model.Name, &model)
	
	return &model, nil
}

// GetByID retrieves a model by ID
func (r *ModelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	var model models.Model
	query := `
		SELECT id, provider_id, name, input_cost_per_token, output_cost_per_token,
		       max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
		       input_cost_per_audio_per_second, input_cost_per_video_per_second,
		       output_cost_per_image, output_cost_per_audio_per_second,
		       output_cost_per_video_per_second, output_vector_size, litellm_provider,
		       mode, supports_function_calling, supports_parallel_function_calling,
		       supports_vision, supports_tool_choice, supports_response_schema,
		       supports_prompt_caching, supports_audio_input, supports_audio_output,
		       supports_pdf_input, supports_video_input, supports_image_input,
		       supports_system_messages, source, metadata, created_at, updated_at
		FROM models
		WHERE id = $1
	`
	
	err := r.db.conn.GetContext(ctx, &model, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	
	return &model, nil
}

// GetByProvider retrieves all models for a provider
func (r *ModelRepository) GetByProvider(ctx context.Context, providerID uuid.UUID) ([]*models.Model, error) {
	query := `
		SELECT id, provider_id, name, input_cost_per_token, output_cost_per_token,
		       max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
		       input_cost_per_audio_per_second, input_cost_per_video_per_second,
		       output_cost_per_image, output_cost_per_audio_per_second,
		       output_cost_per_video_per_second, output_vector_size, litellm_provider,
		       mode, supports_function_calling, supports_parallel_function_calling,
		       supports_vision, supports_tool_choice, supports_response_schema,
		       supports_prompt_caching, supports_audio_input, supports_audio_output,
		       supports_pdf_input, supports_video_input, supports_image_input,
		       supports_system_messages, source, metadata, created_at, updated_at
		FROM models
		WHERE provider_id = $1
		ORDER BY name
	`
	
	var modelsList []*models.Model
	err := r.db.conn.SelectContext(ctx, &modelsList, query, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get models by provider: %w", err)
	}
	
	return modelsList, nil
}

// Create creates a new model
func (r *ModelRepository) Create(ctx context.Context, model *models.Model) error {
	query := `
		INSERT INTO models (
			id, provider_id, name, input_cost_per_token, output_cost_per_token,
			max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
			input_cost_per_audio_per_second, input_cost_per_video_per_second,
			output_cost_per_image, output_cost_per_audio_per_second,
			output_cost_per_video_per_second, output_vector_size, litellm_provider,
			mode, supports_function_calling, supports_parallel_function_calling,
			supports_vision, supports_tool_choice, supports_response_schema,
			supports_prompt_caching, supports_audio_input, supports_audio_output,
			supports_pdf_input, supports_video_input, supports_image_input,
			supports_system_messages, source, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31
		)
		RETURNING created_at, updated_at
	`
	
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}
	
	err := r.db.conn.QueryRowxContext(
		ctx, query,
		model.ID, model.ProviderID, model.Name, model.InputCostPerToken, model.OutputCostPerToken,
		model.MaxTokens, model.MaxInputTokens, model.MaxOutputTokens, model.InputCostPerImage,
		model.InputCostPerAudioPerSecond, model.InputCostPerVideoPerSecond,
		model.OutputCostPerImage, model.OutputCostPerAudioPerSecond,
		model.OutputCostPerVideoPerSecond, model.OutputVectorSize, model.LiteLLMProvider,
		model.Mode, model.SupportsFunctionCalling, model.SupportsParallelFunctionCalling,
		model.SupportsVision, model.SupportsToolChoice, model.SupportsResponseSchema,
		model.SupportsPromptCaching, model.SupportsAudioInput, model.SupportsAudioOutput,
		model.SupportsPDFInput, model.SupportsVideoInput, model.SupportsImageInput,
		model.SupportsSystemMessages, model.Source, model.Metadata,
	).Scan(&model.CreatedAt, &model.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	
	// Invalidate cache
	r.cache.Delete(model.Name)
	
	return nil
}

// Update updates an existing model
func (r *ModelRepository) Update(ctx context.Context, model *models.Model) error {
	query := `
		UPDATE models
		SET provider_id = $2, name = $3, input_cost_per_token = $4, output_cost_per_token = $5,
		    max_tokens = $6, max_input_tokens = $7, max_output_tokens = $8,
		    input_cost_per_image = $9, input_cost_per_audio_per_second = $10,
		    input_cost_per_video_per_second = $11, output_cost_per_image = $12,
		    output_cost_per_audio_per_second = $13, output_cost_per_video_per_second = $14,
		    output_vector_size = $15, litellm_provider = $16, mode = $17,
		    supports_function_calling = $18, supports_parallel_function_calling = $19,
		    supports_vision = $20, supports_tool_choice = $21, supports_response_schema = $22,
		    supports_prompt_caching = $23, supports_audio_input = $24, supports_audio_output = $25,
		    supports_pdf_input = $26, supports_video_input = $27, supports_image_input = $28,
		    supports_system_messages = $29, source = $30, metadata = $31
		WHERE id = $1
		RETURNING updated_at
	`
	
	err := r.db.conn.QueryRowxContext(
		ctx, query,
		model.ID, model.ProviderID, model.Name, model.InputCostPerToken, model.OutputCostPerToken,
		model.MaxTokens, model.MaxInputTokens, model.MaxOutputTokens, model.InputCostPerImage,
		model.InputCostPerAudioPerSecond, model.InputCostPerVideoPerSecond,
		model.OutputCostPerImage, model.OutputCostPerAudioPerSecond,
		model.OutputCostPerVideoPerSecond, model.OutputVectorSize, model.LiteLLMProvider,
		model.Mode, model.SupportsFunctionCalling, model.SupportsParallelFunctionCalling,
		model.SupportsVision, model.SupportsToolChoice, model.SupportsResponseSchema,
		model.SupportsPromptCaching, model.SupportsAudioInput, model.SupportsAudioOutput,
		model.SupportsPDFInput, model.SupportsVideoInput, model.SupportsImageInput,
		model.SupportsSystemMessages, model.Source, model.Metadata,
	).Scan(&model.UpdatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrModelNotFound
		}
		return fmt.Errorf("failed to update model: %w", err)
	}
	
	// Invalidate cache
	r.cache.Delete(model.Name)
	
	return nil
}

// Delete deletes a model
func (r *ModelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Get model name before deletion to invalidate cache
	var name string
	err := r.db.conn.GetContext(ctx, &name, "SELECT name FROM models WHERE id = $1", id)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get model name: %w", err)
	}
	
	query := "DELETE FROM models WHERE id = $1"
	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rows == 0 {
		return ErrModelNotFound
	}
	
	// Invalidate cache
	if name != "" {
		r.cache.Delete(name)
	}
	
	return nil
}

// List returns all models (paginated)
func (r *ModelRepository) List(ctx context.Context, limit, offset int) ([]*models.Model, error) {
	query := `
		SELECT id, provider_id, name, input_cost_per_token, output_cost_per_token,
		       max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
		       input_cost_per_audio_per_second, input_cost_per_video_per_second,
		       output_cost_per_image, output_cost_per_audio_per_second,
		       output_cost_per_video_per_second, output_vector_size, litellm_provider,
		       mode, supports_function_calling, supports_parallel_function_calling,
		       supports_vision, supports_tool_choice, supports_response_schema,
		       supports_prompt_caching, supports_audio_input, supports_audio_output,
		       supports_pdf_input, supports_video_input, supports_image_input,
		       supports_system_messages, source, metadata, created_at, updated_at
		FROM models
		ORDER BY name
		LIMIT $1 OFFSET $2
	`
	
	var modelsList []*models.Model
	err := r.db.conn.SelectContext(ctx, &modelsList, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	
	return modelsList, nil
}

// Search searches for models by name (fuzzy matching)
func (r *ModelRepository) Search(ctx context.Context, searchTerm string, limit int) ([]*models.Model, error) {
	query := `
		SELECT id, provider_id, name, input_cost_per_token, output_cost_per_token,
		       max_tokens, max_input_tokens, max_output_tokens, input_cost_per_image,
		       input_cost_per_audio_per_second, input_cost_per_video_per_second,
		       output_cost_per_image, output_cost_per_audio_per_second,
		       output_cost_per_video_per_second, output_vector_size, litellm_provider,
		       mode, supports_function_calling, supports_parallel_function_calling,
		       supports_vision, supports_tool_choice, supports_response_schema,
		       supports_prompt_caching, supports_audio_input, supports_audio_output,
		       supports_pdf_input, supports_video_input, supports_image_input,
		       supports_system_messages, source, metadata, created_at, updated_at
		FROM models
		WHERE name % $1
		ORDER BY similarity(name, $1) DESC
		LIMIT $2
	`
	
	var modelsList []*models.Model
	err := r.db.conn.SelectContext(ctx, &modelsList, query, searchTerm, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search models: %w", err)
	}
	
	return modelsList, nil
}

// InvalidateCache removes a model from the cache
func (r *ModelRepository) InvalidateCache(name string) {
	r.cache.Delete(name)
}
