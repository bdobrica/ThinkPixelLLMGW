package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"llm_gateway/internal/models"
)

// ModelWithDetails wraps a model with its pricing components
type ModelWithDetails struct {
	*models.Model
	PricingComponents []models.PricingComponent
}

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
		SELECT 
			id, model_name, provider_id, source, version, deprecation_date, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata,
			created_at, updated_at
		FROM models
		WHERE model_name = $1
	`

	err := r.db.conn.GetContext(ctx, &model, query, name)
	if err != nil {
		if err == sql.ErrNoRows {
			// Try to find by alias
			return r.getByAlias(ctx, name)
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	// Load pricing components
	if err := r.loadPricingComponents(ctx, &model); err != nil {
		return nil, fmt.Errorf("failed to load pricing components: %w", err)
	}

	// Cache the result
	r.cache.Set(name, &model)

	return &model, nil
}

// getByAlias retrieves a model by alias
func (r *ModelRepository) getByAlias(ctx context.Context, alias string) (*models.Model, error) {
	query := `
		SELECT 
			m.id, m.model_name, m.provider_id, m.source, m.version, m.deprecation_date, m.is_deprecated,
			m.supported_regions, m.supported_resolutions,
			m.supports_assistant_prefill, m.supports_audio_input, m.supports_audio_output,
			m.supports_computer_use, m.supports_embedding_image_input, m.supports_function_calling,
			m.supports_image_input, m.supports_native_streaming, m.supports_parallel_function_calling,
			m.supports_pdf_input, m.supports_prompt_caching, m.supports_reasoning,
			m.supports_response_schema, m.supports_service_tier, m.supports_system_messages,
			m.supports_tool_choice, m.supports_url_context, m.supports_video_input,
			m.supports_vision, m.supports_web_search,
			m.supports_text_input, m.supports_text_output, m.supports_image_output,
			m.supports_video_output, m.supports_batch_requests, m.supports_json_output,
			m.supports_rerank, m.supports_embedding_text_input, m.supports_streaming_output,
			m.tokens_per_minute, m.requests_per_minute, m.requests_per_day,
			m.max_tokens, m.max_input_tokens, m.max_output_tokens, m.max_query_tokens,
			m.max_tokens_per_document_chunk, m.max_document_chunks_per_query,
			m.tool_use_system_prompt_tokens, m.output_vector_size,
			m.max_audio_length_hours, m.max_audio_per_prompt, m.max_images_per_prompt,
			m.max_pdf_size_mb, m.max_video_length, m.max_videos_per_prompt,
			m.max_requests_per_second, m.max_concurrent_requests, m.max_batch_size,
			m.max_audio_length_seconds, m.max_video_length_seconds,
			m.max_context_window_tokens, m.max_output_tokens_per_request,
			m.max_input_tokens_per_request,
			m.currency, m.pricing_component_schema_version,
			m.average_latency_ms, m.p95_latency_ms, m.availability_slo, m.sla_tier, m.supports_sla,
			m.metadata_schema_version, m.metadata,
			m.created_at, m.updated_at
		FROM models m
		INNER JOIN model_aliases ma ON m.id = ma.target_model_id
		WHERE ma.alias = $1 AND ma.enabled = true
	`

	var model models.Model
	err := r.db.conn.GetContext(ctx, &model, query, alias)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("failed to get model by alias: %w", err)
	}

	// Load pricing components
	if err := r.loadPricingComponents(ctx, &model); err != nil {
		return nil, fmt.Errorf("failed to load pricing components: %w", err)
	}

	// Cache by both alias and actual name
	r.cache.Set(alias, &model)
	r.cache.Set(model.ModelName, &model)

	return &model, nil
}

// loadPricingComponents loads pricing components for a model
func (r *ModelRepository) loadPricingComponents(ctx context.Context, model *models.Model) error {
	query := `
		SELECT id, model_id, code, direction, modality, unit, tier, scope, price,
		       metadata_schema_version, metadata
		FROM pricing_components
		WHERE model_id = $1
		ORDER BY code
	`

	var components []models.PricingComponent
	err := r.db.conn.SelectContext(ctx, &components, query, model.ID)
	if err != nil {
		return err
	}

	model.PricingComponents = components
	return nil
}

// GetByID retrieves a model by ID
func (r *ModelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	var model models.Model
	query := `
		SELECT 
			id, model_name, provider_id, source, version, deprecation_date, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata,
			created_at, updated_at
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

	// Load pricing components
	if err := r.loadPricingComponents(ctx, &model); err != nil {
		return nil, fmt.Errorf("failed to load pricing components: %w", err)
	}

	return &model, nil
}

