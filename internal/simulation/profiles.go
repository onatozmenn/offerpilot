package simulation

import (
	"fmt"
	"math"

	"github.com/onatozmenn/offerpilot/internal/domain"
)

const ProfileVersion = 1

const (
	OfferSlugOrbitMeadow  = "orbit-meadow-travel"
	OfferSlugLanternFork  = "lantern-fork-dining"
	OfferSlugWillowSpring = "willow-spring-wellness"
	OfferSlugNookNest     = "nook-nest-home"
	OfferSlugCircuitBloom = "circuit-bloom-tech"
	OfferSlugEmberStage   = "ember-stage-entertainment"
)

type Float64Source interface {
	Float64() (float64, error)
}

type CatalogOffer struct {
	Slug         string
	MerchantName string
	Title        string
	Description  string
	Category     domain.OfferCategory
}

type OutcomeProbabilities struct {
	Ignored   float64
	Clicked   float64
	Converted float64
}

type Profile struct {
	version int
	catalog []CatalogOffer
	hidden  map[string]hiddenOfferProfile
}

type hiddenOfferProfile struct {
	baseClick        float64
	baseConversion   float64
	preferredDevice  domain.DeviceClass
	preferredDaypart domain.Daypart
}

func DefaultProfile() *Profile {
	catalog := []CatalogOffer{
		{Slug: OfferSlugOrbitMeadow, MerchantName: "Orbit Meadow", Title: "Small-world weekend escape", Description: "A fictional short-break marketplace offer.", Category: domain.OfferCategoryTravel},
		{Slug: OfferSlugLanternFork, MerchantName: "Lantern Fork", Title: "Neighborhood tasting bundle", Description: "A fictional dining marketplace offer.", Category: domain.OfferCategoryDining},
		{Slug: OfferSlugWillowSpring, MerchantName: "Willow Spring", Title: "Everyday reset collection", Description: "A fictional wellness marketplace offer.", Category: domain.OfferCategoryWellness},
		{Slug: OfferSlugNookNest, MerchantName: "Nook & Nest", Title: "Room refresh essentials", Description: "A fictional home marketplace offer.", Category: domain.OfferCategoryHome},
		{Slug: OfferSlugCircuitBloom, MerchantName: "Circuit Bloom", Title: "Desk upgrade selection", Description: "A fictional technology marketplace offer.", Category: domain.OfferCategoryTechnology},
		{Slug: OfferSlugEmberStage, MerchantName: "Ember Stage", Title: "Local night-out pass", Description: "A fictional entertainment marketplace offer.", Category: domain.OfferCategoryEntertainment},
	}
	hidden := map[string]hiddenOfferProfile{
		OfferSlugOrbitMeadow:  {baseClick: 0.13, baseConversion: 0.035, preferredDevice: domain.DeviceClassMobile, preferredDaypart: domain.DaypartEvening},
		OfferSlugLanternFork:  {baseClick: 0.16, baseConversion: 0.045, preferredDevice: domain.DeviceClassMobile, preferredDaypart: domain.DaypartEvening},
		OfferSlugWillowSpring: {baseClick: 0.14, baseConversion: 0.040, preferredDevice: domain.DeviceClassTablet, preferredDaypart: domain.DaypartMorning},
		OfferSlugNookNest:     {baseClick: 0.12, baseConversion: 0.050, preferredDevice: domain.DeviceClassDesktop, preferredDaypart: domain.DaypartAfternoon},
		OfferSlugCircuitBloom: {baseClick: 0.18, baseConversion: 0.030, preferredDevice: domain.DeviceClassDesktop, preferredDaypart: domain.DaypartNight},
		OfferSlugEmberStage:   {baseClick: 0.15, baseConversion: 0.055, preferredDevice: domain.DeviceClassMobile, preferredDaypart: domain.DaypartNight},
	}
	return &Profile{version: ProfileVersion, catalog: catalog, hidden: hidden}
}

func (profile *Profile) Version() int {
	if profile == nil {
		return 0
	}
	return profile.version
}

func (profile *Profile) Catalog() []CatalogOffer {
	if profile == nil {
		return nil
	}
	return append([]CatalogOffer(nil), profile.catalog...)
}

