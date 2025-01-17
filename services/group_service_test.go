package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideGroupService(ctrl *gomock.Controller) *GroupService {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	return NewGroupService(appRepo, groupRepo, eventRepo, eventDeliveryRepo, nooplimiter.NewNoopLimiter())
}

func TestGroupService_CreateGroup(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		newGroup *models.Group
	}
	tests := []struct {
		name        string
		args        args
		wantGroup   *datastore.Group
		dbFn        func(gs *GroupService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Group{
					Name:              "test_group",
					LogoURL:           "https://google.com",
					RateLimit:         1000,
					RateLimitDuration: "1m",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				a.EXPECT().CreateGroup(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantGroup: &datastore.Group{
				Name:              "test_group",
				LogoURL:           "https://google.com",
				RateLimit:         1000,
				RateLimitDuration: "1m",
				Config: &datastore.GroupConfig{
					Signature: datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
						Hash:   "SHA256",
					},
					Strategy: datastore.StrategyConfiguration{
						Type: "default",
						Default: datastore.DefaultStrategyConfiguration{
							IntervalSeconds: 20,
							RetryLimit:      4,
						},
					},
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			wantErr: false,
		},
		{
			name: "should_create_group_with_rate_limit_defaults",
			args: args{
				ctx: ctx,
				newGroup: &models.Group{
					Name:    "test_group_1",
					LogoURL: "https://google.com",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				a.EXPECT().CreateGroup(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantGroup: &datastore.Group{
				Name:              "test_group_1",
				LogoURL:           "https://google.com",
				RateLimit:         5000,
				RateLimitDuration: "1m",
				Config: &datastore.GroupConfig{
					Signature: datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
						Hash:   "SHA256",
					},
					Strategy: datastore.StrategyConfiguration{
						Type: "default",
						Default: datastore.DefaultStrategyConfiguration{
							IntervalSeconds: 20,
							RetryLimit:      4,
						},
					},
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_group",
			args: args{
				ctx: ctx,
				newGroup: &models.Group{
					Name:    "test_group",
					LogoURL: "https://google.com",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				a.EXPECT().CreateGroup(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create group",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, err := gs.CreateGroup(tc.args.ctx, tc.args.newGroup)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, group.UID)
			require.NotEmpty(t, group.ID)
			require.NotEmpty(t, group.CreatedAt)
			require.NotEmpty(t, group.UpdatedAt)
			require.Empty(t, group.DeletedAt)

			stripVariableFields(t, "group", group)
			require.Equal(t, tc.wantGroup, group)
		})
	}
}

func TestGroupService_UpdateGroup(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		group  *datastore.Group
		update *models.Group
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantGroup   *datastore.Group
		dbFn        func(gs *GroupService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_group",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				update: &models.Group{
					Name:    "test_group",
					LogoURL: "https://google.com",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			wantGroup: &datastore.Group{
				UID:     "12345",
				Name:    "test_group",
				LogoURL: "https://google.com",
				Config: &datastore.GroupConfig{
					Signature: datastore.SignatureConfiguration{
						Header: "X-Convoy-Signature",
						Hash:   "SHA256",
					},
					Strategy: datastore.StrategyConfiguration{
						Type: "default",
						Default: datastore.DefaultStrategyConfiguration{
							IntervalSeconds: 20,
							RetryLimit:      4,
						},
					},
					DisableEndpoint: true,
					ReplayAttacks:   true,
				},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				a.EXPECT().UpdateGroup(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_empty_name",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				update: &models.Group{
					Name:    "",
					LogoURL: "https://google.com",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "name:please provide a valid name",
		},
		{
			name: "should_fail_to_update_group",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				update: &models.Group{
					Name:    "test_group",
					LogoURL: "https://google.com",
					Config: datastore.GroupConfig{
						Signature: datastore.SignatureConfiguration{
							Header: "X-Convoy-Signature",
							Hash:   "SHA256",
						},
						Strategy: datastore.StrategyConfiguration{
							Type: "default",
							Default: datastore.DefaultStrategyConfiguration{
								IntervalSeconds: 20,
								RetryLimit:      4,
							},
						},
						DisableEndpoint: true,
						ReplayAttacks:   true,
					},
				},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				a.EXPECT().UpdateGroup(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating Group",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, err := gs.UpdateGroup(tc.args.ctx, tc.args.group, tc.args.update)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			c1 := tc.wantGroup.Config
			c2 := group.Config

			tc.wantGroup.Config = nil
			group.Config = nil
			require.Equal(t, tc.wantGroup, group)
			require.Equal(t, c1, c2)
		})
	}
}

func TestGroupService_GetGroups(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.GroupFilter
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantGroups  []*datastore.Group
		dbFn        func(gs *GroupService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_get_groups",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"default_group"}},
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().LoadGroups(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return([]*datastore.Group{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CountGroupMessages(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)
			},
			wantGroups: []*datastore.Group{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_groups_trims-whitespaces-from-query",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{" default_group "}},
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().LoadGroups(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return([]*datastore.Group{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CountGroupMessages(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)
			},
			wantGroups: []*datastore.Group{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_get_groups_trims-whitespaces-from-query-retains-case",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"  deFault_Group"}},
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().LoadGroups(gomock.Any(), &datastore.GroupFilter{Names: []string{"deFault_Group"}}).
					Times(1).Return([]*datastore.Group{
					{UID: "123"},
					{UID: "abc"},
				}, nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CountGroupMessages(gomock.Any(), gomock.Any()).Times(2).Return(int64(1), nil)
			},
			wantGroups: []*datastore.Group{
				{
					UID: "123",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
				{
					UID: "abc",
					Statistics: &datastore.GroupStatistics{
						MessagesSent: 1,
						TotalApps:    1,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_get_groups",
			args: args{
				ctx:    ctx,
				filter: &datastore.GroupFilter{Names: []string{"default_group"}},
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().LoadGroups(gomock.Any(), &datastore.GroupFilter{Names: []string{"default_group"}}).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while fetching Groups",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			group, err := gs.GetGroups(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantGroups, group)
		})
	}
}

func TestGroupService_FillGroupStatistics(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		g   *datastore.Group
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(gs *GroupService)
		wantGroup   *datastore.Group
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_fill_statistics",
			args: args{
				ctx: ctx,
				g:   &datastore.Group{UID: "1234"},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), "1234").Times(1).Return(int64(1), nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CountGroupMessages(gomock.Any(), "1234").Times(1).Return(int64(1), nil)
			},
			wantGroup: &datastore.Group{
				UID: "1234",
				Statistics: &datastore.GroupStatistics{
					MessagesSent: 1,
					TotalApps:    1,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_count_group_messages",
			args: args{
				ctx: ctx,
				g:   &datastore.Group{UID: "1234"},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), "1234").Times(1).Return(int64(1), nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CountGroupMessages(gomock.Any(), "1234").
					Times(1).Return(int64(1), errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to count group statistics",
		},
		{
			name: "should_fail_to_count_group_apps",
			args: args{
				ctx: ctx,
				g:   &datastore.Group{UID: "1234"},
			},
			dbFn: func(gs *GroupService) {
				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().CountGroupApplications(gomock.Any(), "1234").
					Times(1).Return(int64(1), errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to count group statistics",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			err := gs.FillGroupStatistics(tc.args.ctx, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantGroup, tc.args.g)
		})
	}
}

func TestGroupService_DeleteGroup(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		dbFn        func(gs *GroupService)
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_group",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().DeleteGroup(gomock.Any(), "12345").Times(1).Return(nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), "12345").Times(1).Return(nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().DeleteGroupEvents(gomock.Any(), "12345").Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_delete_group",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().DeleteGroup(gomock.Any(), "12345").Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete group",
		},
		{
			name: "should_fail_to_delete_group_apps",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().DeleteGroup(gomock.Any(), "12345").Times(1).Return(nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), "12345").Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete group apps",
		},
		{
			name: "should_fail_to_delete_group_messages",
			args: args{
				ctx: ctx,
				id:  "12345",
			},
			dbFn: func(gs *GroupService) {
				g, _ := gs.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().DeleteGroup(gomock.Any(), "12345").Times(1).Return(nil)

				a, _ := gs.appRepo.(*mocks.MockApplicationRepository)
				a.EXPECT().DeleteGroupApps(gomock.Any(), "12345").Times(1).Return(nil)

				e, _ := gs.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().DeleteGroupEvents(gomock.Any(), "12345").Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete group events",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			gs := provideGroupService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(gs)
			}

			err := gs.DeleteGroup(tc.args.ctx, tc.args.id)
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