// GetByProvider retrieves all models for a provider
func (r *ModelRepository) GetByProvider(ctx context.Context, providerID string) ([]*models.Model, error) {
	query := `
		SELECT 
			id, model_name, provider_id, source, version, deprecation_date, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata,
			created_at, updated_at
		FROM models
		WHERE provider_id = $1
		ORDER BY model_name
	`

	var modelsList []*models.Model
	err := r.db.conn.SelectContext(ctx, &modelsList, query, providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get models by provider: %w", err)
	}

	// Load pricing components for each model
	for _, model := range modelsList {
		if err := r.loadPricingComponents(ctx, model); err != nil {
			return nil, fmt.Errorf("failed to load pricing components: %w", err)
		}
	}

	return modelsList, nil
}

// List returns all models (paginated)
func (r *ModelRepository) List(ctx context.Context, limit, offset int) ([]*models.Model, error) {
	query := `
		SELECT 
			id, model_name, provider_id, source, version, deprecation_date, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata,
			created_at, updated_at
		FROM models
		WHERE is_deprecated = false
		ORDER BY model_name
		LIMIT $1 OFFSET $2
	`

	var modelsList []*models.Model
	err := r.db.conn.SelectContext(ctx, &modelsList, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	// Load pricing components for each model
	for _, model := range modelsList {
		if err := r.loadPricingComponents(ctx, model); err != nil {
			return nil, fmt.Errorf("failed to load pricing components: %w", err)
		}
	}

	return modelsList, nil
}

// ModelListFilters contains filter parameters for listing models
type ModelListFilters struct {
	ProviderID string
	Search     string
	Page       int
	PageSize   int
}

// ModelListResult contains paginated model list results
type ModelListResult struct {
	Models     []*models.Model
	TotalCount int
	Page       int
	PageSize   int
}

// ListWithFilters returns models with filtering and pagination
func (r *ModelRepository) ListWithFilters(ctx context.Context, filters ModelListFilters) (*ModelListResult, error) {
	// Build WHERE clause
	var whereClauses []string
	var args []interface{}
	argCount := 1

	if filters.ProviderID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("provider_id = $%d", argCount))
		args = append(args, filters.ProviderID)
		argCount++
	}

	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("model_name ILIKE $%d", argCount))
		args = append(args, "%"+filters.Search+"%")
		argCount++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + whereClauses[0]
		for i := 1; i < len(whereClauses); i++ {
			whereClause += " AND " + whereClauses[i]
		}
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM models %s", whereClause)
	var totalCount int
	err := r.db.conn.GetContext(ctx, &totalCount, countQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count models: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	dataQuery := fmt.Sprintf(`
		SELECT 
			id, model_name, provider_id, source, version, deprecation_date, is_deprecated,
			supported_regions, supported_resolutions,
			supports_assistant_prefill, supports_audio_input, supports_audio_output,
			supports_computer_use, supports_embedding_image_input, supports_function_calling,
			supports_image_input, supports_native_streaming, supports_parallel_function_calling,
			supports_pdf_input, supports_prompt_caching, supports_reasoning,
			supports_response_schema, supports_service_tier, supports_system_messages,
			supports_tool_choice, supports_url_context, supports_video_input,
			supports_vision, supports_web_search,
			supports_text_input, supports_text_output, supports_image_output,
			supports_video_output, supports_batch_requests, supports_json_output,
			supports_rerank, supports_embedding_text_input, supports_streaming_output,
			tokens_per_minute, requests_per_minute, requests_per_day,
			max_tokens, max_input_tokens, max_output_tokens, max_query_tokens,
			max_tokens_per_document_chunk, max_document_chunks_per_query,
			tool_use_system_prompt_tokens, output_vector_size,
			max_audio_length_hours, max_audio_per_prompt, max_images_per_prompt,
			max_pdf_size_mb, max_video_length, max_videos_per_prompt,
			max_requests_per_second, max_concurrent_requests, max_batch_size,
			max_audio_length_seconds, max_video_length_seconds,
			max_context_window_tokens, max_output_tokens_per_request,
			max_input_tokens_per_request,
			currency, pricing_component_schema_version,
			average_latency_ms, p95_latency_ms, availability_slo, sla_tier, supports_sla,
			metadata_schema_version, metadata,
			created_at, updated_at
		FROM models
		%s
		ORDER BY model_name
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount, argCount+1)

	args = append(args, filters.PageSize, offset)

	var modelsList []*models.Model
	err = r.db.conn.SelectContext(ctx, &modelsList, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	// Load pricing components for each model
	for _, model := range modelsList {
		if err := r.loadPricingComponents(ctx, model); err != nil {
			return nil, fmt.Errorf("failed to load pricing components: %w", err)
		}
	}

	return &ModelListResult{
		Models:     modelsList,
		TotalCount: totalCount,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
	}, nil
}

// Delete deletes a model
func (r *ModelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Get model name before deletion to invalidate cache
	var modelName string
	err := r.db.conn.GetContext(ctx, &modelName, "SELECT model_name FROM models WHERE id = $1", id)
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
	if modelName != "" {
		r.cache.Delete(modelName)
	}

	return nil
}

// InvalidateCache removes a model from the cache
func (r *ModelRepository) InvalidateCache(modelName string) {
	r.cache.Delete(modelName)
}
