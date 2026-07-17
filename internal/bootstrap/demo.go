package bootstrap

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
	"github.com/onatozmenn/offerpilot/internal/simulation"
)

const (
	DemoTemplateVersion = 1
	DemoTemplateSlug    = "offerpilot-demo-v1"
)

type DemoEngine interface {
	CreateExperiment(context.Context, domain.Experiment, []domain.Offer) (domain.Experiment, error)
	ListExperiments(context.Context, *service.ExperimentCursor, int) ([]domain.Experiment, error)
	GetExperimentDetail(context.Context, uuid.UUID) (domain.Experiment, []domain.Offer, error)
}

type Demo struct {
	engine  DemoEngine
	profile *simulation.Profile
	suffix  func() string
	mu      sync.Mutex
}

var demoSuffixPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func NewDemo(engine DemoEngine, profile *simulation.Profile, suffix func() string) (*Demo, error) {
	if engine == nil {
		return nil, fmt.Errorf("demo engine is required")
	}
	if profile == nil || profile.Version() != simulation.ProfileVersion {
		return nil, fmt.Errorf("valid simulation profile is required")
	}
	if suffix == nil {
		return nil, fmt.Errorf("demo suffix generator is required")
	}
	return &Demo{engine: engine, profile: profile, suffix: suffix}, nil
}

func NewDefaultDemo(engine DemoEngine) (*Demo, error) {
	return NewDemo(engine, simulation.DefaultProfile(), func() string { return uuid.NewString() })
}

func (demo *Demo) EnsureDemo(ctx context.Context) (domain.Experiment, []domain.Offer, error) {
	demo.mu.Lock()
	defer demo.mu.Unlock()

	experiment, offers, found, err := demo.findBySlug(ctx, DemoTemplateSlug)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	if found {
		return experiment, offers, nil
	}

	experiment, offers, err = demo.create(ctx, DemoTemplateSlug, "OfferPilot demo", domain.PolicyKindSegmentedEpsilonGreedy, floatPointer(0.15))
	if err == nil {
		return experiment, offers, nil
	}
	// A concurrent process may have won the unique-slug race.
	existing, existingOffers, found, lookupErr := demo.findBySlug(ctx, DemoTemplateSlug)
	if lookupErr == nil && found {
		return existing, existingOffers, nil
	}
	return domain.Experiment{}, nil, fmt.Errorf("create initial demo: %w", err)
}

func (demo *Demo) CreateFreshDemo(
	ctx context.Context,
	name string,
	policyKind domain.PolicyKind,
	epsilon *float64,
) (domain.Experiment, []domain.Offer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Experiment{}, nil, fmt.Errorf("demo name is required")
	}
	if err := domain.ValidateEpsilon(policyKind, epsilon); err != nil {
		return domain.Experiment{}, nil, err
	}
	suffix := strings.ToLower(strings.TrimSpace(demo.suffix()))
	if !demoSuffixPattern.MatchString(suffix) {
		return domain.Experiment{}, nil, fmt.Errorf("demo suffix must be a bounded lowercase slug")
	}
	return demo.create(ctx, DemoTemplateSlug+"-"+suffix, name, policyKind, cloneFloat(epsilon))
}

func (demo *Demo) create(
	ctx context.Context,
	slug string,
	name string,
	policyKind domain.PolicyKind,
	epsilon *float64,
) (domain.Experiment, []domain.Offer, error) {
	catalog := demo.profile.Catalog()
	if len(catalog) != 6 {
		return domain.Experiment{}, nil, fmt.Errorf("demo profile must contain six offers")
	}
	offers := make([]domain.Offer, len(catalog))
	seenCategories := make(map[domain.OfferCategory]struct{}, len(catalog))
	seenSlugs := make(map[string]struct{}, len(catalog))
	for index, template := range catalog {
		if _, exists := seenSlugs[template.Slug]; exists {
			return domain.Experiment{}, nil, fmt.Errorf("demo profile contains duplicate offer slug")
		}
		if _, exists := seenCategories[template.Category]; exists {
			return domain.Experiment{}, nil, fmt.Errorf("demo profile contains duplicate category")
		}
		seenSlugs[template.Slug] = struct{}{}
		seenCategories[template.Category] = struct{}{}
		offers[index] = domain.Offer{
			Slug:         template.Slug,
			MerchantName: template.MerchantName,
			Title:        template.Title,
			Description:  template.Description,
			Category:     template.Category,
			Active:       true,
		}
	}
	experiment := domain.Experiment{
		Slug:       slug,
		Name:       name,
		Status:     domain.ExperimentStatusRunning,
		PolicyKind: policyKind,
		Epsilon:    cloneFloat(epsilon),
	}
	created, err := demo.engine.CreateExperiment(ctx, experiment, offers)
	if err != nil {
		return domain.Experiment{}, nil, err
	}
	createdExperiment, createdOffers, err := demo.engine.GetExperimentDetail(ctx, created.ID)
	if err != nil {
		return domain.Experiment{}, nil, fmt.Errorf("load created demo: %w", err)
	}
	if err := domain.ValidateExperiment(createdExperiment); err != nil {
		return domain.Experiment{}, nil, fmt.Errorf("validate created demo experiment: %w", err)
	}
	if err := domain.ValidateOffers(createdExperiment.ID, createdOffers); err != nil {
		return domain.Experiment{}, nil, fmt.Errorf("validate created demo offers: %w", err)
	}
	return createdExperiment, createdOffers, nil
}

func (demo *Demo) findBySlug(ctx context.Context, slug string) (domain.Experiment, []domain.Offer, bool, error) {
	var cursor *service.ExperimentCursor
	for {
		experiments, err := demo.engine.ListExperiments(ctx, cursor, 100)
		if err != nil {
			return domain.Experiment{}, nil, false, fmt.Errorf("list experiments for demo: %w", err)
		}
		for _, experiment := range experiments {
			if experiment.Slug != slug {
				continue
			}
			loaded, offers, err := demo.engine.GetExperimentDetail(ctx, experiment.ID)
			if err != nil {
				return domain.Experiment{}, nil, false, fmt.Errorf("load existing demo: %w", err)
			}
			return loaded, offers, true, nil
		}
		if len(experiments) < 100 {
			return domain.Experiment{}, nil, false, nil
		}
		last := experiments[len(experiments)-1]
		cursor = &service.ExperimentCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
}

func floatPointer(value float64) *float64 {
	return &value
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
