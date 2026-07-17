package bootstrap

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
	"github.com/onatozmenn/offerpilot/internal/simulation"
)

func TestDemo_CatalogAndPolicyDefaults(t *testing.T) {
	engine := newDemoFakeEngine()
	demo := newTestDemo(t, engine)
	experiment, offers, err := demo.EnsureDemo(context.Background())
	if err != nil {
		t.Fatalf("EnsureDemo: %v", err)
	}
	if experiment.Slug != DemoTemplateSlug || experiment.Status != domain.ExperimentStatusRunning || experiment.PolicyKind != domain.PolicyKindSegmentedEpsilonGreedy || experiment.Epsilon == nil || *experiment.Epsilon != 0.15 || experiment.PolicyVersion != 1 {
		t.Fatalf("experiment = %#v", experiment)
	}
	assertDemoCatalog(t, experiment, offers)
	if engine.createCount != 1 {
		t.Fatalf("create count = %d, want 1", engine.createCount)
	}

	profileCatalog := simulation.DefaultProfile().Catalog()
	profileCategories := make(map[domain.OfferCategory]string, len(profileCatalog))
	for _, offer := range profileCatalog {
		profileCategories[offer.Category] = offer.Slug
	}
	for _, offer := range offers {
		if profileCategories[offer.Category] != offer.Slug {
			t.Fatalf("offer/profile mismatch: %#v", offer)
		}
	}
}

func TestDemo_EnsureIsConcurrentAndIdempotent(t *testing.T) {
	engine := newDemoFakeEngine()
	demo := newTestDemo(t, engine)
	const callers = 32
	results := make(chan domain.Experiment, callers)
	errorsChannel := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			experiment, offers, err := demo.EnsureDemo(context.Background())
			if err != nil {
				errorsChannel <- err
				return
			}
			if len(offers) != 6 {
				errorsChannel <- fmt.Errorf("offers = %d", len(offers))
				return
			}
			results <- experiment
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("EnsureDemo: %v", err)
	}
	var firstID uuid.UUID
	for experiment := range results {
		if firstID == uuid.Nil {
			firstID = experiment.ID
		}
		if experiment.ID != firstID {
			t.Fatalf("EnsureDemo returned distinct IDs: %s and %s", firstID, experiment.ID)
		}
	}
	if engine.createCount != 1 {
		t.Fatalf("create count = %d, want 1", engine.createCount)
	}
}

func TestDemo_FreshCreationPreservesHistory(t *testing.T) {
	engine := newDemoFakeEngine()
	demo := newTestDemo(t, engine)
	initial, _, err := demo.EnsureDemo(context.Background())
	if err != nil {
		t.Fatalf("EnsureDemo: %v", err)
	}
	first, firstOffers, err := demo.CreateFreshDemo(context.Background(), "Fresh random", domain.PolicyKindRandom, nil)
	if err != nil {
		t.Fatalf("CreateFreshDemo(random): %v", err)
	}
	epsilon := 0.2
	second, secondOffers, err := demo.CreateFreshDemo(context.Background(), "Fresh adaptive", domain.PolicyKindSegmentedEpsilonGreedy, &epsilon)
	if err != nil {
		t.Fatalf("CreateFreshDemo(adaptive): %v", err)
	}
	if initial.ID == first.ID || first.ID == second.ID || initial.Slug == first.Slug || first.Slug == second.Slug {
		t.Fatalf("fresh identities are not distinct: %#v %#v %#v", initial, first, second)
	}
	if first.PolicyKind != domain.PolicyKindRandom || first.Epsilon != nil || second.Epsilon == nil || *second.Epsilon != 0.2 {
		t.Fatalf("fresh policy config: first=%#v second=%#v", first, second)
	}
	assertDemoCatalog(t, first, firstOffers)
	assertDemoCatalog(t, second, secondOffers)
	if len(engine.experiments) != 3 {
		t.Fatalf("persisted experiments = %d, want 3", len(engine.experiments))
	}
}

func TestDemo_PrivacyAndBrandDenylist(t *testing.T) {
	engine := newDemoFakeEngine()
	demo := newTestDemo(t, engine)
	_, offers, err := demo.EnsureDemo(context.Background())
	if err != nil {
		t.Fatalf("EnsureDemo: %v", err)
	}
	deniedText := []string{"sezzle", "klarna", "amazon", "walmart", "paypal", "stripe", "visa", "mastercard", "email", "phone", "address", "credit", "income", "gender", "race"}
	for _, offer := range offers {
		text := strings.ToLower(strings.Join([]string{offer.Slug, offer.MerchantName, offer.Title, offer.Description}, " "))
		for _, denied := range deniedText {
			if strings.Contains(text, denied) {
				t.Fatalf("offer %q contains denied term %q", offer.Slug, denied)
			}
		}
	}
	contextType := reflect.TypeOf(domain.SessionContext{})
	for index := 0; index < contextType.NumField(); index++ {
		field := strings.ToLower(contextType.Field(index).Name)
		for _, denied := range []string{"name", "email", "phone", "address", "birth", "gender", "race", "income", "credit", "deviceid", "location"} {
			if strings.Contains(field, denied) {
				t.Fatalf("SessionContext contains denied field %q", field)
			}
		}
	}
}

