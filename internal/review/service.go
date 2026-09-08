// Package review joins trusted approval sources without owning execution.
package review

import (
	"context"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type Source interface {
	PendingApprovals() []core.ApprovalRequest
	DecideApproval(context.Context, string, capability.ApprovalDecision) (core.CapabilityResult, error)
}

type Service struct{ sources []Source }

func New(sources ...Source) *Service { return &Service{sources: append([]Source(nil), sources...)} }

func (s *Service) snapshot(ctx context.Context) ([]core.ApprovalRequest, map[string]Source, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	owners := make(map[string]Source)
	requests := make([]core.ApprovalRequest, 0)
	for _, source := range s.sources {
		if source == nil {
			return nil, nil, core.ErrIncompatibleState
		}
		for _, request := range source.PendingApprovals() {
			if request.RequestID == "" || len(request.RequestID) > 128 || owners[request.RequestID] != nil {
				return nil, nil, core.ErrIncompatibleState
			}
			if len(requests) >= 256 {
				return nil, nil, core.ErrIncompatibleState
			}
			owners[request.RequestID] = source
			requests = append(requests, capability.CloneApprovalRequest(request))
		}
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].RequestID < requests[j].RequestID })
	return requests, owners, nil
}

func (s *Service) Pending(ctx context.Context) ([]core.ApprovalRequest, error) {
	requests, _, err := s.snapshot(ctx)
	return requests, err
}

func (s *Service) Decide(ctx context.Context, id string, decision capability.ApprovalDecision) (core.CapabilityResult, error) {
	requests, owners, err := s.snapshot(ctx)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	for _, request := range requests {
		if request.RequestID != id {
			continue
		}
		if err := capability.ValidateApprovalDecision(request, decision); err != nil {
			return core.CapabilityResult{}, err
		}
		return owners[id].DecideApproval(ctx, id, decision)
	}
	return core.CapabilityResult{}, core.ErrNotFound
}
