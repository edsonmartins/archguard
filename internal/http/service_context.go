// Copyright 2026 IntegrAllTech Ltda.
//
// Licensed under the Apache License, Version 2.0 (the "License");

package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/casdoor/casdoor/internal/domain"
	"github.com/google/uuid"
)

// ServiceIdentityReader is the minimum cross-tenant read needed by a trusted
// ArchGate caller. Keeping this port narrow prevents the machine endpoint from
// accidentally gaining access to the administrative control-plane handlers.
type ServiceIdentityReader interface {
	FindBySubject(ctx context.Context, subject string) (domain.Identity, error)
}

type ServiceMembershipReader interface {
	ListByIdentity(ctx context.Context, identityID uuid.UUID) ([]domain.Membership, error)
}

type ServiceContextHandler struct {
	token       []byte
	identities  ServiceIdentityReader
	memberships ServiceMembershipReader
}

func NewServiceContextHandler(serviceToken string, identities ServiceIdentityReader, memberships ServiceMembershipReader) *ServiceContextHandler {
	return &ServiceContextHandler{token: []byte(strings.TrimSpace(serviceToken)), identities: identities, memberships: memberships}
}

type serviceContextRequest struct {
	Subject string `json:"subject"`
}

type serviceMembership struct {
	MembershipID   string `json:"membership_id"`
	OrganizationID string `json:"organization_id"`
	Status         string `json:"status"`
}

type serviceContextResponse struct {
	Subject        string              `json:"subject"`
	IdentityID     string              `json:"identity_id"`
	IdentityStatus string              `json:"identity_status"`
	Memberships    []serviceMembership `json:"memberships"`
}

func (h *ServiceContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeError(w, http.StatusMethodNotAllowed, "método não suportado")
		return
	}
	if len(h.token) == 0 || !validServiceBearer(r, h.token) {
		writeError(w, http.StatusUnauthorized, "credencial de serviço inválida")
		return
	}
	var req serviceContextRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || strings.TrimSpace(req.Subject) == "" {
		writeError(w, http.StatusBadRequest, "subject obrigatório")
		return
	}
	idn, err := h.identities.FindBySubject(r.Context(), strings.TrimSpace(req.Subject))
	if err != nil {
		writeError(w, http.StatusNotFound, "identidade não encontrada")
		return
	}
	memberships, err := h.memberships.ListByIdentity(r.Context(), idn.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "não foi possível resolver memberships")
		return
	}
	out := serviceContextResponse{
		Subject:        idn.Subject,
		IdentityID:     idn.ID.String(),
		IdentityStatus: string(idn.Status),
		Memberships:    make([]serviceMembership, 0, len(memberships)),
	}
	for _, m := range memberships {
		out.Memberships = append(out.Memberships, serviceMembership{
			MembershipID:   m.ID.String(),
			OrganizationID: m.OrganizationID.String(),
			Status:         string(m.Status),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func validServiceBearer(r *http.Request, expected []byte) bool {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	got := []byte(strings.TrimSpace(strings.TrimPrefix(value, prefix)))
	return len(got) == len(expected) && subtle.ConstantTimeCompare(got, expected) == 1
}
