package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func provideSecurityService(ctrl *gomock.Controller) *SecurityService {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
	return NewSecurityService(groupRepo, apiKeyRepo)
}

func TestSecurityService_CreateAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		newApiKey *models.APIKey
	}
	expires := time.Now().Add(time.Hour)
	tests := []struct {
		name        string
		args        args
		wantAPIKey  *datastore.APIKey
		dbFn        func(ss *SecurityService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   auth.RoleAdmin,
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires,
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "test_api_key",
				Type: "api",
				Role: auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234"},
					Apps:   []string{"1234"},
				},
				ExpiresAt:      primitive.NewDateTimeFromTime(expires),
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234"}).
					Times(1).Return([]datastore.Group{{UID: "1234"}}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_invalid_expiry",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   auth.RoleAdmin,
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires.Add(-2 * time.Hour),
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "expiry date is invalid",
		},
		{
			name: "should_error_for_invalid_api_key_role",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   "abc",
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires,
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid api key role",
		},
		{
			name: "should_fail_to_fetch_groups",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   auth.RoleAdmin,
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires,
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234"}).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid group",
		},
		{
			name: "should_error_for_group_length_mismatch",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   auth.RoleAdmin,
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires,
				},
			},
			wantAPIKey: &datastore.APIKey{
				Name: "test_api_key",
				Type: "api",
				Role: auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234"},
					Apps:   []string{"1234"},
				},
				ExpiresAt:      primitive.NewDateTimeFromTime(expires),
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234"}).
					Times(1).Return([]datastore.Group{}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "cannot find group",
		},
		{
			name: "should_fail_to_create_api_key",
			args: args{
				ctx: ctx,
				newApiKey: &models.APIKey{
					Name: "test_api_key",
					Type: "api",
					Role: auth.Role{
						Type:   auth.RoleAdmin,
						Groups: []string{"1234"},
						Apps:   []string{"1234"},
					},
					ExpiresAt: expires,
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234"}).
					Times(1).Return([]datastore.Group{{UID: "1234"}}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, keyString, err := ss.CreateAPIKey(tc.args.ctx, tc.args.newApiKey)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, apiKey.UID)
			require.NotEmpty(t, apiKey.MaskID)
			require.NotEmpty(t, apiKey.Hash)
			require.NotEmpty(t, apiKey.Salt)
			require.NotEmpty(t, apiKey.ID)
			require.NotEmpty(t, apiKey.CreatedAt)
			require.NotEmpty(t, apiKey.UpdatedAt)
			require.NotEmpty(t, keyString)
			require.Empty(t, apiKey.DeletedAt)

			stripVariableFields(t, "apiKey", apiKey)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_CreateAppPortalAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		group   *datastore.Group
		app     *datastore.Application
		baseUrl *string
	}
	tests := []struct {
		name        string
		args        args
		wantAPIKey  *datastore.APIKey
		dbFn        func(ss *SecurityService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_app_portal_api_key",
			args: args{
				ctx:     ctx,
				group:   &datastore.Group{UID: "1234"},
				app:     &datastore.Application{UID: "abc", GroupID: "1234", Title: "test_app"},
				baseUrl: stringPtr("https://getconvoy.io"),
			},
			wantAPIKey: &datastore.APIKey{
				Name: "test_app",
				Type: "app_portal",
				Role: auth.Role{
					Type:   auth.RoleUIAdmin,
					Groups: []string{"1234"},
					Apps:   []string{"abc"},
				},
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_app_not_belong_to_group_api_key",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "1234"},
				app:   &datastore.Application{GroupID: "12345"},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "app does not belong to group",
		},
		{
			name: "should_fail_to_create_app_portal_api_key",
			args: args{
				ctx:     ctx,
				group:   &datastore.Group{UID: "1234"},
				app:     &datastore.Application{UID: "abc", GroupID: "1234", Title: "test_app"},
				baseUrl: stringPtr("https://getconvoy.io"),
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().CreateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, keyString, err := ss.CreateAppPortalAPIKey(tc.args.ctx, tc.args.group, tc.args.app, tc.args.baseUrl)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, apiKey.UID)
			require.NotEmpty(t, apiKey.MaskID)
			require.NotEmpty(t, apiKey.Hash)
			require.NotEmpty(t, apiKey.Salt)
			require.NotEmpty(t, apiKey.ID)
			require.NotEmpty(t, apiKey.CreatedAt)
			require.NotEmpty(t, apiKey.UpdatedAt)
			require.NotEmpty(t, keyString)
			require.Empty(t, apiKey.DeletedAt)

			require.True(t, strings.HasSuffix(*tc.args.baseUrl, fmt.Sprintf("?groupID=%s&appId=%s", tc.args.group.UID, tc.args.app.UID)))

			stripVariableFields(t, "apiKey", apiKey)
			apiKey.ExpiresAt = 0
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_RevokeAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		uid string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_revoke_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"1234"}).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx: ctx,
				uid: "",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_fail_to_revoke_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().RevokeAPIKeys(gomock.Any(), []string{"1234"}).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to revoke api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			err := ss.RevokeAPIKey(tc.args.ctx, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestSecurityService_GetAPIKeyByID(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		uid string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_get_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(&datastore.APIKey{UID: "1234"}, nil)
			},
			wantAPIKey: &datastore.APIKey{UID: "1234"},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx: ctx,
				uid: "",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_fail_to_get_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, err := ss.GetAPIKeyByID(tc.args.ctx, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_UpdateAPIKey(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx  context.Context
		uid  string
		role *auth.Role
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ss *SecurityService)
		wantAPIKey  *datastore.APIKey
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234", "abc"}).
					Times(1).Return([]datastore.Group{
					{UID: "1234"},
					{UID: "abc"},
				}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(
					&datastore.APIKey{
						UID: "ref",
						Role: auth.Role{
							Type:   auth.RoleAPI,
							Groups: []string{"avs"},
							Apps:   nil,
						},
					}, nil)

				a.EXPECT().UpdateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantAPIKey: &datastore.APIKey{
				UID: "ref",
				Role: auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
		},
		{
			name: "should_error_for_empty_uid",
			args: args{
				ctx: ctx,
				uid: "",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "key id is empty",
		},
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type: "abc",
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid api key role",
		},
		{
			name: "should_fail_to_fetch_groups",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234", "abc"}).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invalid group",
		},
		{
			name: "should_error_for_group_length_mismatch",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234", "abc"}).
					Times(1).Return([]datastore.Group{
					{UID: "1234"},
				}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "cannot find group",
		},
		{
			name: "should_fail_find_api_key_by_id",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234", "abc"}).
					Times(1).Return([]datastore.Group{
					{UID: "1234"},
					{UID: "abc"},
				}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch api key",
		},
		{
			name: "should_update_api_key",
			args: args{
				ctx: ctx,
				uid: "1234",
				role: &auth.Role{
					Type:   auth.RoleAdmin,
					Groups: []string{"1234", "abc"},
				},
			},
			dbFn: func(ss *SecurityService) {
				g, _ := ss.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupsByIDs(gomock.Any(), []string{"1234", "abc"}).
					Times(1).Return([]datastore.Group{
					{UID: "1234"},
					{UID: "abc"},
				}, nil)

				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().FindAPIKeyByID(gomock.Any(), "1234").
					Times(1).Return(
					&datastore.APIKey{
						UID: "ref",
						Role: auth.Role{
							Type:   auth.RoleAPI,
							Groups: []string{"avs"},
							Apps:   nil,
						},
					}, nil)

				a.EXPECT().UpdateAPIKey(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update api key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKey, err := ss.UpdateAPIKey(tc.args.ctx, tc.args.uid, tc.args.role)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantAPIKey, apiKey)
		})
	}
}

func TestSecurityService_GetAPIKeys(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		pageable *datastore.Pageable
	}
	tests := []struct {
		name               string
		args               args
		wantAPIKeys        []datastore.APIKey
		dbFn               func(ss *SecurityService)
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_fetch_api_keys",
			args: args{
				ctx: ctx,
				pageable: &datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().LoadAPIKeysPaged(gomock.Any(), &datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).
					Times(1).Return(
					[]datastore.APIKey{
						{
							UID: "ref",
							Role: auth.Role{
								Type:   auth.RoleAPI,
								Groups: []string{"avs"},
								Apps:   nil,
							},
						},
						{
							UID: "abc",
							Role: auth.Role{
								Type:   auth.RoleAPI,
								Groups: []string{"123"},
								Apps:   nil,
							},
						},
					},
					datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   1,
						Prev:      1,
						Next:      1,
						TotalPage: 1,
					}, nil)
			},
			wantAPIKeys: []datastore.APIKey{
				{
					UID: "ref",
					Role: auth.Role{
						Type:   auth.RoleAPI,
						Groups: []string{"avs"},
						Apps:   nil,
					},
				},
				{
					UID: "abc",
					Role: auth.Role{
						Type:   auth.RoleAPI,
						Groups: []string{"123"},
						Apps:   nil,
					},
				},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     1,
				Page:      1,
				PerPage:   1,
				Prev:      1,
				Next:      1,
				TotalPage: 1,
			},
		},
		{
			name: "should_fetch_api_keys",
			args: args{
				ctx: ctx,
				pageable: &datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			dbFn: func(ss *SecurityService) {
				a, _ := ss.apiKeyRepo.(*mocks.MockAPIKeyRepository)
				a.EXPECT().LoadAPIKeysPaged(gomock.Any(), &datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).
					Times(1).
					Return(
						nil,
						datastore.PaginationData{},
						errors.New("failed"),
					)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to load api keys",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ss := provideSecurityService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			apiKeys, paginationData, err := ss.GetAPIKeys(tc.args.ctx, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantAPIKeys, apiKeys)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