func TestDemo_InvalidConfigurationAndSuffix(t *testing.T) {
	engine := newDemoFakeEngine()
	demo, err := NewDemo(engine, simulation.DefaultProfile(), func() string { return "invalid suffix" })
	if err != nil {
		t.Fatalf("NewDemo: %v", err)
	}
	if _, _, err := demo.CreateFreshDemo(context.Background(), "Fresh", domain.PolicyKindRandom, nil); err == nil {
		t.Fatal("CreateFreshDemo(invalid suffix) error = nil")
	}
	if _, _, err := demo.CreateFreshDemo(context.Background(), "", domain.PolicyKindRandom, nil); err == nil {
		t.Fatal("CreateFreshDemo(empty name) error = nil")
	}
	epsilon := 0.2
	if _, _, err := demo.CreateFreshDemo(context.Background(), "Random", domain.PolicyKindRandom, &epsilon); err == nil {
		t.Fatal("CreateFreshDemo(random epsilon) error = nil")
	}
	if _, err := NewDemo(nil, simulation.DefaultProfile(), func() string { return "x" }); err == nil {
		t.Fatal("NewDemo(nil engine) error = nil")
	}
}

func assertDemoCatalog(t *testing.T, experiment domain.Experiment, offers []domain.Offer) {
	t.Helper()
	if err := domain.ValidateExperiment(experiment); err != nil {
		t.Fatalf("ValidateExperiment: %v", err)
	}
	if err := domain.ValidateOffers(experiment.ID, offers); err != nil {
		t.Fatalf("ValidateOffers: %v", err)
	}
	if len(offers) != 6 {
		t.Fatalf("offers = %d, want 6", len(offers))
	}
	ids := make(map[uuid.UUID]struct{}, len(offers))
	slugs := make(map[string]struct{}, len(offers))
	categories := make(map[domain.OfferCategory]struct{}, len(offers))
	for _, offer := range offers {
		ids[offer.ID] = struct{}{}
		slugs[offer.Slug] = struct{}{}
		categories[offer.Category] = struct{}{}
	}
	if len(ids) != 6 || len(slugs) != 6 || len(categories) != 6 {
		t.Fatalf("catalog uniqueness: ids=%d slugs=%d categories=%d", len(ids), len(slugs), len(categories))
	}
}

func newTestDemo(t *testing.T, engine *demoFakeEngine) *Demo {
	t.Helper()
	var suffixMu sync.Mutex
	nextSuffix := 1
	demo, err := NewDemo(engine, simulation.DefaultProfile(), func() string {
		suffixMu.Lock()
		defer suffixMu.Unlock()
		suffix := fmt.Sprintf("fresh-%d", nextSuffix)
		nextSuffix++
		return suffix
	})
	if err != nil {
		t.Fatalf("NewDemo: %v", err)
	}
	return demo
}

type demoFakeEngine struct {
	mu          sync.Mutex
	createCount int
	nextID      int
	experiments []domain.Experiment
	offers      map[uuid.UUID][]domain.Offer
}

func newDemoFakeEngine() *demoFakeEngine {
	return &demoFakeEngine{nextID: 1, offers: make(map[uuid.UUID][]domain.Offer)}
}

func (engine *demoFakeEngine) CreateExperiment(_ context.Context, experiment domain.Experiment, offers []domain.Offer) (domain.Experiment, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, existing := range engine.experiments {
		if existing.Slug == experiment.Slug {
			return domain.Experiment{}, fmt.Errorf("duplicate slug")
		}
	}
	engine.createCount++
	experiment.ID = demoTestUUID(engine.nextID)
	engine.nextID++
	experiment.PolicyVersion = 1
	experiment.CreatedAt = demoTestTime().Add(time.Duration(engine.createCount) * time.Second)
	experiment.UpdatedAt = experiment.CreatedAt
	persistedOffers := append([]domain.Offer(nil), offers...)
	for index := range persistedOffers {
		persistedOffers[index].ID = demoTestUUID(engine.nextID)
		engine.nextID++
		persistedOffers[index].ExperimentID = experiment.ID
	}
	sort.Slice(persistedOffers, func(left, right int) bool {
		return persistedOffers[left].ID.String() < persistedOffers[right].ID.String()
	})
	if err := domain.ValidateExperiment(experiment); err != nil {
		return domain.Experiment{}, err
	}
	if err := domain.ValidateOffers(experiment.ID, persistedOffers); err != nil {
		return domain.Experiment{}, err
	}
	engine.experiments = append(engine.experiments, experiment)
	engine.offers[experiment.ID] = persistedOffers
	return experiment, nil
}

func (engine *demoFakeEngine) ListExperiments(_ context.Context, cursor *service.ExperimentCursor, limit int) ([]domain.Experiment, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	items := append([]domain.Experiment(nil), engine.experiments...)
	sort.Slice(items, func(left, right int) bool {
		if items[left].CreatedAt.Equal(items[right].CreatedAt) {
			return items[left].ID.String() > items[right].ID.String()
		}
		return items[left].CreatedAt.After(items[right].CreatedAt)
	})
	start := 0
	if cursor != nil {
		for index, item := range items {
			if item.ID == cursor.ID {
				start = index + 1
				break
			}
		}
	}
	end := min(start+limit, len(items))
	return append([]domain.Experiment(nil), items[start:end]...), nil
}

func (engine *demoFakeEngine) GetExperimentDetail(_ context.Context, id uuid.UUID) (domain.Experiment, []domain.Offer, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, experiment := range engine.experiments {
		if experiment.ID == id {
			return experiment, append([]domain.Offer(nil), engine.offers[id]...), nil
		}
	}
	return domain.Experiment{}, nil, service.ErrNotFound
}

func demoTestUUID(value int) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("bootstrap-test-%08d", value)))
}

func demoTestTime() time.Time {
	return time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
}
