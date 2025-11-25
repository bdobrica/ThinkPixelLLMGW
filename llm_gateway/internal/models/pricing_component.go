package models

//
// Pricing enums (stored as TEXT in Postgres)
//

type PricingDirection string
type PricingModality string
type PricingUnit string
type PricingTier string
type PricingScope string

const (
	PricingDirectionInput  PricingDirection = "input"
	PricingDirectionOutput PricingDirection = "output"
	PricingDirectionTool   PricingDirection = "tool"
	PricingDirectionCache  PricingDirection = "cache"

	PricingModalityText    PricingModality = "text"
	PricingModalityImage   PricingModality = "image"
	PricingModalityAudio   PricingModality = "audio"
	PricingModalityVideo   PricingModality = "video"
	PricingModalityTool    PricingModality = "tool"
	PricingModalityGeneric PricingModality = "generic"

	PricingUnitToken     PricingUnit = "token"
	PricingUnit1KTokens  PricingUnit = "1k_tokens"
	PricingUnitCharacter PricingUnit = "character"
	PricingUnitImage     PricingUnit = "image"
	PricingUnitPixel     PricingUnit = "pixel"
	PricingUnitSecond    PricingUnit = "second"
	PricingUnitPage      PricingUnit = "page"
	PricingUnitGBPerDay  PricingUnit = "gb_per_day"

	PricingTierDefault   PricingTier = "default"
	PricingTierAbove128K PricingTier = "above_128k"
	PricingTierAbove200K PricingTier = "above_200k"
	PricingTierPriority  PricingTier = "priority"
	PricingTierFlex      PricingTier = "flex"
	PricingTierPremium   PricingTier = "premium"

	PricingScopeRequest  PricingScope = "request"
	PricingScopeSession  PricingScope = "session"
	PricingScopeQuery    PricingScope = "query"
	PricingScopePage     PricingScope = "page"
	PricingScopeGBPerDay PricingScope = "gb_per_day"
)

//
// PricingComponent (pricing_components table)
//

type PricingComponent struct {
	// Primary key
	ID string `db:"id" json:"id"` // uuid

	// FK to models.id
	ModelID string `db:"model_id" json:"model_id"`

	// Business identifier (e.g. "input_text_default")
	Code string `db:"code" json:"code"`

	Direction PricingDirection `db:"direction" json:"direction"`
	Modality  PricingModality  `db:"modality" json:"modality"`
	Unit      PricingUnit      `db:"unit" json:"unit"`
	Tier      *string          `db:"tier" json:"tier,omitempty"`
	Scope     *string          `db:"scope" json:"scope,omitempty"`

	// Price in Model.Currency
	Price float64 `db:"price" json:"price"`

	// Provider-specific extras
	MetadataSchemaVersion *string `db:"metadata_schema_version" json:"metadata_schema_version,omitempty"`
	Metadata              JSONB   `db:"metadata" json:"metadata,omitempty"`
}
