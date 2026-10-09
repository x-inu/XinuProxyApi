package auth

import (
	"context"
	"testing"
)

type dummyStore struct {
	savedCount int
}

func (d *dummyStore) Save(ctx context.Context, auth *Auth) (string, error) {
	d.savedCount++
	return "", nil
}

func (d *dummyStore) List(ctx context.Context) ([]*Auth, error) {
	return nil, nil
}

func (d *dummyStore) Delete(ctx context.Context, id string) error {
	return nil
}

func TestConductorPersist_SkipsKeylessAndConfigAuth(t *testing.T) {
	store := &dummyStore{}
	mgr := NewManager(nil, nil, nil)
	mgr.store = store

	keylessAuth := &Auth{
		ID:       "openai-compatibility:opencodefree:12345",
		Provider: "openai-compatible-opencodefree",
		Attributes: map[string]string{
			"source":    "config:opencodeFree[tok]",
			"auth_kind": AuthKindAPIKey,
		},
		Metadata: map[string]any{
			"disable_cooling": true,
		},
	}

	if err := mgr.persist(context.Background(), keylessAuth); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.savedCount != 0 {
		t.Fatalf("expected 0 saves to store, got %d", store.savedCount)
	}
}
