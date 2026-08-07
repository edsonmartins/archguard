package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

type fakeServiceIdentityReader struct {
	identity domain.Identity
	err      error
}

func (f fakeServiceIdentityReader) FindBySubject(context.Context, string) (domain.Identity, error) {
	return f.identity, f.err
}

type fakeServiceMembershipReader struct {
	memberships []domain.Membership
	err         error
}

func (f fakeServiceMembershipReader) ListByIdentity(context.Context, uuid.UUID) ([]domain.Membership, error) {
	return f.memberships, f.err
}

func TestServiceContextRequiresDedicatedBearer(t *testing.T) {
	id, _ := domain.NewIdentity(domain.IdentityHuman)
	h := NewServiceContextHandler("secret", fakeServiceIdentityReader{identity: id}, fakeServiceMembershipReader{})
	req := httptest.NewRequest(http.MethodPost, "/service/session-context", strings.NewReader(`{"subject":"`+id.Subject+`"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestServiceContextReturnsIdentityAndMemberships(t *testing.T) {
	id, _ := domain.NewIdentity(domain.IdentityHuman)
	m, _ := domain.NewMembership(id.ID, uuid.New())
	h := NewServiceContextHandler("secret", fakeServiceIdentityReader{identity: id}, fakeServiceMembershipReader{memberships: []domain.Membership{m}})
	req := httptest.NewRequest(http.MethodPost, "/service/session-context", strings.NewReader(`{"subject":"`+id.Subject+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), id.ID.String()) || !strings.Contains(rec.Body.String(), m.OrganizationID.String()) {
		t.Fatalf("response missing identity or membership: %s", rec.Body.String())
	}
}