func (profile *Profile) Context(source Float64Source) (domain.SessionContext, error) {
	if profile == nil || profile.version != ProfileVersion {
		return domain.SessionContext{}, fmt.Errorf("profile is unavailable")
	}
	if source == nil {
		return domain.SessionContext{}, fmt.Errorf("random source is required")
	}
	device, err := choose(source, []domain.DeviceClass{domain.DeviceClassMobile, domain.DeviceClassDesktop, domain.DeviceClassTablet}, []float64{0.55, 0.35, 0.10})
	if err != nil {
		return domain.SessionContext{}, fmt.Errorf("draw device class: %w", err)
	}
	daypart, err := choose(source, []domain.Daypart{domain.DaypartMorning, domain.DaypartAfternoon, domain.DaypartEvening, domain.DaypartNight}, []float64{0.22, 0.28, 0.35, 0.15})
	if err != nil {
		return domain.SessionContext{}, fmt.Errorf("draw daypart: %w", err)
	}
	categories := make([]domain.OfferCategory, len(profile.catalog))
	categoryWeights := make([]float64, len(profile.catalog))
	for index, offer := range profile.catalog {
		categories[index] = offer.Category
		categoryWeights[index] = 1 / float64(len(profile.catalog))
	}
	category, err := choose(source, categories, categoryWeights)
	if err != nil {
		return domain.SessionContext{}, fmt.Errorf("draw category affinity: %w", err)
	}
	visitor, err := choose(source, []domain.VisitorType{domain.VisitorTypeNew, domain.VisitorTypeReturning}, []float64{0.62, 0.38})
	if err != nil {
		return domain.SessionContext{}, fmt.Errorf("draw visitor type: %w", err)
	}
	contextValue := domain.SessionContext{DeviceClass: device, Daypart: daypart, CategoryAffinity: category, VisitorType: visitor}
	if err := domain.ValidateSessionContext(contextValue); err != nil {
		return domain.SessionContext{}, fmt.Errorf("validate generated context: %w", err)
	}
	return contextValue, nil
}

func (profile *Profile) Probabilities(contextValue domain.SessionContext, offerSlug string) (OutcomeProbabilities, error) {
	if profile == nil || profile.version != ProfileVersion {
		return OutcomeProbabilities{}, fmt.Errorf("profile is unavailable")
	}
	if err := domain.ValidateSessionContext(contextValue); err != nil {
		return OutcomeProbabilities{}, err
	}
	hidden, exists := profile.hidden[offerSlug]
	if !exists {
		return OutcomeProbabilities{}, fmt.Errorf("unknown fictional offer slug %q", offerSlug)
	}
	offerCategory, exists := profile.categoryForSlug(offerSlug)
	if !exists {
		return OutcomeProbabilities{}, fmt.Errorf("profile catalog mismatch for %q", offerSlug)
	}
	clicked := hidden.baseClick
	converted := hidden.baseConversion
	if contextValue.CategoryAffinity == offerCategory {
		clicked += 0.08
		converted += 0.04
	}
	if contextValue.DeviceClass == hidden.preferredDevice {
		clicked += 0.015
	}
	if contextValue.Daypart == hidden.preferredDaypart {
		clicked += 0.02
	}
	if contextValue.VisitorType == domain.VisitorTypeReturning {
		converted += 0.015
	}
	probabilities := OutcomeProbabilities{Clicked: clicked, Converted: converted}
	probabilities.Ignored = 1 - probabilities.Clicked - probabilities.Converted
	if err := validateOutcomeProbabilities(probabilities); err != nil {
		return OutcomeProbabilities{}, fmt.Errorf("invalid hidden profile for %q: %w", offerSlug, err)
	}
	return probabilities, nil
}

