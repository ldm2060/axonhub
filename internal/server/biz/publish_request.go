package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/model"
	"github.com/ldm2060/axonhub/internal/ent/publishrequest"
)

// PublishRequestService handles publish request CRUD and review logic.
type PublishRequestService struct {
	*AbstractService
}

// PublishRequestServiceParams holds the dependencies.
type PublishRequestServiceParams struct {
	fx.In

	Ent *ent.Client
}

// NewPublishRequestService creates a new PublishRequestService.
func NewPublishRequestService(params PublishRequestServiceParams) *PublishRequestService {
	return &PublishRequestService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

// CreatePublishRequest creates a new publish request for a resource.
func (s *PublishRequestService) CreatePublishRequest(ctx context.Context, resourceType publishrequest.ResourceType, resourceID, requesterID int, comment string) (*ent.PublishRequest, error) {
	client := s.entFromContext(ctx)

	existing, err := client.PublishRequest.Query().
		Where(
			publishrequest.ResourceTypeEQ(resourceType),
			publishrequest.ResourceIDEQ(resourceID),
			publishrequest.RequesterIDEQ(requesterID),
			publishrequest.StatusEQ(publishrequest.StatusPending),
		).First(ctx)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("a pending publish request already exists for this resource")
	}

	req, err := client.PublishRequest.Create().
		SetResourceType(resourceType).
		SetResourceID(resourceID).
		SetRequesterID(requesterID).
		SetRequestComment(comment).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create publish request: %w", err)
	}
	return req, nil
}

// CancelPublishRequest cancels a pending publish request.
func (s *PublishRequestService) CancelPublishRequest(ctx context.Context, id, requesterID int) error {
	client := s.entFromContext(ctx)

	req, err := client.PublishRequest.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("publish request not found: %w", err)
	}
	if req.RequesterID != requesterID {
		return fmt.Errorf("only the requester can cancel")
	}
	if req.Status != publishrequest.StatusPending {
		return fmt.Errorf("only pending requests can be cancelled")
	}

	return client.PublishRequest.DeleteOneID(id).Exec(ctx)
}

// ReviewPublishRequest approves or rejects a publish request.
func (s *PublishRequestService) ReviewPublishRequest(ctx context.Context, id, reviewerID int, action publishrequest.Status, comment string) (*ent.PublishRequest, error) {
	client := s.entFromContext(ctx)

	req, err := client.PublishRequest.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("publish request not found: %w", err)
	}
	if req.Status != publishrequest.StatusPending {
		return nil, fmt.Errorf("only pending requests can be reviewed")
	}

	update := client.PublishRequest.UpdateOneID(id).
		SetStatus(action).
		SetReviewerID(reviewerID).
		SetReviewComment(comment)

	req, err = update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update publish request: %w", err)
	}

	if action == publishrequest.StatusApproved {
		if err := s.publishResource(ctx, req.ResourceType, req.ResourceID); err != nil {
			return nil, fmt.Errorf("failed to publish resource: %w", err)
		}
	}

	return req, nil
}

func (s *PublishRequestService) publishResource(ctx context.Context, resourceType publishrequest.ResourceType, resourceID int) error {
	client := s.entFromContext(ctx)

	switch resourceType {
	case publishrequest.ResourceTypeChannel:
		return client.Channel.UpdateOneID(resourceID).
			SetVisibility(channel.VisibilityPublished).
			ClearSharedWith().
			Exec(ctx)
	case publishrequest.ResourceTypeModel:
		return client.Model.UpdateOneID(resourceID).
			SetVisibility(model.VisibilityPublished).
			ClearSharedWith().
			Exec(ctx)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

// ListPublishRequests lists publish requests, optionally filtered by status.
func (s *PublishRequestService) ListPublishRequests(ctx context.Context, status *publishrequest.Status) ([]*ent.PublishRequest, error) {
	client := s.entFromContext(ctx)
	query := client.PublishRequest.Query()
	if status != nil {
		query = query.Where(publishrequest.StatusEQ(*status))
	}
	return query.All(ctx)
}
