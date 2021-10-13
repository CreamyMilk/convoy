package convoy

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrGroupNotFound = errors.New("group not found")

type Group struct {
	ID      primitive.ObjectID `json:"-" bson:"_id"`
	UID     string             `json:"uid" bson:"uid"`
	Name    string             `json:"name" bson:"name"`
	LogoURL string             `json:"logo_url" bson:"logo_url"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

func (o *Group) IsDeleted() bool { return o.DeletedAt > 0 }

func (o *Group) IsOwner(a *Application) bool { return o.UID == a.GroupID }

type GroupRepository interface {
	LoadGroups(context.Context) ([]*Group, error)
	CreateGroup(context.Context, *Group) error
	UpdateGroup(context.Context, *Group) error
	FetchGroupByID(context.Context, string) (*Group, error)
}