func (profile *Profile) DrawOutcome(source Float64Source, contextValue domain.SessionContext, offerSlug string) (domain.OutcomeKind, error) {
	if source == nil {
		return "", fmt.Errorf("random source is required")
	}
	probabilities, err := profile.Probabilities(contextValue, offerSlug)
	if err != nil {
		return "", err
	}
	draw, err := source.Float64()
	if err != nil {
		return "", fmt.Errorf("draw outcome: %w", err)
	}
	if !finiteProfile(draw) || draw < 0 || draw >= 1 {
		return "", fmt.Errorf("outcome draw must be in [0,1)")
	}
	if draw < probabilities.Ignored {
		return domain.OutcomeKindIgnored, nil
	}
	if draw < probabilities.Ignored+probabilities.Clicked {
		return domain.OutcomeKindClicked, nil
	}
	return domain.OutcomeKindConverted, nil
}

func (profile *Profile) UniformExpectedReward(contextValue domain.SessionContext, offerSlugs []string) (float64, error) {
	if len(offerSlugs) == 0 {
		return 0, fmt.Errorf("offer slugs must not be empty")
	}
	total := 0.0
	seen := make(map[string]struct{}, len(offerSlugs))
	for _, offerSlug := range offerSlugs {
		if _, exists := seen[offerSlug]; exists {
			return 0, fmt.Errorf("offer slugs contain duplicates")
		}
		seen[offerSlug] = struct{}{}
		expected, err := profile.ExpectedReward(contextValue, offerSlug)
		if err != nil {
			return 0, err
		}
		total += expected
	}
	return total / float64(len(offerSlugs)), nil
}

func (profile *Profile) OracleExpectedReward(contextValue domain.SessionContext, offerSlugs []string) (float64, error) {
	if len(offerSlugs) == 0 {
		return 0, fmt.Errorf("offer slugs must not be empty")
	}
	best := math.Inf(-1)
	seen := make(map[string]struct{}, len(offerSlugs))
	for _, offerSlug := range offerSlugs {
		if _, exists := seen[offerSlug]; exists {
			return 0, fmt.Errorf("offer slugs contain duplicates")
		}
		seen[offerSlug] = struct{}{}
		expected, err := profile.ExpectedReward(contextValue, offerSlug)
		if err != nil {
			return 0, err
		}
		if expected > best {
			best = expected
		}
	}
	return best, nil
}

func (profile *Profile) ExpectedReward(contextValue domain.SessionContext, offerSlug string) (float64, error) {
	probabilities, err := profile.Probabilities(contextValue, offerSlug)
	if err != nil {
		return 0, err
	}
	expected := probabilities.Clicked*0.25 + probabilities.Converted
	if !finiteProfile(expected) || expected < 0 || expected > 1 {
		return 0, fmt.Errorf("expected reward is invalid")
	}
	return expected, nil
}

func (profile *Profile) categoryForSlug(slug string) (domain.OfferCategory, bool) {
	for _, offer := range profile.catalog {
		if offer.Slug == slug {
			return offer.Category, true
		}
	}
	return "", false
}

func choose[T any](source Float64Source, values []T, weights []float64) (T, error) {
	var zero T
	if len(values) == 0 || len(values) != len(weights) {
		return zero, fmt.Errorf("weighted choices are invalid")
	}
	draw, err := source.Float64()
	if err != nil {
		return zero, err
	}
	if !finiteProfile(draw) || draw < 0 || draw >= 1 {
		return zero, fmt.Errorf("random draw must be in [0,1)")
	}
	total := 0.0
	for _, weight := range weights {
		if !finiteProfile(weight) || weight < 0 {
			return zero, fmt.Errorf("choice weight is invalid")
		}
		total += weight
	}
	if math.Abs(total-1) > domain.ProbabilityTolerance {
		return zero, fmt.Errorf("choice weights must sum to one")
	}
	cumulative := 0.0
	for index, weight := range weights {
		cumulative += weight
		if draw < cumulative {
			return values[index], nil
		}
	}
	return values[len(values)-1], nil
}

func validateOutcomeProbabilities(probabilities OutcomeProbabilities) error {
	values := []float64{probabilities.Ignored, probabilities.Clicked, probabilities.Converted}
	total := 0.0
	for _, value := range values {
		if !finiteProfile(value) || value < 0 || value > 1 {
			return fmt.Errorf("outcome probability must be finite and between zero and one")
		}
		total += value
	}
	if math.Abs(total-1) > domain.ProbabilityTolerance {
		return fmt.Errorf("outcome probabilities must sum to one")
	}
	return nil
}

func finiteProfile(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
